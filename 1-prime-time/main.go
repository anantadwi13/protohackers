package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/big"
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
					buf         = bufio.NewReader(conn)
					ctx, cancel = context.WithCancel(ctxServer)
				)
				defer cancel()

				go func() {
					<-ctx.Done()
					_ = conn.Close()
				}()

				for {
					req, err := buf.ReadBytes('\n')
					if err != nil {
						if len(req) <= 0 {
							return
						}
					}

					res, err := handler(req)
					if err != nil {
						log.Println(err)
						_, _ = conn.Write(res)
						return
					}
					_, err = conn.Write(res)
					if err != nil {
						return
					}
				}

			}()
		}
	}()

	<-signChan
	ctxServerCancel()
	_ = listener.Close()
	wgServer.Wait()
}

type Request struct {
	Method *string `json:"method"`
	Number *Number `json:"number"`
}

type Number struct {
	Val interface{}
}

func (n *Number) IsValid() bool {
	return n.Val != nil
}

func (n *Number) UnmarshalJSON(bytes []byte) error {
	if string(bytes) == "null" {
		return nil
	}

	vInt := big.NewInt(0)
	err := vInt.UnmarshalText(bytes)
	if err == nil {
		n.Val = vInt
		return nil
	}
	vFloat := big.NewFloat(0)
	err = vFloat.UnmarshalText(bytes)
	if err == nil {
		n.Val = vFloat
		return nil
	}

	return errors.New("invalid number")
}

type Response struct {
	Method string `json:"method"`
	Prime  bool   `json:"prime"`
}

func handler(rawReq []byte) (rawRes []byte, err error) {
	defer func() {
		if err != nil {
			rawRes = []byte("invalid request")
		}
	}()

	// construct request
	req := &Request{}
	err = json.Unmarshal(rawReq, req)
	if err != nil {
		return nil, err
	}

	// validate request
	if req.Method == nil || req.Number == nil ||
		*req.Method != "isPrime" {
		return nil, errors.New("invalid request")
	}

	// check prime
	isPrime := false
	if req.Number.IsValid() {
		switch val := req.Number.Val.(type) {
		case *big.Int:
			isPrime = val.ProbablyPrime(10)
		}
	}

	// generate response
	rawRes, err = json.Marshal(Response{
		Method: "isPrime",
		Prime:  isPrime,
	})
	if err != nil {
		return nil, err
	}

	rawRes = append(rawRes, '\n')
	return
}
