package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type StupidJobQueue struct {
	queue    chan string
	executed int64
}

func NewStupidJobQueue() *StupidJobQueue {
	return &StupidJobQueue{queue: make(chan string, 100)}
}

func (s *StupidJobQueue) Enqueue(url string) {
	s.queue <- url
}

func (s *StupidJobQueue) Waiting() int {
	return len(s.queue)
}

func (s *StupidJobQueue) Executed() int64 {
	return atomic.LoadInt64(&s.executed)
}

func (s *StupidJobQueue) Run(ctx context.Context) error {
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
