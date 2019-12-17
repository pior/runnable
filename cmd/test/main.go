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
		// &Server{deadline: time.Millisecond * 1500},
		// &Server{deadline: time.Millisecond * 2000},
		// &Server{deadline: time.Millisecond * 2500},
		// &ServerNoShutdown{},
		// &ServerPanic{},
		runnable.Periodic(runnable.PeriodicOptions{Period: time.Second}, &OneOff{}),
	}

	runnable.RunGroup(runnables...)
}
