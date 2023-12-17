package lrcp

import (
	"context"
	"github.com/stretchr/testify/assert"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

type handlerTest struct {
}

func (h *handlerTest) Handle(w io.Writer, r io.Reader) {
	_, _ = io.Copy(os.Stdout, r)
}

func TestName(t *testing.T) {
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr, err := net.ResolveUDPAddr("udp", "localhost:1234")
	assert.NoError(t, err)
	udpConn, err := net.DialUDP("udp", nil, addr)
	assert.NoError(t, err)

	pipeInR, pipeInW := io.Pipe()
	pipeOutR, pipeOutW := io.Pipe()
	sess := &session{
		handler:  &handlerTest{},
		sessId:   123,
		ctx:      ctx,
		cancel:   cancel,
		addr:     addr,
		udpConn:  udpConn,
		pipeInW:  pipeInW,
		pipeInR:  pipeInR,
		pipeOutW: pipeOutW,
		pipeOutR: pipeOutR,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		sess.looper()
	}()
	go sess.handleData(0, []byte("test1\n"))
	time.Sleep(1 * time.Microsecond)
	go sess.handleData(6, []byte("test2\n"))
	wg.Wait()
}
