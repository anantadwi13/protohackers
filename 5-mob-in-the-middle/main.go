package main

import (
	"bufio"
	"bytes"
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
					wg          = sync.WaitGroup{}
					ctx, cancel = context.WithCancel(ctxServer)
				)
				defer cancel()

				chatServerAddr, err := net.ResolveTCPAddr("tcp", "chat.protohackers.com:16963")
				if err != nil {
					log.Println(err)
					return
				}
				chatServerConn, err := net.DialTCP("tcp", nil, chatServerAddr)
				if err != nil {
					log.Println(err)
					return
				}

				go func() {
					<-ctx.Done()
					_ = conn.Close()
					_ = chatServerConn.Close()
				}()

				prCTS, pwCTS := io.Pipe() // client to server
				prSTC, pwSTC := io.Pipe() // server to client

				wg.Add(4)

				go func() {
					// client read
					defer wg.Done()
					defer cancel()
					defer pwCTS.Close()
					forward(ctx, pwCTS, conn, "client read", bogusCoinModifier("7YWHMfk9JZe0LM0g1ZauHuiSxhI"))
				}()

				go func() {
					// client write
					defer wg.Done()
					defer cancel()
					defer prSTC.Close()
					forward(ctx, conn, prSTC, "client write")
				}()

				go func() {
					// server read
					defer wg.Done()
					defer cancel()
					defer pwSTC.Close()
					forward(ctx, pwSTC, chatServerConn, "server read", bogusCoinModifier("7YWHMfk9JZe0LM0g1ZauHuiSxhI"))
				}()

				go func() {
					// server write
					defer wg.Done()
					defer cancel()
					defer prCTS.Close()
					forward(ctx, chatServerConn, prCTS, "server write")
				}()

				wg.Wait()
			}()
		}
	}()

	<-signChan
	ctxServerCancel()
	_ = listener.Close()
	wgServer.Wait()
}

func replaceBogusCoin(message []byte, replace []byte) (res []byte) {
	var (
		i int
		b byte
	)
	res = make([]byte, 0, len(message))

	for {
		if i >= len(message) {
			break
		}
		b = message[i]

		if b == '7' && (i == 0 || message[i-1] == ' ') {
			var (
				j    int
				b2   byte
				isBC = true
			)

			for {
				j++
				if j >= len(message[i:]) || message[i+j] == ' ' {
					break
				}

				b2 = message[i+j]
				if (b2 >= 'A' && b2 <= 'Z') || (b2 >= 'a' && b2 <= 'z') || (b2 >= '0' && b2 <= '9') {
					continue
				}
				isBC = false
				break
			}

			if isBC && j >= 26 && j <= 35 {
				res = append(res, replace...)
			} else {
				res = append(res, message[i:i+j]...)
			}
			i += j
			continue
		}

		res = append(res, b)
		i++
	}

	return res
}

func bogusCoinModifier(replace string) func(message []byte) []byte {
	replaceBytes := []byte(replace)
	return func(message []byte) []byte {
		return replaceBogusCoin(message, replaceBytes)
	}
}

func forward(
	ctx context.Context,
	dst io.Writer,
	src io.Reader,
	identifier string,
	modifiers ...func(message []byte) []byte,
) {
	var (
		err error
		buf = bufio.NewReader(src)
	)

	defer func() {
		if err != nil {
			log.Println(identifier, err)
		}
	}()

	for {
		if ctx.Err() != nil {
			err = ctx.Err()
			return
		}

		message, err := buf.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				err = nil
			}
			return
		}

		message = bytes.TrimRight(message, "\n")

		for _, modifier := range modifiers {
			if modifier == nil {
				continue
			}
			message = modifier(message)
		}

		message = append(message, '\n')

		n, err := dst.Write(message)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				err = nil
			}
			return
		}

		if n != len(message) {
			err = errors.New("invalid message length")
			return
		}
	}
}
