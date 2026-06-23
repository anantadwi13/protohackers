package proto

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

var (
	ErrServerAlreadyStartedOrStopped = errors.New("server already started/stopped")
)

type server struct {
	handler ServerHandler
	tcpAddr *net.TCPAddr

	status          atomic.Uint32
	stopped         chan struct{}
	serverCtx       context.Context
	serverCtxCancel context.CancelCauseFunc
}

func NewServer(address string, handler ServerHandler) (Server, error) {
	if handler == nil {
		return nil, errors.New("nil ServerHandler")
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancelCause(context.Background())

	return &server{
		handler:         handler,
		tcpAddr:         tcpAddr,
		stopped:         make(chan struct{}),
		serverCtx:       ctx,
		serverCtxCancel: cancel,
	}, nil
}

func (s *server) Listen() error {
	if !s.status.CompareAndSwap(0, 1) {
		return ErrServerAlreadyStartedOrStopped
	}

	defer close(s.stopped)

	listener, err := net.ListenTCP("tcp", s.tcpAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	wg := sync.WaitGroup{}
	defer wg.Wait()

	wg.Go(func() {
		<-s.serverCtx.Done()
		_ = listener.Close()
	})

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			if s.serverCtx.Err() == nil {
				return err
			}
			return nil
		}

		wg.Go(func() {
			ch := &connHandler{conn: conn, handler: s.handler}
			ch.looper(s.serverCtx)
		})
	}
}

func (s *server) Shutdown(ctx context.Context) error {
	if !s.status.CompareAndSwap(1, 2) {
		return ErrServerAlreadyStartedOrStopped
	}

	s.serverCtxCancel(nil)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.stopped:
		return nil
	}
}

func (s *server) internalProtoImplementation() {
	// no-op
}

type connHandler struct {
	conn    *net.TCPConn
	handler ServerHandler
}

func (c *connHandler) looper(ctx context.Context) {
	defer c.conn.Close()

	wg := sync.WaitGroup{}
	defer wg.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg.Go(func() {
		<-ctx.Done()
		_ = c.conn.Close()
	})

	// hello handshake

	hello := &MessageHello{}
	_, _, err := hello.Unmarshal(c.conn)
	if err != nil {
		return
	}
	if hello.Protocol != "pestcontrol" {
		writeError(c.conn, fmt.Errorf("invalid hello protocol: %s", hello.Protocol))
		return
	}
	if hello.Version != 1 {
		writeError(c.conn, fmt.Errorf("invalid hello version: %d", hello.Version))
		return
	}
	_, err = hello.Marshal(c.conn)
	if err != nil {
		writeError(c.conn, fmt.Errorf("failed to write hello response: %w", err))
		return
	}

	for {
		siteVisit := &MessageSiteVisit{}
		_, _, err = siteVisit.Unmarshal(c.conn)
		if err != nil {
			writeError(c.conn, fmt.Errorf("failed to read site visit: %w", err))
			return
		}
		err = c.handler.HandleSiteVisit(ctx, *siteVisit)
		if err != nil {
			writeError(c.conn, fmt.Errorf("failed to handle site visit: %w", err))
			// will continue receiving
		}
	}
}

func writeError(w io.Writer, err error) (written bool) {
	if err == nil {
		return
	}

	msgError := &MessageError{Message: String(err.Error())}
	_, err = msgError.Marshal(w)
	if err != nil {
		log.Println("failed to write error message:", err)
		return
	}
	return true
}
