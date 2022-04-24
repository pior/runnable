package di

import (
	"context"
	"sync/atomic"
)

type DB struct {
	closed int32
}

func (s *DB) Read() {
	if atomic.LoadInt32(&s.closed) == 1 {
		panic("db is closed !")
	}
}

func (s *DB) Run(ctx context.Context) error {
	<-ctx.Done()
	atomic.StoreInt32(&s.closed, 1)
	return ctx.Err()
}

// func (s *DB) Close() error {
// 	atomic.StoreInt32(&s.closed, 1)
// 	return nil
// }
