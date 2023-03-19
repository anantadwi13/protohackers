package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
)

func main() {
	var (
		ctxServer, ctxServerCancel = context.WithCancel(context.Background())
		wgServer                   = sync.WaitGroup{}
		signChan                   = make(chan os.Signal, 1)
		chatRoom                   = &ChatRoom{users: map[string]*Session{}, event: make(chan *Event, 100)}
	)
	defer ctxServerCancel()

	signal.Notify(signChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)

	addr, err := net.ResolveTCPAddr("tcp", ":8000")
	if err != nil {
		panic(err)
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}

	wgServer.Add(2)
	go func() {
		defer wgServer.Done()

		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					break
				}
				log.Println("accept conn", err)
				continue
			}

			wgServer.Add(1)
			go func() {
				defer wgServer.Done()
				defer conn.Close()

				ctx, cancel := context.WithCancel(ctxServer)
				defer cancel()

				go func() {
					<-ctx.Done()
					_ = conn.Close()
				}()

				// run blocking session
				s := Session{conn: conn, cr: chatRoom}
				err := s.run(ctx)
				if err != nil {
					log.Println("session run", err)
					return
				}
			}()
		}
	}()

	go func() {
		defer wgServer.Done()

		err := chatRoom.looper(ctxServer)
		if err != nil {
			log.Println("looper", err)
			return
		}
	}()

	<-signChan
	ctxServerCancel()
	_ = listener.Close()
	wgServer.Wait()
}

type EventType int

const (
	EventTypeUnknown EventType = iota
	EventTypeMessage
	EventTypeJoin
	EventTypeLeave
)

type Event struct {
	User    string
	Type    EventType
	Message string
}

type ChatRoom struct {
	users     map[string]*Session
	usersLock sync.RWMutex

	event chan *Event
}

func (c *ChatRoom) looper(ctx context.Context) error {
	var event *Event
	for {
		select {
		case <-ctx.Done():
			return nil
		case event = <-c.event:
			if event == nil {
				continue
			}
		}

		message := ""
		switch event.Type {
		case EventTypeJoin:
			message = fmt.Sprintf("* %s has entered the room", event.User)
		case EventTypeLeave:
			message = fmt.Sprintf("* %s has left the room", event.User)
		case EventTypeMessage:
			message = fmt.Sprintf("[%s] %s", event.User, event.Message)
		}

		c.usersLock.RLock()
		for _, userSess := range c.users {
			if userSess.name == event.User {
				continue
			}
			err := userSess.sendMessage(message)
			if err != nil {
				log.Println("looper", err)
			}
		}
		c.usersLock.RUnlock()
	}
}

func (c *ChatRoom) register(s *Session) error {
	c.usersLock.Lock()
	defer c.usersLock.Unlock()

	if _, exist := c.users[s.name]; exist {
		return errors.New("user already exist")
	}

	c.users[s.name] = s
	s.cr.event <- &Event{
		User: s.name,
		Type: EventTypeJoin,
	}
	log.Printf("%v has entered the room", s.name)
	userList := make([]string, 0, len(c.users))
	for name := range c.users {
		if name == s.name {
			continue
		}
		userList = append(userList, name)
	}
	sort.Strings(userList)
	presentNotif := "* The room contains: "
	for i, name := range userList {
		if i == 0 {
			presentNotif = fmt.Sprintf("%s%s", presentNotif, name)
		} else {
			presentNotif = fmt.Sprintf("%s, %s", presentNotif, name)
		}
	}
	err := s.sendMessage(presentNotif)
	if err != nil {
		return err
	}
	return nil
}

func (c *ChatRoom) leave(s *Session) {
	c.usersLock.Lock()
	defer c.usersLock.Unlock()

	delete(c.users, s.name)
	s.cr.event <- &Event{
		User: s.name,
		Type: EventTypeLeave,
	}
	log.Printf("%v has left the room", s.name)
}

type Session struct {
	cr *ChatRoom

	conn *net.TCPConn
	buf  *bufio.Reader

	name string
}

var regexName = regexp.MustCompile("^[0-9A-Za-z]{1,50}$")

func (s *Session) run(ctx context.Context) error {
	s.buf = bufio.NewReader(s.conn)

	err := s.register()
	if err != nil {
		return err
	}

	defer s.conn.Close()
	defer s.cr.leave(s)

	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg.Add(1)
	go func() {
		// reader
		defer wg.Done()
		defer cancel()

		for {
			msg, err := s.readMessage()
			if err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
					log.Printf("session reader: %s", err)
				}
				return
			}
			s.cr.event <- &Event{
				User:    s.name,
				Type:    EventTypeMessage,
				Message: msg,
			}
		}
	}()

	<-ctx.Done()
	wg.Wait()
	return nil
}

func (s *Session) register() error {
	err := s.sendMessage("Welcome to budgetchat! What shall I call you?")
	if err != nil {
		return err
	}
	name, err := s.readMessage()
	if err != nil {
		return err
	}
	if !regexName.MatchString(name) {
		return s.sendError("invalid user name", name)
	}
	s.name = name

	err = s.cr.register(s)
	if err != nil {
		return s.sendError(err.Error())
	}
	return nil
}

func (s *Session) readMessage() (string, error) {
	msgRaw, err := s.buf.ReadBytes('\n')
	if err != nil {
		return "", err
	}
	msg := strings.Trim(string(msgRaw), "\r\n")
	return msg, nil
}

func (s *Session) sendMessage(msg string) error {
	msg = strings.Trim(msg, "\r\n")
	msg += "\n"
	n, err := s.conn.Write([]byte(msg))
	if err != nil {
		return err
	}
	if n != len(msg) {
		return errors.New("invalid message length")
	}
	return nil
}

func (s *Session) sendError(errorMsg string, details ...any) error {
	err := s.sendMessage(errorMsg)
	if err != nil {
		return err
	}
	detailStr := ""
	for _, detail := range details {
		if detail == nil {
			continue
		}
		if detailStr == "" {
			detailStr = fmt.Sprintf(". %v", detail)
		} else {
			detailStr = fmt.Sprintf("%v, %v", detailStr, detail)
		}
	}
	return fmt.Errorf("%v%v", errorMsg, detailStr)
}
