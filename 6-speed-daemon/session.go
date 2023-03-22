package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type SessionType string

const (
	SessionTypeUnknown    SessionType = ""
	SessionTypeCamera     SessionType = "camera"
	SessionTypeDispatcher SessionType = "dispatcher"
)

type Session struct {
	server        *Server
	connWriteLock sync.Mutex
	conn          *net.TCPConn
	wg            sync.WaitGroup

	heartBeatLock sync.RWMutex
	heartBeatID   uint8

	sessID              int64
	sessLock            sync.RWMutex
	sessType            SessionType
	sessCameraRoad      uint16
	sessCameraMile      uint16
	sessCameraLimit     uint16
	sessDispatcherRoads []uint16
}

func (s *Session) GetDispatcherInfo() (sessID int64, roads []uint16, err error) {
	s.sessLock.RLock()
	defer s.sessLock.RUnlock()

	if s.sessType != SessionTypeDispatcher {
		return 0, nil, errors.New("not a dispatcher client")
	}

	return s.sessID, s.sessDispatcherRoads, nil
}

func (s *Session) GetCameraInfo() (sessID int64, road uint16, mile uint16, limit uint16, err error) {
	s.sessLock.RLock()
	defer s.sessLock.RUnlock()

	if s.sessType != SessionTypeCamera {
		return 0, 0, 0, 0, errors.New("not a dispatcher client")
	}

	return s.sessID, s.sessCameraRoad, s.sessCameraMile, s.sessCameraLimit, nil
}

func (s *Session) Start(ctx context.Context) error {
	s.sessID = time.Now().UnixNano()
	defer log.Println("client disconnected", s.sessID)
	defer s.wg.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Println("connecting clients", s.sessID)
	defer log.Println("disconnecting client", s.sessID)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		<-ctx.Done()
		_ = s.conn.Close()
	}()

	for {
		reqMsg, err := UnmarshalMessage(s.conn)
		if err != nil {
			return err
		}

		resMsg, err := s.HandleRequest(ctx, reqMsg)
		if err != nil {
			return err
		}
		if resMsg == nil {
			continue
		}
		s.connWriteLock.Lock()
		err = resMsg.Marshal(s.conn)
		s.connWriteLock.Unlock()
		if err != nil {
			return err
		}
	}
}

func (s *Session) HandleRequest(ctx context.Context, req Message) (res Message, err error) {
	defer func() {
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				res = &Error{Message: err.Error()}
				err = nil
			}
		}
	}()

	switch r := req.(type) {
	case *Plate:
		err := s.server.ProcessPlate(s, r)
		if err != nil {
			return nil, err
		}
	case *WantHeartbeat:
		s.wg.Add(1)
		go s.HandleHeartBeat(ctx, r)
	case *IAmCamera:
		err := s.HandleClientRegistration(ctx, req)
		if err != nil {
			return nil, err
		}
	case *IAmDispatcher:
		err := s.HandleClientRegistration(ctx, req)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("not a valid request")
	}

	return
}

func (s *Session) HandleClientRegistration(ctx context.Context, req Message) error {
	s.sessLock.Lock()
	defer s.sessLock.Unlock()

	if s.sessType != SessionTypeUnknown {
		return errors.New(fmt.Sprintf("session already identified as %s", s.sessType))
	}

	switch r := req.(type) {
	case *IAmCamera:
		s.sessType = SessionTypeCamera
		s.sessCameraRoad = r.Road
		s.sessCameraMile = r.Mile
		s.sessCameraLimit = r.Limit
	case *IAmDispatcher:
		s.sessType = SessionTypeDispatcher
		s.sessDispatcherRoads = r.Roads

		isCancel := make(chan struct{})
		isCancelOnce := &sync.Once{}
		s.wg.Add(len(r.Roads))
		for _, road := range r.Roads {
			// creating listeners
			go func(road uint16) {
				defer s.wg.Done()
				defer func() {
					// signal session to close all listeners
					isCancelOnce.Do(func() {
						close(isCancel)
					})
				}()
				for {
					select {
					case <-isCancel:
						return
					case <-ctx.Done():
						return
					case ticket := <-s.server.GetDispatcherChan(road):
						if ticket == nil {
							continue
						}
						s.connWriteLock.Lock()
						err := ticket.Marshal(s.conn)
						s.connWriteLock.Unlock()
						if err != nil {
							log.Println("unable to send ticket", err)
							return
						}
					}
				}
			}(road)
		}
	}

	return nil
}

func (s *Session) HandleHeartBeat(ctx context.Context, heartBeatReq *WantHeartbeat) {
	defer s.wg.Done()

	if heartBeatReq == nil {
		return
	}

	hb := &Heartbeat{}

	s.heartBeatLock.Lock()
	s.heartBeatID++
	hbID := s.heartBeatID
	s.heartBeatLock.Unlock()

	if heartBeatReq.Interval <= 0 {
		// stop heartbeat
		return
	}

	interval := time.Duration(heartBeatReq.Interval) * 100 * time.Millisecond

	heartBeatInfo := func() uint8 {
		s.heartBeatLock.RLock()
		defer s.heartBeatLock.RUnlock()

		return s.heartBeatID
	}

	for {
		id := heartBeatInfo()
		if id != hbID {
			return
		}

		s.connWriteLock.Lock()
		err := hb.Marshal(s.conn)
		s.connWriteLock.Unlock()
		if err != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
