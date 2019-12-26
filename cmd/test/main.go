package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pior/runnable"
)

func printf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

type InitCleanup struct {
	initTime    time.Duration
	cleanupTime time.Duration
}

func (s *InitCleanup) Init(ctx context.Context) error {
	time.Sleep(s.initTime)
	return nil
}

func (s *InitCleanup) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (s *InitCleanup) Cleanup(ctx context.Context) error {
	time.Sleep(s.cleanupTime)
	return nil
}

var _ runnable.RunnableInit = &InitCleanup{}
var _ runnable.RunnableCleanup = &InitCleanup{}

type ServerNoShutdown struct{}

func (s *ServerNoShutdown) Run(ctx context.Context) error {
	printf("%T: start\n", s)
	<-make(chan struct{})

	printf("%T: stop\n", s)
	return nil
}

type ServerPanic struct{}

func (s *ServerPanic) Run(ctx context.Context) error {
	printf("%T: start\n", s)

	time.Sleep(time.Second * 1)
	panic("yooooolooooooo")
}

type Server struct {
	deadline time.Duration
}

func (s *Server) Run(ctx context.Context) error {
	printf("%T: start\n", s)

	if s.deadline.Seconds() == 0 {
		s.deadline = time.Second * 10000000
	}

	theEnd := time.After(s.deadline)

	select {
	case <-ctx.Done():
	case <-theEnd:
		printf("%T: sepuku\n", s)
		return errors.New("sepuku")
	}

	printf("%T: stop\n", s)
	return nil
}

type OneOff struct {
}

func (s *OneOff) Run(ctx context.Context) error {
	printf("%T: run\n", s)
	return nil
}

func main() {
	runnables := []runnable.Runnable{
		&InitCleanup{time.Second, time.Second},
		&InitCleanup{time.Second * 2, time.Second * 2},

		&Server{deadline: time.Millisecond * 2500},
		// &ServerNoShutdown{},
		// &ServerPanic{},

		runnable.Periodic(runnable.PeriodicOptions{Period: time.Second}, &OneOff{}),
	}

	runnable.RunGroup(runnables...)
}
