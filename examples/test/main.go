package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pior/runnable"
)

func log(runner runnable.Runnable, format string, a ...interface{}) {
	fmt.Println(fmt.Sprintf("%T: ", runner) + fmt.Sprintf(format, a...))
}

type ServerNoShutdown struct{}

func (s *ServerNoShutdown) Run(ctx context.Context) error {
	log(s, "start")
	select {} // blocking
}

type ServerPanic struct{}

func (s *ServerPanic) Run(ctx context.Context) error {
	log(s, "start")

	time.Sleep(time.Second * 1)
	panic("yooooolooooooo")
}

type Server struct {
	deadline time.Duration
}

func (s *Server) Run(ctx context.Context) error {
	log(s, "start")

	if s.deadline.Seconds() == 0 {
		s.deadline = time.Second * 10000000
	}

	theEnd := time.After(s.deadline)

	select {
	case <-ctx.Done():
	case <-theEnd:
		log(s, "sepuku")
		return errors.New("sepuku")
	}

	log(s, "stop")
	return nil
}

type OneOff struct {
}

func (s *OneOff) Run(ctx context.Context) error {
	log(s, "run")
	return nil
}

type ServerWithDB struct {
	db *DB
}

func (s *ServerWithDB) Run(ctx context.Context) error {
	log(s, "start")

	ticker := time.NewTicker(time.Millisecond * 1)

	for {
		select {
		case <-ctx.Done():
			log(s, "stop")
			return nil
		case <-ticker.C:
			s.db.Read()
		}
	}
}

type Metrics struct {
	running bool
}

func (m *Metrics) Publish() {
	if !m.running {
		panic("Metrics is closed !")
	}
}

func (m *Metrics) Run(ctx context.Context) error {
	log(m, "start")
	m.running = true

	<-ctx.Done()
	log(m, "stopping")
	time.Sleep(time.Millisecond * 300)

	m.running = false
	log(m, "stop")
	return nil
}

type DB struct {
	running bool
	metrics *Metrics
}

func (s *DB) Read() {
	if !s.running {
		panic("db is closed !")
	}
	s.metrics.Publish()
}

func (s *DB) Run(ctx context.Context) error {
	log(s, "start")
	s.running = true

	ticker := time.NewTicker(time.Millisecond * 1)

	for {
		select {
		case <-ctx.Done():
			log(s, "stopping")
			time.Sleep(time.Millisecond * 300)

			s.running = false
			log(s, "stop")
			return nil
		case <-ticker.C:
			s.metrics.Publish()
		}
	}
}

func main() {
	_ = &ServerNoShutdown{}
	_ = &OneOff{}
	_ = &ServerPanic{}
	_ = &Server{}

	metrics := &Metrics{}
	db := &DB{metrics: metrics}
	server := &ServerWithDB{db}

	g := runnable.Manager()
	g.Add(metrics)
	g.Add(db, metrics)
	g.Add(server, db)

	runnable.Run(g.Build())
}
