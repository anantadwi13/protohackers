package main

import (
	"container/heap"
	"context"
	"errors"
	"sync"
)

var (
	ErrJobQueueEmpty    = errors.New("job queue is empty")
	ErrJobQueueNotFound = errors.New("job is not found")
)

/*
Requirements
1. Push without blocking
2. Pop with blocking & non-blocking
3. Can remove the item from the queue
4. Contain multiple queues
5. map[string]Queue. where string => queue name and Queue => []Job. Job contain JobID, Job data, and Priority
*/

type Job struct {
	Queue    string
	JobID    uint32
	Job      any
	Priority uint32
}

type jobLocator struct {
	queue string
	job   *job
}

type JobQueues struct {
	jobQueues   map[string]*jobQueue
	jobLocators map[uint32]jobLocator // key JobID
	notifiers   []chan struct{}
	lock        sync.Mutex
	lockCond    *sync.Cond
}

func NewJobQueues() *JobQueues {
	jq := &JobQueues{
		jobQueues:   make(map[string]*jobQueue),
		jobLocators: make(map[uint32]jobLocator),
		lock:        sync.Mutex{},
	}
	jq.lockCond = sync.NewCond(&jq.lock)
	return jq
}

func (jq *JobQueues) Push(j Job) {
	jq.lock.Lock()
	defer jq.lock.Unlock()

	jj := &job{Job: j}

	queue, ok := jq.jobQueues[j.Queue]
	if !ok {
		queue = &jobQueue{}
		heap.Init(queue)
		jq.jobQueues[j.Queue] = queue
	}

	heap.Push(queue, jj)
	jq.jobLocators[jj.JobID] = jobLocator{queue: j.Queue, job: jj}

	for _, notifier := range jq.notifiers {
		close(notifier)
	}
	jq.notifiers = jq.notifiers[:0]
}

func (jq *JobQueues) Remove(jobID uint32) error {
	jq.lock.Lock()
	defer jq.lock.Unlock()

	jl, ok := jq.jobLocators[jobID]
	if !ok {
		return ErrJobQueueNotFound
	}

	queue := jq.jobQueues[jl.queue]
	heap.Remove(queue, jl.job.index)
	delete(jq.jobLocators, jobID)

	return nil
}

func (jq *JobQueues) Pop(ctx context.Context, queues []string, wait bool) (Job, error) {
	jq.lock.Lock()
	defer jq.lock.Unlock()

	validQueues := make(map[string]*jobQueue)

	for _, queueName := range queues {
		if _, ok := jq.jobQueues[queueName]; !ok {
			continue
		}
		validQueues[queueName] = jq.jobQueues[queueName]
	}

	for {
		if ctx.Err() != nil {
			return Job{}, ctx.Err()
		}

		var (
			highestQueue    *jobQueue
			highestPriority uint32
		)
		for _, queue := range validQueues {
			if queue.Len() == 0 {
				continue
			}
			priority := (*queue)[0].Priority
			if priority > highestPriority {
				highestQueue = queue
				highestPriority = priority
			}
		}
		if highestQueue != nil {
			jj := heap.Pop(highestQueue).(*job)
			delete(jq.jobLocators, jj.JobID)
			return jj.Job, nil
		}

		if !wait {
			break
		}

		notifier := make(chan struct{})
		jq.notifiers = append(jq.notifiers, notifier)

		jq.lock.Unlock()

		select {
		case <-notifier:
		case <-ctx.Done():
		}

		jq.lock.Lock()
	}

	return Job{}, ErrJobQueueEmpty
}

type job struct {
	Job

	index int // maintained by heap
}

type jobQueue []*job

func (jq *jobQueue) Len() int {
	return len(*jq)
}

func (jq *jobQueue) Less(i, j int) bool {
	return (*jq)[i].Priority > (*jq)[j].Priority
}

func (jq *jobQueue) Swap(i, j int) {
	(*jq)[i], (*jq)[j] = (*jq)[j], (*jq)[i]
	(*jq)[i].index = i
	(*jq)[j].index = j
}

func (jq *jobQueue) Push(x any) {
	n := len(*jq)
	item := x.(*job)
	item.index = n
	*jq = append(*jq, item)
}

func (jq *jobQueue) Pop() any {
	old := *jq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // don't stop the GC from reclaiming the item eventually
	item.index = -1 // for safety
	*jq = old[0 : n-1]
	return item
}
