package lrcp

import (
	"context"
	"errors"
	"io"
	"log"
	"math"
	"net"
	"sync"
	"time"
)

var (
	maxMessageSize        = 999
	maxSession            = math.MaxInt32
	maxDataSize           = math.MaxInt32
	timeoutRetransmission = 3 * time.Second
	timeoutSessionExpiry  = 60 * time.Second
)

var (
	messageBufPool = sync.Pool{New: func() any { return make([]byte, maxMessageSize) }}
)

type LRCPHandler interface {
	Handle(w io.Writer, r io.Reader)
}

type LRCP struct {
	udpConn *net.UDPConn

	sessions     map[int32]*session
	sessionsLock sync.RWMutex

	ctx       context.Context
	ctxCancel context.CancelFunc

	handler LRCPHandler
}

func NewLRCP(ctx context.Context, udpConn *net.UDPConn, handler LRCPHandler) *LRCP {
	ctx, cancel := context.WithCancel(ctx)

	if handler == nil {
		handler = &handlerMock{}
	}
	lrcp := &LRCP{
		udpConn:   udpConn,
		ctx:       ctx,
		ctxCancel: cancel,
		handler:   handler,
		sessions:  make(map[int32]*session),
	}
	go lrcp.looper()
	return lrcp
}

func (l *LRCP) looper() {
	defer l.ctxCancel()

	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() {
		defer wg.Done()
		<-l.ctx.Done()
		_ = l.udpConn.Close()
	}()

	go func() {
		defer wg.Done()
		var (
			err     error
			wgLocal sync.WaitGroup
		)
		for {
			if l.ctx.Err() != nil {
				break
			}

			err = func() error {
				buf := messageBufPool.Get().([]byte)

				n, addr, err := l.udpConn.ReadFromUDP(buf)
				if err != nil {
					return err
				}

				buf = buf[:n]

				wgLocal.Add(1)
				go func() {
					defer func() {
						messageBufPool.Put(buf[:cap(buf)])
						wgLocal.Done()
					}()

					mReq := &message{}
					err := mReq.unmarshal(buf)
					if err != nil {
						log.Println(err)
						return
					}

					var (
						sess  *session
						found bool
					)

					switch mReq.MessageType {
					case messageTypeConnect:
						l.sessionsLock.Lock()
						sess, found = l.sessions[mReq.SessionId]
						if !found {
							ctx, cancel := context.WithCancel(l.ctx)
							pipeInR, pipeInW := io.Pipe()
							pipeOutR, pipeOutW := io.Pipe()
							sess = &session{
								ctx:      ctx,
								cancel:   cancel,
								handler:  l.handler,
								sessId:   mReq.SessionId,
								pipeInR:  pipeInR,
								pipeInW:  pipeInW,
								pipeOutR: pipeOutR,
								pipeOutW: pipeOutW,
								addr:     addr,
								udpConn:  l.udpConn,
							}
							wgLocal.Add(1)
							go func() {
								defer wgLocal.Done()
								sess.looper()
							}()
							l.sessions[mReq.SessionId] = sess
						}
						l.sessionsLock.Unlock()
						sess.handleConnect()
					case messageTypeData:
						l.sessionsLock.RLock()
						sess, found = l.sessions[mReq.SessionId]
						l.sessionsLock.RUnlock()
						if !found {
							log.Println("session not found")
							l.closeSession(mReq.SessionId, addr)
							return
						}
						sess.handleData(mReq.Length, mReq.Data)
					case messageTypeAck:
						l.sessionsLock.RLock()
						sess, found = l.sessions[mReq.SessionId]
						l.sessionsLock.RUnlock()
						if !found {
							log.Println("session not found")
							l.closeSession(mReq.SessionId, addr)
							return
						}
						sess.handleAck(mReq.Length)
					case messageTypeClose:
						l.sessionsLock.Lock()
						sess, found = l.sessions[mReq.SessionId]
						delete(l.sessions, mReq.SessionId)
						l.sessionsLock.Unlock()
						if !found {
							log.Println("session not found")
							l.closeSession(mReq.SessionId, addr)
							return
						}
						sess.handleClose()
					}
				}()

				return nil
			}()
			if errors.Is(err, net.ErrClosed) {
				log.Println("looper err.", err)
				break
			}
		}
		wgLocal.Wait()
	}()

	go func() {
		defer wg.Done()
	}()

	wg.Wait()
}

func (l *LRCP) closeSession(sessId int32, addr *net.UDPAddr) {
	buf := messageBufPool.Get().([]byte)
	defer func() {
		messageBufPool.Put(buf[:cap(buf)])
	}()
	mRes := &message{
		MessageType: messageTypeClose,
		SessionId:   sessId,
	}
	n, _, err := mRes.marshal(buf)
	if err != nil {
		log.Println("error closeSession", err)
		return
	}
	buf = buf[:n]
	n, err = l.udpConn.WriteToUDP(buf, addr)
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			return
		}
		log.Println("error WriteToUDP", err)
		return
	}
}

type session struct {
	handler  LRCPHandler
	sessId   int32
	ctx      context.Context
	cancel   context.CancelFunc
	addr     *net.UDPAddr
	udpConn  *net.UDPConn
	pipeInW  *io.PipeWriter
	pipeInR  *io.PipeReader
	pipeOutW *io.PipeWriter
	pipeOutR *io.PipeReader

	// only one handler can run at a time
	handlerLock sync.Mutex

	lock   sync.RWMutex
	posIn  int32
	posOut int32
}

func (s *session) runHandler(wg *sync.WaitGroup, r io.Reader) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.handlerLock.Lock()
		defer s.handlerLock.Unlock()
		s.handler.Handle(s.pipeOutW, r)
	}()
}

func (s *session) looper() {
	var (
		wg = &sync.WaitGroup{}
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		<-s.ctx.Done()
		_ = s.pipeInR.Close()
		_ = s.pipeInW.Close()
		_ = s.pipeOutR.Close()
		_ = s.pipeOutW.Close()
	}()

	go func() {
		defer wg.Done()
		defer s.cancel()

		var (
			appPipeR, appPipeW = io.Pipe()
			buf                = make([]byte, 4096)
		)
		s.runHandler(wg, appPipeR)
		for {
			n, err := s.pipeInR.Read(buf)
			if err != nil {
				return
			}
			var (
				lastWrite = 0
			)
			for i := lastWrite; i < n; i++ {
				if buf[i] == '\n' {
					appPipeW.Write(buf[lastWrite : i+1])
					lastWrite = i + 1
					appPipeW.Close()
					appPipeR, appPipeW = io.Pipe()
					s.runHandler(wg, appPipeR)
				}
			}
			appPipeW.Write(buf[lastWrite:n])
		}
	}()

	go func() {
		defer wg.Done()
		defer s.cancel()

		// todo
	}()

	wg.Wait()
}

func (s *session) handleConnect() {
	s.lock.RLock()
	pos := s.posIn
	s.lock.RUnlock()
	s.sendAck(pos)
}

func (s *session) handleData(pos int32, data []byte) {
	var posIn int32
	func() {
		s.lock.Lock()
		defer s.lock.Unlock()

		if pos == s.posIn {
			n, err := s.pipeInW.Write(data)
			if err != nil {
				log.Println("error handleData bufIn.Write", err)
			}
			s.posIn += int32(n)
			posIn = s.posIn
		}
	}()
	s.sendAck(posIn)
}

func (s *session) handleAck(length int32) {
	// todo
}

func (s *session) sendAck(pos int32) {
	buf := messageBufPool.Get().([]byte)
	defer func() {
		messageBufPool.Put(buf[:cap(buf)])
	}()
	m := &message{
		MessageType: messageTypeAck,
		SessionId:   s.sessId,
		Length:      pos,
	}
	n, _, err := m.marshal(buf)
	if err != nil {
		log.Println("err marshal message", err)
		return
	}
	buf = buf[:n]
	n, err = s.udpConn.WriteToUDP(buf, s.addr)
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			return
		}
		log.Println("err WriteToUDP", err)
		return
	}
}

func (s *session) handleClose() {
	s.cancel()

	buf := messageBufPool.Get().([]byte)
	defer func() {
		messageBufPool.Put(buf[:cap(buf)])
	}()
	mRes := &message{
		MessageType: messageTypeClose,
		SessionId:   s.sessId,
	}
	n, _, err := mRes.marshal(buf)
	if err != nil {
		log.Println("error session.close", err)
		return
	}
	buf = buf[:n]
	n, err = s.udpConn.WriteToUDP(buf, s.addr)
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			return
		}
		log.Println("error WriteToUDP", err)
		return
	}
}

type handlerMock struct {
}

func (h *handlerMock) Handle(w io.Writer, r io.Reader) {
	_, _ = io.Copy(io.Discard, r)
}
