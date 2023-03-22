package main

import (
	"context"
	"errors"
	"io"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
)

func main() {
	var (
		ctxServer, ctxServerCancel = context.WithCancel(context.Background())
		wgServer                   = sync.WaitGroup{}
		signChan                   = make(chan os.Signal, 1)
	)
	defer ctxServerCancel()

	s := &Server{}

	signal.Notify(signChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)

	addr, err := net.ResolveTCPAddr("tcp", ":8000")
	if err != nil {
		panic(err)
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}

	wgServer.Add(1)
	go func() {
		defer wgServer.Done()

		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					break
				}
				log.Println(err)
				continue
			}

			wgServer.Add(1)
			go func() {
				defer wgServer.Done()
				defer conn.Close()

				session := &Session{
					server: s,
					conn:   conn,
				}
				err := session.Start(ctxServer)
				if err != nil {
					if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
						log.Println(err)
					}
					return
				}
			}()
		}
	}()

	<-signChan
	go func() {
		<-signChan
		panic("force shutdown")
	}()
	ctxServerCancel()
	_ = listener.Close()
	wgServer.Wait()
}

type PositionLog struct {
	Mile      uint16
	Timestamp uint32
}

type Server struct {
	DatabaseLogsLock sync.RWMutex
	DatabaseLogs     map[string]map[uint16][]PositionLog // plate, road, []PositionLog

	DispatcherChanLock sync.Mutex
	DispatcherChan     map[uint16]chan *Ticket

	TicketSentLock sync.Mutex
	TicketSent     map[string]map[uint32]struct{} // idx => plate, day
}

func (s *Server) ProcessPlate(sess *Session, plate *Plate) error {
	_, road, mile, limit, err := sess.GetCameraInfo()
	if err != nil {
		return err
	}

	curPosition := PositionLog{
		Mile:      mile,
		Timestamp: plate.Timestamp,
	}
	log.Println(road, mile, limit, plate.Plate, plate.Timestamp, "process plate")
	defer log.Println(road, mile, limit, plate.Plate, plate.Timestamp, "process plate done")

	// insert current data
	s.DatabaseLogsLock.Lock()
	if s.DatabaseLogs == nil {
		s.DatabaseLogs = map[string]map[uint16][]PositionLog{}
	}
	if s.DatabaseLogs[plate.Plate] == nil {
		s.DatabaseLogs[plate.Plate] = map[uint16][]PositionLog{}
	}
	s.DatabaseLogs[plate.Plate][road] = append(s.DatabaseLogs[plate.Plate][road], curPosition)
	sort.SliceStable(s.DatabaseLogs[plate.Plate][road], func(i, j int) bool {
		log1 := s.DatabaseLogs[plate.Plate][road][i]
		log2 := s.DatabaseLogs[plate.Plate][road][j]
		return log1.Timestamp < log2.Timestamp && log1.Mile < log2.Mile
	})
	s.DatabaseLogsLock.Unlock()

	// calculate speed
	s.DatabaseLogsLock.RLock()
	for _, position := range s.DatabaseLogs[plate.Plate][road] {
		if position.Timestamp == curPosition.Timestamp && position.Mile == curPosition.Mile {
			continue
		}

		var (
			pos1 PositionLog
			pos2 PositionLog
		)

		if curPosition.Timestamp > position.Timestamp {
			pos1 = position
			pos2 = curPosition
		} else {
			pos1 = curPosition
			pos2 = position
		}

		// calculate speed
		speed := math.Abs(float64(pos2.Mile)-float64(pos1.Mile)) / ((float64(pos2.Timestamp) - float64(pos1.Timestamp)) / 3600)
		speedRounded := uint16(speed)
		speedPrecision := speed - float64(speedRounded)

		log.Printf("calculating speed %#v %#v %v %v %v %v\n", pos1, pos2, speed, speedRounded, speedPrecision, limit)

		if speedRounded < limit || (speedRounded == limit && speedPrecision < 0.5) {
			continue
		}

		// send ticket
		ticket := &Ticket{
			Plate:      plate.Plate,
			Road:       road,
			Mile1:      pos1.Mile,
			Timestamp1: pos1.Timestamp,
			Mile2:      pos2.Mile,
			Timestamp2: pos2.Timestamp,
			Speed:      uint16(speed * 100),
		}
		err = s.SendTicket(ticket)
		if err != nil {
			s.DatabaseLogsLock.RUnlock()
			return err
		}
	}
	s.DatabaseLogsLock.RUnlock()

	return nil
}

func (s *Server) SendTicket(ticket *Ticket) error {
	s.TicketSentLock.Lock()
	defer s.TicketSentLock.Unlock()

	if s.TicketSent == nil {
		s.TicketSent = map[string]map[uint32]struct{}{}
	}
	if s.TicketSent[ticket.Plate] == nil {
		s.TicketSent[ticket.Plate] = map[uint32]struct{}{}
	}

	day1 := ticket.Timestamp1 / 86400
	day2 := ticket.Timestamp2 / 86400

	for i := day1; i <= day2; i++ {
		_, found := s.TicketSent[ticket.Plate][i]
		if found {
			// do not send the ticket
			return nil
		}
	}
	for i := day1; i <= day2; i++ {
		s.TicketSent[ticket.Plate][i] = struct{}{}
	}

	log.Printf("sending ticket %#v\n", ticket)
	s.getDispatcherChan(ticket.Road) <- ticket
	return nil
}

func (s *Server) getDispatcherChan(road uint16) chan *Ticket {
	s.DispatcherChanLock.Lock()
	defer s.DispatcherChanLock.Unlock()

	if s.DispatcherChan == nil {
		s.DispatcherChan = map[uint16]chan *Ticket{}
	}
	if s.DispatcherChan[road] == nil {
		s.DispatcherChan[road] = make(chan *Ticket, 1024)
	}
	return s.DispatcherChan[road]
}

func (s *Server) GetDispatcherChan(road uint16) <-chan *Ticket {
	return s.getDispatcherChan(road)
}
