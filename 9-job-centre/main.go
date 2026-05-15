package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"

	"github.com/anantadwi13/protohackers/9-job-centre/proto"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	qh := initQueueHandler(ctx)
	srv, err := proto.NewServer(":19090", qh)
	if err != nil {
		log.Fatalln(err)
	}
	go func() {
		log.Println("listening")
		err := srv.Listen()
		if err != nil {
			log.Fatalln(err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")
	err = srv.Shutdown(context.TODO())
	if err != nil {
		log.Fatalln(err)
	}
}

type queueHandler struct {
	jobID atomic.Uint32

	queues *JobQueues

	lock       sync.RWMutex
	clientJob  map[string]map[uint32]struct{} // key clientID, value set of jobID
	inProgress map[uint32]jobClient           // key jobID
}

func initQueueHandler(ctx context.Context) *queueHandler {
	qh := &queueHandler{
		queues:     NewJobQueues(),
		clientJob:  make(map[string]map[uint32]struct{}),
		inProgress: make(map[uint32]jobClient),
	}
	return qh
}

func (h *queueHandler) PutJob(ctx context.Context, queue string, job any, priority uint32) (jobID uint32, err error) {
	jobID = h.jobID.Add(1)

	log.Println("put job", queue, jobID, priority)

	j := Job{
		Queue:    queue,
		JobID:    jobID,
		Job:      job,
		Priority: priority,
	}

	h.queues.Push(j)
	return j.JobID, nil
}

func (h *queueHandler) GetJob(ctx context.Context, queues []string, wait bool) (jobID uint32, job any, priority uint32, queue string, err error) {
	log.Println("get job", queues, wait)

	clientID, err := proto.GetClientID(ctx)
	if err != nil {
		return 0, nil, 0, "", err
	}

	j, err := h.queues.Pop(ctx, queues, wait)
	if err != nil {
		if errors.Is(err, ErrJobQueueEmpty) {
			return 0, nil, 0, "", proto.ErrNoJob
		}
		return 0, nil, 0, "", err
	}

	h.lock.Lock()
	defer h.lock.Unlock()

	h.clientJob[clientID][j.JobID] = struct{}{}
	h.inProgress[j.JobID] = jobClient{
		Job:      j,
		ClientID: clientID,
	}

	return j.JobID, j.Job, j.Priority, j.Queue, nil
}

func (h *queueHandler) DeleteJob(ctx context.Context, jobID uint32) (err error) {
	log.Println("delete job", jobID)

	h.lock.Lock()
	defer h.lock.Unlock()

	found := false

	// delete job from queue
	err = h.queues.Remove(jobID)
	if err == nil {
		found = true
	}

	// delete job from inProgress
	if jc, isInProgress := h.inProgress[jobID]; isInProgress {
		found = true

		if _, ok := h.clientJob[jc.ClientID]; ok {
			delete(h.clientJob[jc.ClientID], jc.Job.JobID)
		}
		delete(h.inProgress, jobID)
	}

	if !found {
		return proto.ErrNoJob
	}

	return nil
}

func (h *queueHandler) AbortJob(ctx context.Context, jobID uint32) (err error) {
	log.Println("abort job", jobID)

	clientID, err := proto.GetClientID(ctx)
	if err != nil {
		return err
	}

	h.lock.Lock()
	defer h.lock.Unlock()

	jc, ok := h.inProgress[jobID]
	if !ok {
		return proto.ErrNoJob
	}

	_, ok = h.clientJob[clientID][jobID]
	if !ok {
		return errors.Join(proto.ErrUnknown, fmt.Errorf("you are not allowed to abort job %d", jobID))
	}

	delete(h.clientJob[clientID], jobID)
	delete(h.inProgress, jobID)
	h.queues.Push(jc.Job)

	return nil
}

func (h *queueHandler) OnClientConnected(ctx context.Context, clientID string) {
	log.Println("client connected", clientID)

	h.lock.RLock()
	client, ok := h.clientJob[clientID]
	h.lock.RUnlock()

	if ok && client != nil {
		return
	}

	h.lock.Lock()
	defer h.lock.Unlock()
	client, ok = h.clientJob[clientID]
	if !ok || client == nil {
		h.clientJob[clientID] = make(map[uint32]struct{})
	}
}

func (h *queueHandler) OnClientDisconnected(ctx context.Context, clientID string) {
	log.Println("client disconnected", clientID)

	h.lock.RLock()
	client, ok := h.clientJob[clientID]
	h.lock.RUnlock()

	if !ok || client == nil {
		return
	}
	h.lock.Lock()
	defer h.lock.Unlock()

	defer delete(h.clientJob, clientID)

	// abort job
	for jobID := range h.clientJob[clientID] {
		j, isInProgress := h.inProgress[jobID]
		if !isInProgress {
			continue
		}

		h.queues.Push(j.Job)

		delete(h.inProgress, jobID)
	}
}

type jobClient struct {
	Job      Job
	ClientID string
}
