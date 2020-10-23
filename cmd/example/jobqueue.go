package main

import (
	"context"
	"fmt"
	"sync/atomic"
)

type StupidJobQueue struct {
	queue    chan string
	executed int64
}

func NewStupidJobQueue() *StupidJobQueue {
	return &StupidJobQueue{queue: make(chan string)}
}

func (s *StupidJobQueue) Perform(url string) {
	s.queue <- url
}

func (s *StupidJobQueue) Executed() int64 {
	return atomic.LoadInt64(&s.executed)
}

func (s *StupidJobQueue) Run(ctx context.Context) error {
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
