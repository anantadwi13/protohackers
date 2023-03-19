package main

import (
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
		wgServer = sync.WaitGroup{}
		signChan = make(chan os.Signal, 1)
	)

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

				pr, pw := io.Pipe()
				go func() {
					defer pw.Close()
					_, err := io.Copy(pw, conn)
					if err != nil {
						log.Println(err)
						return
					}
				}()
				_, err = io.Copy(conn, pr)
				if err != nil {
					log.Println(err)
					return
				}
			}()
		}
	}()

	<-signChan
	_ = listener.Close()
	wgServer.Wait()
}
