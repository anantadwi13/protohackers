package lrcp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anantadwi13/protohackers/7-line-reversal/util"
)

const (
	MaxLineLength = 10_000 // 10k with 1 char as newline
)

type Handler func(ctx context.Context, w io.Writer, r io.Reader)

type Server interface {
	Listen() error
	Shutdown(ctx context.Context) error
}

type serverUDP struct {
	addr    *net.UDPAddr
	handler Handler

	sessionManager *sessionManager
	lock           sync.Mutex
	stopped        chan struct{}
	cancel         context.CancelFunc
	isRunning      int32
}

func NewServer(addr string, handler Handler) (Server, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	server := &serverUDP{
		addr:    udpAddr,
		handler: handler,
		stopped: make(chan struct{}),
	}

	return server, nil
}

func (s *serverUDP) Listen() error {
	if !atomic.CompareAndSwapInt32(&s.isRunning, 0, 1) {
		return nil
	}

	serverCtx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	s.lock.Lock()
	s.cancel = cancelFunc
	s.lock.Unlock()

	defer close(s.stopped)

	wg := sync.WaitGroup{}
	defer wg.Wait()

	conn, err := net.ListenUDP("udp", s.addr)
	if err != nil {
		return err
	}

	var errChan = make(chan error)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancelFunc()

		<-serverCtx.Done()
		_ = conn.Close()
	}()

	s.lock.Lock()
	sessManager := newSessionManager(s.handler)
	s.sessionManager = sessManager
	s.lock.Unlock()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancelFunc()

		var (
			wgLocal = sync.WaitGroup{}
			err     error
		)
		defer wgLocal.Wait()
		defer func() {
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					errChan <- err
				}
			}
		}()

		for {
			if serverCtx.Err() != nil {
				return
			}

			var (
				buf  = util.PoolGet(MTU)
				n    int
				addr *net.UDPAddr
			)

			n, addr, err = conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			wgLocal.Add(1)
			go func() {
				defer wgLocal.Done()
				defer util.PoolPut(buf)

				buf = buf[:n]
				message, err := UnmarshalMessage(bytes.NewReader(buf))
				if err != nil {
					if !errors.Is(err, io.EOF) {
						log.Printf("UnmarshalMessage err: %s", err)
					}
					return
				}
				log.Printf("incoming %d %d %s", message.SessionNumber(), len(buf), string(buf))
				s.handleIncomingMessage(serverCtx, conn, message, addr)
			}()
		}
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for e := range errChan {
		err = errors.Join(err, e)
	}
	if err != nil {
		return err
	}

	log.Printf("server stopped")

	return nil
}

func (s *serverUDP) handleIncomingMessage(ctx context.Context, conn *net.UDPConn, msg Message, addr *net.UDPAddr) {
	switch m := msg.(type) {
	case *MessageConnect:
		sess := s.sessionManager.GetOrCreate(ctx, conn, addr, m.sessionNumber)
		sess.HandleIncoming(conn, msg)
	default:
		// if no session, close conn
		sess := s.sessionManager.Get(ctx, m.SessionNumber())
		if sess == nil {
			res := &MessageClose{
				sessionNumber: m.SessionNumber(),
			}

			dispatcher(ctx, conn, res, addr)
			return
		}
		sess.HandleIncoming(conn, msg)
	}
}

func (s *serverUDP) Shutdown(ctx context.Context) error {
	if atomic.LoadInt32(&s.isRunning) == 0 {
		return nil
	}

	s.lock.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	if s.sessionManager != nil {
		s.sessionManager.Shutdown(ctx)
	}
	s.lock.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.stopped:
	}
	return nil
}

func dispatcher(ctx context.Context, conn *net.UDPConn, msg Message, clientAddr *net.UDPAddr) {
	buf := util.PoolGet(MTU)
	defer util.PoolPut(buf)

	bufWriter := bytes.NewBuffer(buf) // todo put bytes.Buffer to pool?
	bufWriter.Reset()

	err := msg.Marshal(bufWriter)
	if err != nil {
		log.Printf("dispatcher Marshal. err: %s", err)
		return
	}

	log.Printf("outgoing %d %d %s", msg.SessionNumber(), bufWriter.Len(), string(bufWriter.Bytes()))

	err = conn.SetWriteDeadline(time.Now().Add(TimeoutRetransmission))
	if err != nil {
		log.Printf("dispatcher SetWriteDeadline. err: %s", err)
		return
	}
	_, err = conn.WriteToUDP(bufWriter.Bytes(), clientAddr)
	if err != nil {
		log.Printf("dispatcher WriteToUDP. err: %s", err)
		return
	}
}
