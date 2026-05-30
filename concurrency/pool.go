package concurrency

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aaron70/goaty/channels"
	"github.com/google/uuid"
)

type WorkerFunc[T any] func(context.Context, T)

type Pool[T, R any] struct {
	MaxWorkers   int
	BufferSize   int
	IdleDuration time.Duration

	queue     chan T
	results   chan R
	errors    chan error
	queueDone chan struct{}

	ctx          context.Context
	workers      sync.WaitGroup
	aliveWorkers atomic.Int32
}

type newPoolOption[T, R any] func(*Pool[T, R]) error

func NewPool[T, R any](ctx context.Context, options ...newPoolOption[T, R]) (*Pool[T, R], error) {
	pool := &Pool[T, R]{
		MaxWorkers:   25,
		BufferSize:   0,
		IdleDuration: time.Second * 3,

		ctx: ctx,
	}

	for _, option := range options {
		if err := option(pool); err != nil {
			return nil, err
		}
	}

	pool.queue = make(chan T, pool.BufferSize)
	pool.results = make(chan R, pool.BufferSize)
	pool.errors = make(chan error, pool.BufferSize)

	return pool, nil
}

func NewPoolWithMaxWorkers[T, R any](maxWorkers int) newPoolOption[T, R] {
	return func(p *Pool[T, R]) error {
		p.MaxWorkers = maxWorkers
		return nil
	}
}

func NewPoolWithIdleDuration[T, R any](idleDuration time.Duration) newPoolOption[T, R] {
	return func(p *Pool[T, R]) error {
		p.IdleDuration = idleDuration
		return nil
	}
}

func NewPoolWithBufferSize[T, R any](bufferSize int) newPoolOption[T, R] {
	return func(p *Pool[T, R]) error {
		p.BufferSize = bufferSize
		return nil
	}
}

func (p *Pool[T, R]) tryCreateGoroutine(worker func(ctx context.Context, id string) func()) bool {
	for {
		aliveWorkers := p.aliveWorkers.Load()
		if aliveWorkers >= int32(p.MaxWorkers) {
			return false
		}
		if p.aliveWorkers.CompareAndSwap(aliveWorkers, aliveWorkers+1) {
			id := uuid.NewString()
			p.workers.Go(worker(p.ctx, id))
			return true
		}
	}
}

func (p *Pool[T, R]) submitWorker(worker WorkerFunc[T]) bool {
	return p.tryCreateGoroutine(func(ctx context.Context, id string) func() {
		return func() {
			defer p.aliveWorkers.Add(-1)
			timer := time.NewTimer(p.IdleDuration)
			done := true
			for {
				select {
				case <-ctx.Done():
					err := ctx.Err()
					if err != nil {
						_ = channels.Send(p.ctx, p.errors, err)
					}
					return

				case task, open := <-p.queue:
					done = false
					if !open {
						return
					}
					worker(p.ctx, task)
					done = true
					timer.Reset(p.IdleDuration)
				case <-timer.C:
					if !done {
						continue
					}
					timer.Stop()
					return
				}
			}
		}
	})
}

func (p *Pool[T, R]) PushTasks(tasks <-chan T, worker WorkerFunc[T]) error {
	p.queueDone = make(chan struct{}, 1)
	defer func() {
		close(p.queue)
		close(p.queueDone)
	}()
	for {
		task, open, err := channels.Recv(p.ctx, tasks)
		if err != nil {
			return err
		}
		if !open {
			return nil
		}

		p.submitWorker(worker)
		err = channels.Send(p.ctx, p.queue, task)
		if err != nil {
			return err
		}
	}
}

func (p *Pool[T, R]) Wait() {
	for range p.queueDone {}
	p.workers.Wait()
}
