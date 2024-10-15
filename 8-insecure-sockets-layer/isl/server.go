package isl

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anantadwi13/protohackers/8-insecure-sockets-layer/util"
)

const (
	MaxLineLength = 5000
	debug         = false
)

var (
	ErrServerAlreadyRunning = errors.New("server is already running")
)

type Handler func(ctx context.Context, w io.Writer, r io.Reader)

type Server interface {
	Listen() error
	Shutdown(ctx context.Context) error
}

func NewServer(addr string, handler Handler) (Server, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	s := &server{
		addr:       tcpAddr,
		handler:    handler,
		serverCtx:  ctx,
		cancelCtx:  cancelFunc,
		closedChan: make(chan struct{}),
	}
	return s, nil
}

type server struct {
	addr    *net.TCPAddr
	handler Handler

	status     atomic.Int32 // 0: initialized, 1: running, 2: stopped
	serverCtx  context.Context
	cancelCtx  context.CancelFunc
	closedChan chan struct{}
}

func (s *server) Listen() error {
	if !s.status.CompareAndSwap(0, 1) {
		return ErrServerAlreadyRunning
	}

	defer close(s.closedChan)

	listener, err := net.ListenTCP("tcp", s.addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	wg := sync.WaitGroup{}
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-s.serverCtx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			if errors.Is(err, net.ErrClosed) && s.serverCtx.Err() != nil {
				// expected error because server closed
				return nil
			}
			return err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			s.handleConn(conn)
		}()
	}
}

func (s *server) handleConn(conn *net.TCPConn) {
	wg := sync.WaitGroup{}
	defer wg.Wait()
	defer conn.Close()

	var (
		connReader io.Reader = conn
		connWriter io.Writer = conn
	)
	if debug {
		connReader = &debugReader{
			name: "raw",
			rd:   conn,
		}
		connWriter = &debugWriter{
			name: "raw",
			rd:   conn,
		}
	}

	// read spec
	cipherSpec, err := ReadCipherSpec(connReader)
	if err != nil {
		log.Println(err)
		return
	}

	var (
		cipherReader = cipherSpec.NewReader(connReader)
		reader       = util.GetBufferReader(MaxLineLength)
		writer       = cipherSpec.NewWriter(connWriter)
		bufLine      = util.GetBytes(MaxLineLength)[:0]
	)
	defer util.PutBufferReader(reader)
	defer util.PutBytes(bufLine)

	if debug {
		cipherReader = &debugReader{
			name: "plain",
			rd:   cipherReader,
		}
		writer = &debugWriter{
			name: "plain",
			rd:   writer,
		}
	}
	reader.Reset(cipherReader)

	for {
		var (
			sliceLine []byte
			n         int
		)

		sliceLine, err = reader.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			bufLine = append(bufLine, sliceLine...)
			continue
		}
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Println(err)
			}
			return
		}
		sliceLine = sliceLine[:len(sliceLine)-1]
		bufLine = append(bufLine, sliceLine...)

		err = func() error {
			ctx, cancel := context.WithTimeout(s.serverCtx, 30*time.Second)
			defer cancel()
			defer func() {
				bufLine = bufLine[:0]
			}()

			handlerReader := bytes.NewReader(bufLine)
			s.handler(ctx, writer, handlerReader)
			n, err = writer.Write([]byte("\n"))
			if err != nil {
				return err
			}
			if n != 1 {
				return errors.New("can't write newline")
			}
			return nil
		}()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Println(err)
			}
			return
		}
	}
}

func (s *server) Shutdown(ctx context.Context) error {
	if !s.status.CompareAndSwap(1, 2) {
		return nil
	}
	s.cancelCtx()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.closedChan:
	}
	return nil
}

type debugReader struct {
	name string
	rd   io.Reader
}

func (d *debugReader) Read(p []byte) (n int, err error) {
	n, err = d.rd.Read(p)
	log.Printf("debug reader %s %x", d.name, p)
	return
}

type debugWriter struct {
	name string
	rd   io.Writer
}

func (d *debugWriter) Write(p []byte) (n int, err error) {
	log.Printf("debug writer %s %x", d.name, p)
	return d.rd.Write(p)
}
