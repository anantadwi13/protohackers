package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/anantadwi13/protohackers/9-job-centre/proto"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	h := &handler{}
	srv, err := proto.NewServer(":19090", h)
	if err != nil {
		log.Fatalln(err)
	}
	go func() {
		log.Println("listening")
		err = srv.Listen()
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

type handler struct {
}

func (h *handler) PutJob(ctx context.Context, queue string, job any, priority int) (jobID int, err error) {
	log.Println("put job")

	// todo
	// - create a new unique jobID for each put request
	return 0, nil
}

func (h *handler) GetJob(ctx context.Context, queues []string, wait bool) (jobID int, job any, priority int, queue string, err error) {
	log.Println("get job")
	return
}

func (h *handler) DeleteJob(ctx context.Context, id int) (err error) {
	log.Println("delete job")

	// todo
	// - deleted job makes the job can't be retrieved, aborted, or deleted again
	// - can be called from any client
	// - send no-job for unregistered job or deleted job, otherwise ok
	return
}

func (h *handler) AbortJob(ctx context.Context, id int) (err error) {
	log.Println("abort job")

	// todo
	// - check client valid for job id, otherwise return error
	// - send no-job for unassigned job, deleted job, or unregistered job
	// - automatically abort when client disconnect
	return
}
