package proto

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/anantadwi13/protohackers/9-job-centre/util"
	"github.com/rs/xid"
)

var (
	ErrNoJob                   = errors.New("error no job")
	ErrUnknown                 = errors.New("unknown error")
	ErrAlreadyStartedOrStopped = errors.New("server already started/stopped")
)

type Handler interface {
	PutJob(ctx context.Context, queue string, job any, priority uint32) (jobID uint32, err error)
	GetJob(ctx context.Context, queues []string, wait bool) (jobID uint32, job any, priority uint32, queue string, err error)
	DeleteJob(ctx context.Context, jobID uint32) (err error)
	AbortJob(ctx context.Context, jobID uint32) (err error)

	OnClientConnected(ctx context.Context, clientID string)
	OnClientDisconnected(ctx context.Context, clientID string)
}

type Server interface {
	Listen() error
	Shutdown(ctx context.Context) error
}

const bufSize = 4 << 10 // 4KB

type server struct {
	handler Handler
	addr    *net.TCPAddr

	status      atomic.Int32 // 0: init, 1: running, 2: stopped
	baseCtx     context.Context
	cancelCtx   context.CancelFunc
	stoppedChan chan struct{}
}

func NewServer(addr string, handler Handler) (Server, error) {
	if handler == nil {
		return nil, errors.New("handler is nil")
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &server{
		handler:     handler,
		addr:        tcpAddr,
		baseCtx:     ctx,
		cancelCtx:   cancel,
		stoppedChan: make(chan struct{}),
	}
	return s, nil
}

func (s *server) Listen() error {
	if !s.status.CompareAndSwap(0, 1) {
		return ErrAlreadyStartedOrStopped
	}

	defer close(s.stoppedChan)

	listener, err := net.ListenTCP("tcp", s.addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	wg := sync.WaitGroup{}
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-s.baseCtx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			if errors.Is(err, net.ErrClosed) && s.baseCtx.Err() == nil {
				return err
			}
			return nil
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			clientID := xid.New().String()
			ctx := context.WithValue(s.baseCtx, clientIDContextKey, clientID)

			ch := &connHandler{conn: conn, handler: s.handler, clientID: clientID}
			ch.looper(ctx)
		}()
	}
}

func (s *server) Shutdown(ctx context.Context) error {
	if !s.status.CompareAndSwap(1, 2) {
		return ErrAlreadyStartedOrStopped
	}

	s.cancelCtx()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.stoppedChan:
	}

	return nil
}

type contextKey string

var clientIDContextKey = contextKey("clientID")

func GetClientID(ctx context.Context) (string, error) {
	clientID, ok := ctx.Value(clientIDContextKey).(string)
	if !ok || clientID == "" {
		return "", errors.New("client ID not found in context")
	}
	return clientID, nil
}

type connHandler struct {
	clientID string
	conn     *net.TCPConn
	handler  Handler
}

func (c *connHandler) looper(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c.handler.OnClientConnected(ctx, c.clientID)
	defer c.handler.OnClientDisconnected(ctx, c.clientID)

	wg := sync.WaitGroup{}
	defer wg.Wait()

	reqChan := make(chan *requestBuffer)

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()
		_ = c.conn.Close()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		rb := new(requestBuffer)
		for {
			nextRb, err := nextRequestBuffer(ctx, bufSize, c.conn, rb)
			if err != nil {
				return
			}
			reqChan <- rb
			rb = nextRb
		}

	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		var (
			rb  *requestBuffer
			err error
		)

		for {
			select {
			case rb = <-reqChan:
			case <-ctx.Done():
				return
			}

			if rb == nil {
				continue
			}

			err = func() (err error) {
				defer rb.Close()

				var (
					isResponseWritten bool
				)

				defer func() {
					// handle newline on each write
					var n int
					n, err = c.conn.Write([]byte("\n"))
					if err != nil {
						return
					}
					if n != 1 {
						err = errors.New("inconsistent write")
					}
				}()

				defer func() {
					if err != nil {
						// return error response
						res := &response{}
						if errors.Is(err, ErrNoJob) {
							res.Status = "no-job"
						} else {
							res.Status = "error"
							res.Error = err.Error()
						}

						err = json.NewEncoder(c.conn).Encode(res)
						if err != nil {
							return
						}
					} else {
						if isResponseWritten {
							return
						}

						// print response ok
						err = json.NewEncoder(c.conn).Encode(&response{Status: "ok"})
						if err != nil {
							return
						}
					}
				}()

				req := &request{}
				err = json.NewDecoder(rb).Decode(req)
				if err != nil {
					return err
				}

				_, err = rb.Seek(0, io.SeekStart)
				if err != nil {
					return err
				}

				//ctx, cancelFunc := context.WithTimeout(ctx, 30*time.Second)
				//defer cancelFunc()

				switch req.Request {
				case "put":
					var (
						putReq = &putRequest{}
						putRes = &putResponse{}
						jobID  uint32
					)
					err = json.NewDecoder(rb).Decode(putReq)
					if err != nil {
						return err
					}
					jobID, err = c.handler.PutJob(ctx, putReq.Queue, putReq.Job, putReq.Pri)
					if err != nil {
						return err
					}
					putRes.Status = "ok"
					putRes.ID = jobID
					err = json.NewEncoder(c.conn).Encode(putRes)
					if err != nil {
						return err
					}
					isResponseWritten = true
				case "get":
					var (
						getReq   = &getRequest{}
						getRes   = &getResponse{}
						jobID    uint32
						job      any
						priority uint32
						queue    string
					)
					err = json.NewDecoder(rb).Decode(getReq)
					if err != nil {
						return err
					}
					jobID, job, priority, queue, err = c.handler.GetJob(ctx, getReq.Queues, getReq.Wait)
					if err != nil {
						return err
					}
					getRes.Status = "ok"
					getRes.ID = jobID
					getRes.Job = job
					getRes.Pri = priority
					getRes.Queue = queue
					err = json.NewEncoder(c.conn).Encode(getRes)
					if err != nil {
						return err
					}
					isResponseWritten = true
				case "delete":
					var (
						deleteReq = &deleteRequest{}
					)
					err = json.NewDecoder(rb).Decode(deleteReq)
					if err != nil {
						return err
					}
					err = c.handler.DeleteJob(ctx, deleteReq.ID)
					if err != nil {
						return err
					}
				case "abort":
					var (
						abortReq = &abortRequest{}
					)
					err = json.NewDecoder(rb).Decode(abortReq)
					if err != nil {
						return err
					}
					err = c.handler.AbortJob(ctx, abortReq.ID)
					if err != nil {
						return err
					}
				default:
					return errors.New("unrecognised request type")
				}
				return nil
			}()

			if err != nil {
				return
			}
		}
	}()
}

type (
	request struct {
		Request string `json:"request"`
	}
	response struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	putRequest struct {
		request
		Queue string `json:"queue"`
		Job   any    `json:"job"`
		Pri   uint32 `json:"pri"`
	}
	putResponse struct {
		response
		ID uint32 `json:"id"`
	}
	getRequest struct {
		request
		Queues []string `json:"queues"`
		Wait   bool     `json:"wait"`
	}
	getResponse struct {
		response
		ID    uint32 `json:"id,omitempty"`
		Job   any    `json:"job,omitempty"`
		Pri   uint32 `json:"pri,omitempty"`
		Queue string `json:"queue,omitempty"`
	}
	deleteRequest struct {
		request
		ID uint32 `json:"id"`
	}
	abortRequest struct {
		request
		ID uint32 `json:"id"`
	}
)

func nextRequestBuffer(ctx context.Context, bufSize int, reader io.Reader, curReqBuffer *requestBuffer) (nextReqBuffer *requestBuffer, err error) {
	var (
		buf = util.GetBytes(bufSize)
		n   int
	)
	defer util.PutBytes(buf)

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		n, err = reader.Read(buf)
		if err != nil {
			return
		}
		buf = buf[:n]

		if len(buf) == 0 {
			continue
		}

		newlineIdx := -1
		for i, b := range buf {
			if b == '\n' {
				newlineIdx = i
				break
			}
		}

		var (
			dataBeforeNewLine []byte
			dataAfterNewLine  []byte
		)
		if newlineIdx != -1 {
			dataBeforeNewLine = buf[:newlineIdx]
			dataAfterNewLine = buf[newlineIdx+1:]
		} else {
			dataBeforeNewLine = buf
		}

		if curReqBuffer.data == nil {
			curReqBuffer.data = append(curReqBuffer.data, util.GetBytes(bufSize)[:0])
		}
		for len(dataBeforeNewLine) > 0 {
			bufData := curReqBuffer.data[len(curReqBuffer.data)-1]
			if len(bufData) == cap(bufData) {
				bufData = util.GetBytes(bufSize)[:0]
				curReqBuffer.data = append(curReqBuffer.data, bufData)
			}
			n = copy(bufData[len(bufData):cap(bufData)], dataBeforeNewLine)
			bufData = bufData[:len(bufData)+n]
			curReqBuffer.data[len(curReqBuffer.data)-1] = bufData

			dataBeforeNewLine = dataBeforeNewLine[n:]
		}

		if len(dataAfterNewLine) > 0 {
			nextReqBuffer = new(requestBuffer)
			bufData := util.GetBytes(bufSize)
			nextReqBuffer.data = append(nextReqBuffer.data, bufData[:copy(bufData, dataAfterNewLine)])
			break
		}

		if newlineIdx != -1 {
			break
		}
	}
	if nextReqBuffer == nil {
		nextReqBuffer = new(requestBuffer)
	}
	return
}

type requestBuffer struct {
	closed bool
	data   [][]byte // must have same capacity
	pos    int
}

func (r *requestBuffer) Seek(offset int64, whence int) (n int64, err error) {
	if r.closed {
		return 0, errors.New("closed request buffer")
	}

	size := cap(r.data[0])
	maxSize := ((len(r.data) - 1) * size) + len(r.data[len(r.data)-1])

	switch whence {
	case io.SeekStart:
		if offset < 0 {
			return 0, nil
		}
		if offset >= int64(maxSize) {
			return 0, errors.New("out of range")
		}
		n = int64(r.pos)
		r.pos = 0
		return
	case io.SeekCurrent:
		return 0, errors.New("unimplemented yet")
	case io.SeekEnd:
		return 0, errors.New("unimplemented yet")
	default:
		return 0, errors.New("unknown whence")
	}
}

func (r *requestBuffer) Read(p []byte) (n int, err error) {
	if r.closed {
		return 0, errors.New("closed request buffer")
	}

	if len(r.data) <= 0 {
		return 0, io.EOF
	}

	size := cap(r.data[0])
	maxSize := ((len(r.data) - 1) * size) + len(r.data[len(r.data)-1])

	if r.pos >= maxSize {
		return 0, io.EOF
	}

	for n < len(p) && r.pos < maxSize {
		copied := copy(p[n:], r.data[r.pos/size][r.pos%size:])
		n += copied
		r.pos += copied
	}
	return
}

func (r *requestBuffer) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	for i, buf := range r.data {
		if buf == nil {
			continue
		}
		r.data[i] = nil
		util.PutBytes(buf)
	}
	r.data = nil
	return nil
}
