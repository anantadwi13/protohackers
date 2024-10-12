package lrcp

import (
	"bytes"
	"context"
	"github.com/anantadwi13/protohackers/7-line-reversal/util"
	"log"
	"net"
	"sync"
	"time"
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
		incomingChan:   make(chan []byte),
		outgoingChan:   make(chan []byte),
		signalAck:      make(chan int32),
	}

	log.Println("new session", addr.IP.String(), addr.Port)
	// todo
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
					log.Println("incoming", string(buf))

					bufOutgoing := util.PoolGet(MaxLineLength)[:0]
					bufOutgoingWriter := bytes.NewBuffer(bufOutgoing)
					session.handler(bufOutgoingWriter, bytes.NewReader(buf))

					session.outgoingChan <- bufOutgoingWriter.Bytes()
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
					log.Println("outgoing", string(buf), buf)

					session.lock.Lock()
					initLen := session.lenTotalOutgoing
					session.shouldBeSent = session.lenTotalOutgoing + int32(len(buf))
					session.lock.Unlock()
					curPos := int32(0)
					var msg *MessageData

					var (
						timerRetransmit = time.NewTimer(TimeoutRetransmission)
						timerSession    = time.NewTimer(TimeoutSession)
					)
					defer timerRetransmit.Stop()
					defer timerSession.Stop()

					for {
						if curPos >= int32(len(buf)) {
							return
						}
						if ctx.Err() != nil {
							return
						}

						if msg == nil {
							msg = &MessageData{
								sessionNumber: session.sessionNumber,
								Pos:           curPos + initLen,
								Data:          buf[curPos:],
							}
							for msg.MarshalSize() > MTU {
								// todo slice packet
								msg.Data = msg.Data[:min(MTU, len(msg.Data)/2)]
							}
						}
						session.dispatcher(ctx, conn, msg)

						// wait acknowledge
						select {
						case ackLen := <-session.signalAck:
							curPos += ackLen - initLen
							msg = nil
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

	log.Printf("session %d closed", sessionNumber)

	sess.cancelCtx()
	sess.wg.Wait()
}

func (s *sessionManager) Shutdown(ctx context.Context) {
	// todo
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
}

/*
	1234567890
	len 0
	data pos 0 1234
	ack len 4
	data pos 4 567
	ack len 7
	data pos 5 67890
	ack len 10
*/

func (s *sessionUDP) HandleIncoming(ctx context.Context, conn *net.UDPConn, msg Message) {
	s.lock.Lock()
	defer s.lock.Unlock()

	switch m := msg.(type) {
	case *MessageConnect:
		res := &MessageAck{
			sessionNumber: s.sessionNumber,
			Length:        s.lenTotalIncoming,
		}
		s.dispatcher(ctx, conn, res)
	case *MessageData:
		if s.lenTotalIncoming < m.Pos || // data skipped
			s.lenTotalIncoming-m.Pos >= int32(len(m.Data)) { // no new data
			res := &MessageAck{
				sessionNumber: s.sessionNumber,
				Length:        s.lenTotalIncoming,
			}
			s.dispatcher(ctx, conn, res)
			return
		}
		newData := m.Data[s.lenTotalIncoming-m.Pos:]
		s.lenTotalIncoming += int32(len(newData))

		if s.incomingBuf == nil {
			s.incomingBuf = util.PoolGet(MaxLineLength)
			s.incomingBuf = s.incomingBuf[:0]
		}
		curLineLen := len(s.incomingBuf)
		newlineIdx := len(newData)
		for idx, c := range newData {
			if c == '\n' {
				newlineIdx = idx
				break
			}
		}

		s.incomingBuf = s.incomingBuf[:curLineLen+newlineIdx] // should not contain newline
		copy(s.incomingBuf[curLineLen:], newData)

		// found a new line
		if newlineIdx != len(newData) {
			// todo
			s.incomingChan <- s.incomingBuf
			s.incomingBuf = nil
		}

		// handle rest data
		if newlineIdx+1 < len(newData) {
			s.incomingBuf = util.PoolGet(MaxLineLength)
			s.incomingBuf = s.incomingBuf[:len(newData)-(newlineIdx+1)]
			copy(s.incomingBuf, newData[newlineIdx+1:])
		}
		res := &MessageAck{
			sessionNumber: s.sessionNumber,
			Length:        s.lenTotalIncoming,
		}
		s.dispatcher(ctx, conn, res)
	case *MessageAck:
		if m.Length <= s.lenTotalOutgoing {
			// already acked, do nothing
			return
		}
		if m.Length > s.shouldBeSent {
			// peer misbehaving, close session
			s.close(ctx, conn)
			return
		}

		s.lenTotalOutgoing = m.Length
		s.signalAck <- m.Length
	case *MessageClose:
		s.close(ctx, conn)
	}
}

func (s *sessionUDP) close(ctx context.Context, conn *net.UDPConn) {
	s.sessionManager.Remove(ctx, s.sessionNumber)
	res := &MessageClose{
		sessionNumber: s.sessionNumber,
	}
	s.dispatcher(ctx, conn, res)
}

func (s *sessionUDP) dispatcher(ctx context.Context, conn *net.UDPConn, msg Message) {
	buf := util.PoolGet(MTU)
	defer util.PoolPut(buf)

	bufWriter := bytes.NewBuffer(buf) // todo put bytes.Buffer to pool?
	bufWriter.Reset()

	err := msg.Marshal(bufWriter)
	if err != nil {
		log.Printf("dispatcher Marshal. err: %s", err)
		return
	}

	log.Println("dispatcher", string(bufWriter.Bytes()), bufWriter.Bytes(), bufWriter.Len())

	_, err = conn.WriteToUDP(bufWriter.Bytes(), s.clientAddr)
	if err != nil {
		log.Printf("dispatcher WriteToUDP. err: %s", err)
		return
	}
}
