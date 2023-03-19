package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
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

				ctx, cancel := context.WithCancel(ctxServer)
				defer cancel()

				go func() {
					<-ctx.Done()
					_ = conn.Close()
				}()

				// run blocking session
				err := handleSession(conn, ctx)
				if err != nil {
					log.Println(err)
					return
				}
			}()
		}
	}()

	<-signChan
	ctxServerCancel()
	_ = listener.Close()
	wgServer.Wait()
}

func handleSession(conn *net.TCPConn, ctx context.Context) error {
	db := make(map[int32]int32)
	buf := make([]byte, 9)
	for {
		n, err := io.ReadFull(conn, buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if n != 9 {
			return errors.New("invalid message length")
		}
		message, err := UnmarshalMessage(buf[:n])
		if err != nil {
			return err
		}
		switch msg := message.(type) {
		case *MessageInsert:
			// handle insert message

			if _, found := db[msg.Timestamp]; found {
				return errors.New("multiple prices with the same timestamp")
			}
			db[msg.Timestamp] = msg.Price

		case *MessageQuery:
			// handle query message

			var (
				total int
				count int
				res   Result
			)
			if msg.MaxTime >= msg.MinTime {
				for timestamp, price := range db {
					if timestamp >= msg.MinTime && timestamp <= msg.MaxTime {
						total += int(price)
						count++
					}
				}
			}
			if count > 0 {
				res = Result{MeanPrice: int32(total / count)}
			} else {
				res = Result{MeanPrice: 0}
			}
			resRaw, err := res.MarshalBytes()
			if err != nil {
				return err
			}
			n, err := conn.Write(resRaw)
			if err != nil {
				return err
			}
			if n != len(resRaw) {
				return errors.New("invalid response length")
			}
		}
	}
}
