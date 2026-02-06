package jobqueue

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type JobQueue struct {
	queue    chan string
	executed int64
}

func New() *JobQueue {
	return &JobQueue{queue: make(chan string, 100)}
}

func (s *JobQueue) Enqueue(url string) {
	s.queue <- url
}

func (s *JobQueue) Waiting() int {
	return len(s.queue)
}

func (s *JobQueue) Executed() int64 {
	return atomic.LoadInt64(&s.executed)
}

func (s *JobQueue) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for url := range s.queue {
			fmt.Printf("Performing job: %s\n", url)
			atomic.AddInt64(&s.executed, 1)
			time.Sleep(time.Second)
		}
		wg.Done()
	}()

	<-ctx.Done()
	fmt.Printf("Draining job queue\n")
	close(s.queue)
	wg.Wait()

	return ctx.Err()
}
