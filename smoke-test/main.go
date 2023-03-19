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

				var (
					pr, pw      = io.Pipe()
					ctx, cancel = context.WithCancel(ctxServer)
				)
				defer cancel()

				go func() {
					<-ctx.Done()
					_ = conn.Close()
				}()

				go func() {
					defer pw.Close()
					_, err := io.Copy(pw, conn)
					if err != nil {
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
	ctxServerCancel()
	_ = listener.Close()
	wgServer.Wait()
}
