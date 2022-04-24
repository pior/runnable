package di

import (
	"context"
	"fmt"
	"sync/atomic"
)

type JobQueue struct {
	queue    chan string
	db       *DB
	executed int64
}

func NewJobQueue(db *DB) *JobQueue {
	return &JobQueue{queue: make(chan string), db: db}
}

func (s *JobQueue) Perform(url string) {
	s.queue <- url
}

func (s *JobQueue) Executed() int64 {
	return atomic.LoadInt64(&s.executed)
}

func (s *JobQueue) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case url := <-s.queue:
			fmt.Printf("Job from %s\n", url)
			atomic.AddInt64(&s.executed, 1)
		}
	}
}
