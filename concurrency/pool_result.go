package concurrency

import "context"

type WorkerFuncResult[T, R any] func(ctx context.Context, task T) (R, error)

type PoolResult[T, R any] struct {
}

func NewPoolResult[T, R any](ctx context.Context, worker WorkerFuncResult[T, R]) *PoolResult[T, R] {
	pool := &PoolResult[T, R]{
	}
	return pool
}
