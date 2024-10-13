package lrcp

import (
	"bytes"
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/anantadwi13/protohackers/7-line-reversal/util"
)

const (
	TimeoutRetransmission = 3 * time.Second
	TimeoutSession        = 60 * time.Second
)

type sessionManager struct {
	handler      Handler
	sessions     map[int32]*sessionUDP
	sessionsLock sync.RWMutex
}

func newSessionManager(handler Handler) *sessionManager {
	return &sessionManager{
		handler:  handler,
		sessions: make(map[int32]*sessionUDP),
	}
}

func (s *sessionManager) GetOrCreate(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, sessionNumber int32) *sessionUDP {
	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()

	session, found := s.sessions[sessionNumber]
	if found {
		return session
	}

	ctx, cancelCtx := context.WithCancel(ctx)
	session = &sessionUDP{
		ctx:            ctx,
		cancelCtx:      cancelCtx,
		sessionManager: s,
		sessionNumber:  sessionNumber,
		handler:        s.handler,
		clientAddr:     addr,
		incomingChan:   make(chan []byte, 100), // todo: need buffered chan?
		outgoingChan:   make(chan []byte, 100), // todo: need buffered chan?
		signalAck:      make(chan int32),
	}

	log.Println("new session", sessionNumber, addr.IP.String(), addr.Port)

	session.wg.Add(1)
	go func() {
		defer session.wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case buf := <-session.incomingChan:
				func() {
					defer util.PoolPut(buf)

					handlerCtx, handlerCtxCancel := context.WithCancel(ctx)
					defer handlerCtxCancel()

					bufOutgoing := util.PoolGet(MaxLineLength)[:0]
					bufOutgoingWriter := bytes.NewBuffer(bufOutgoing)
					log.Printf("handler %d. %s", session.sessionNumber, buf)
					session.handler(handlerCtx, bufOutgoingWriter, bytes.NewReader(buf))

					select {
					case session.outgoingChan <- bufOutgoingWriter.Bytes():
					case <-ctx.Done():
						return
					}
				}()
			}
		}
	}()
	session.wg.Add(1)
	go func() {
		defer session.wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case outBuf := <-session.outgoingChan:
				func() {
					defer util.PoolPut(outBuf)

					buf := append(outBuf, '\n')

					session.lock.Lock()
					initLen := session.lenTotalOutgoing
					session.shouldBeSent = session.lenTotalOutgoing + int32(len(buf))
					session.lock.Unlock()
					curPos := int32(0)

					var (
						timerRetransmit = time.NewTimer(TimeoutRetransmission)
						timerSession    = time.NewTimer(TimeoutSession)
						wgLocal         = sync.WaitGroup{}
					)
					defer timerRetransmit.Stop()
					defer timerSession.Stop()
					defer wgLocal.Wait()

					for {
						if curPos >= int32(len(buf)) {
							return
						}
						if ctx.Err() != nil {
							return
						}

						burstPos := int32(0)
						for curPos+burstPos < int32(len(buf)) {
							msg := &MessageData{
								sessionNumber: session.sessionNumber,
								Pos:           initLen + curPos + burstPos,
								Data:          buf[curPos+burstPos:],
							}
							for msgRawSize := msg.MarshalSize(); msgRawSize > MTU; msgRawSize = msg.MarshalSize() {
								// slice packet
								maxLen := len(msg.Data) * 9 / 10
								msg.Data = msg.Data[:min(MTU, maxLen)]
							}
							burstPos += int32(len(msg.Data))

							dispatcher(ctx, conn, msg, session.clientAddr)
						}

						// wait acknowledge
						select {
						case ackLen := <-session.signalAck:
							curPos = ackLen - initLen
						case <-ctx.Done():
							return
						case <-timerSession.C:
							session.close(ctx, conn)
							return
						case <-timerRetransmit.C:
							timerRetransmit.Reset(TimeoutRetransmission)
							// retry
						}
					}
				}()
			}
		}
	}()
	s.sessions[sessionNumber] = session
	return session
}

func (s *sessionManager) Get(ctx context.Context, sessionNumber int32) *sessionUDP {
	s.sessionsLock.RLock()
	defer s.sessionsLock.RUnlock()
	return s.sessions[sessionNumber]
}

func (s *sessionManager) Remove(ctx context.Context, sessionNumber int32) {
	s.sessionsLock.Lock()
	sess, found := s.sessions[sessionNumber]
	if !found || sess == nil {
		s.sessionsLock.Unlock()
		return
	}
	delete(s.sessions, sessionNumber)
	s.sessionsLock.Unlock()

	sess.cancelCtx()
	sess.wg.Wait()

	log.Printf("session %d closed", sessionNumber)
}

func (s *sessionManager) Shutdown(ctx context.Context) {
	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()

	wgLocal := sync.WaitGroup{}
	defer wgLocal.Wait()

	for _, sess := range s.sessions {
		sess := sess
		wgLocal.Add(1)
		go func() {
			defer wgLocal.Done()
			sess.cancelCtx()
			sess.wg.Wait()
		}()
	}
	s.sessions = make(map[int32]*sessionUDP)
}

type sessionUDP struct {
	sessionManager *sessionManager

	sessionNumber int32
	handler       Handler
	clientAddr    *net.UDPAddr

	lock      sync.Mutex
	wg        sync.WaitGroup
	ctx       context.Context
	cancelCtx context.CancelFunc

	lenTotalIncoming int32
	lenTotalOutgoing int32
	shouldBeSent     int32

	incomingChan chan []byte
	outgoingChan chan []byte
	signalAck    chan int32

	incomingBuf []byte // max size 10K

	lockIncomingData sync.Mutex // serialize incoming data
}

func (s *sessionUDP) HandleIncoming(conn *net.UDPConn, msg Message) {
	switch m := msg.(type) {
	case *MessageConnect:
		res := &MessageAck{
			sessionNumber: s.sessionNumber,
		}
		s.lock.Lock()
		res.Length = s.lenTotalIncoming
		s.lock.Unlock()

		dispatcher(s.ctx, conn, res, s.clientAddr)
	case *MessageData:
		s.lock.Lock()
		if s.lenTotalIncoming < m.Pos || // data skipped
			s.lenTotalIncoming-m.Pos >= int32(len(m.Data)) { // no new data
			s.lock.Unlock()
			res := &MessageAck{
				sessionNumber: s.sessionNumber,
				Length:        s.lenTotalIncoming,
			}
			dispatcher(s.ctx, conn, res, s.clientAddr)
			return
		}

		m.Data = m.Data[s.lenTotalIncoming-m.Pos:]
		s.lenTotalIncoming += int32(len(m.Data))
		curTotalIncoming := s.lenTotalIncoming
		s.lock.Unlock()

		err := s.readNewLineFromMessage(m)
		if err != nil {
			return
		}

		res := &MessageAck{
			sessionNumber: s.sessionNumber,
			Length:        curTotalIncoming,
		}
		dispatcher(s.ctx, conn, res, s.clientAddr)
	case *MessageAck:
		s.lock.Lock()
		if m.Length <= s.lenTotalOutgoing {
			s.lock.Unlock()
			// already acked, do nothing
			return
		}
		if m.Length > s.shouldBeSent {
			s.lock.Unlock()
			// peer misbehaving, close session
			s.close(s.ctx, conn)
			return
		}

		s.lenTotalOutgoing = m.Length
		s.lock.Unlock()

		select {
		case s.signalAck <- m.Length:
		case <-s.ctx.Done():
			return
		}
	case *MessageClose:
		s.close(s.ctx, conn)
	}
}

func (s *sessionUDP) readNewLineFromMessage(m *MessageData) error {
	s.lockIncomingData.Lock()
	defer s.lockIncomingData.Unlock()

	var (
		curPos       = 0
		newLinePos   = len(m.Data)
		foundNewLine = false
	)

	for {
		if curPos >= len(m.Data) {
			break
		}

		if s.ctx.Err() != nil {
			return s.ctx.Err()
		}

		if s.incomingBuf == nil {
			s.incomingBuf = util.PoolGet(MaxLineLength)[:0]
		}

		for idx, c := range m.Data[curPos:] {
			if c == '\n' {
				newLinePos = curPos + idx
				foundNewLine = true
				break
			}
		}

		curBufLen := len(s.incomingBuf)
		s.incomingBuf = s.incomingBuf[:curBufLen+(newLinePos-curPos)] // without newline
		n := copy(s.incomingBuf[curBufLen:], m.Data[curPos:])

		if foundNewLine {
			select {
			case s.incomingChan <- s.incomingBuf:
			case <-s.ctx.Done():
				return s.ctx.Err()
			}
			s.incomingBuf = nil
			foundNewLine = false
			newLinePos = len(m.Data)
			n++ // to skip newline char
		}

		curPos += n
	}
	return nil
}

func (s *sessionUDP) close(ctx context.Context, conn *net.UDPConn) {
	s.sessionManager.Remove(ctx, s.sessionNumber)
	res := &MessageClose{
		sessionNumber: s.sessionNumber,
	}
	dispatcher(ctx, conn, res, s.clientAddr)
}
