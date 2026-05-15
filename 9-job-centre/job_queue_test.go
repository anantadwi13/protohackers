package main

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJobQueues(t *testing.T) {
	ctx := t.Context()

	jq := NewJobQueues()

	j, err := jq.Pop(ctx, []string{"q1"}, false)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrJobQueueInvalid)
	assert.Equal(t, Job{}, j)

	jq.Push(Job{Queue: "q1", JobID: 2, Job: "job2", Priority: 2})
	jq.Push(Job{Queue: "q1", JobID: 1, Job: "job1", Priority: 1})
	jq.Push(Job{Queue: "q1", JobID: 3, Job: "job3", Priority: 3})

	j, err = jq.Pop(ctx, []string{"q1"}, false)
	assert.NoError(t, err)
	assert.Equal(t, Job{Queue: "q1", JobID: 3, Job: "job3", Priority: 3}, j)

	j, err = jq.Pop(ctx, []string{"q1"}, false)
	assert.NoError(t, err)
	assert.Equal(t, Job{Queue: "q1", JobID: 2, Job: "job2", Priority: 2}, j)

	j, err = jq.Pop(ctx, []string{"q1"}, false)
	assert.NoError(t, err)
	assert.Equal(t, Job{Queue: "q1", JobID: 1, Job: "job1", Priority: 1}, j)

	j, err = jq.Pop(ctx, []string{"q1"}, false)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrJobQueueEmpty)
	assert.Equal(t, Job{}, j)
}

func jobQueues_concurrent(tb testing.TB) {
	ctx := tb.Context()
	jq := NewJobQueues()

	j, err := jq.Pop(ctx, []string{"q1"}, false)
	assert.Error(tb, err)
	assert.ErrorIs(tb, err, ErrJobQueueInvalid)
	assert.Equal(tb, Job{}, j)

	start := time.Now()

	go func() {
		jq.Push(Job{Queue: "q1", JobID: 2, Job: "job2", Priority: 2})
		jq.Push(Job{Queue: "q1", JobID: 4, Job: "job4", Priority: 4})
		time.Sleep(250 * time.Millisecond)
		jq.Push(Job{Queue: "q1", JobID: 1, Job: "job1", Priority: 1})
		time.Sleep(500 * time.Millisecond)
		jq.Push(Job{Queue: "q1", JobID: 3, Job: "job3", Priority: 3})
	}()

	go func() {
		jq.Push(Job{Queue: "q2", JobID: 5, Job: "job5", Priority: 5})
		time.Sleep(750 * time.Millisecond)
		jq.Push(Job{Queue: "q2", JobID: 6, Job: "job6", Priority: 6})
	}()

	time.Sleep(100 * time.Millisecond)

	j, err = jq.Pop(ctx, []string{"q1"}, true)
	assert.NoError(tb, err)
	assert.Equal(tb, Job{Queue: "q1", JobID: 4, Job: "job4", Priority: 4}, j)

	j, err = jq.Pop(ctx, []string{"q1"}, true)
	assert.NoError(tb, err)
	assert.Equal(tb, Job{Queue: "q1", JobID: 2, Job: "job2", Priority: 2}, j)

	j, err = jq.Pop(ctx, []string{"q2"}, true)
	assert.NoError(tb, err)
	assert.Equal(tb, Job{Queue: "q2", JobID: 5, Job: "job5", Priority: 5}, j)

	assert.Less(tb, time.Since(start), 250*time.Millisecond)

	j, err = jq.Pop(ctx, []string{"q1"}, true)
	assert.NoError(tb, err)
	assert.Equal(tb, Job{Queue: "q1", JobID: 1, Job: "job1", Priority: 1}, j)

	assert.Greater(tb, time.Since(start), 250*time.Millisecond)
	assert.Less(tb, time.Since(start), 750*time.Millisecond)

	j, err = jq.Pop(ctx, []string{"q1", "q2"}, true)
	assert.NoError(tb, err)
	assert.True(tb, (j.Queue == "q1" && j.JobID == 3) || (j.Queue == "q2" && j.JobID == 6))
	j, err = jq.Pop(ctx, []string{"q1", "q2"}, true)
	assert.NoError(tb, err)
	assert.True(tb, (j.Queue == "q1" && j.JobID == 3) || (j.Queue == "q2" && j.JobID == 6))

	assert.Greater(tb, time.Since(start), 750*time.Millisecond)

	j, err = jq.Pop(ctx, []string{"q1"}, false)
	assert.Error(tb, err)
	assert.ErrorIs(tb, err, ErrJobQueueEmpty)
	assert.Equal(tb, Job{}, j)
}

func TestJobQueues_concurrent(t *testing.T) {
	jobQueues_concurrent(t)
}

func TestJobQueues_concurrent_multiple(t *testing.T) {
	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Go(func() {
			jobQueues_concurrent(t)
		})
	}
	wg.Wait()
	wg = sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Go(func() {
			jobQueues_concurrent(t)
		})
	}
	wg.Wait()
}
