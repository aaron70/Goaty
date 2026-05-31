package concurrency

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aaron70/goaty/channels"
	"github.com/aaron70/goaty/errors"
	"github.com/google/uuid"
)

type eventType string

const (
	taskEnqueued    = "TASK_ENQUEUED"
	taskConsumed    = "TASK_CONSUMED"
	taskDone        = "TASK_DONE"
	taskFailed      = "TASK_FAILED"
	taskQueueClosed = "TASK_QUEUE_CLOSED"
)

type event struct {
	Type eventType
}

type TasksMetrics struct {
	Enqueued int64
	Consumed int64
	Done     int64
	Failed   int64
}

type WorkersMetrics struct {
	Alive int64
}

type Metrics struct {
	Tasks   TasksMetrics
	Workers WorkersMetrics
}

type WorkerFuncResult[T, R any] func(ctx context.Context, task T) (R, error)

type PoolResult[T, R any] struct {
	MaxWorkers   int
	BufferSize   int
	IdleDuration time.Duration

	ctx  context.Context
	work WorkerFuncResult[T, R]

	monitorDone chan struct{}
	events      chan event
	queueDone   chan struct{}
	queue       chan T
	results     chan R
	errors      chan error

	tasksEnqueued atomic.Int64
	tasksConsumed atomic.Int64
	tasksDone     atomic.Int64
	tasksFailed   atomic.Int64
	workersAlive  atomic.Int64
	workersIdle   atomic.Int64

	workers   sync.WaitGroup
	closeOnce sync.Once
}

func NewPoolResult[T, R any](ctx context.Context, work WorkerFuncResult[T, R]) (*PoolResult[T, R], error) {
	if ctx == nil {
		return nil, errors.NewError(errors.ErrInvalidArgument, errors.ErrNilReference, "Context should not be nil, please use content.Background() or similar if no need a context.")
	}

	if work == nil {
		return nil, errors.NewError(errors.ErrInvalidArgument, errors.ErrNilReference, "The work function can't be nil, the work function provides the ability to workers to process the tasks.")
	}

	maxWorkers := 20
	bufferSize := 20
	idleDuration := time.Second

	pool := &PoolResult[T, R]{
		MaxWorkers:   maxWorkers,
		BufferSize:   bufferSize,
		IdleDuration: idleDuration,

		ctx:  ctx,
		work: work,

		queueDone:   make(chan struct{}, 1),
		queue:       make(chan T, bufferSize),
		results:     make(chan R, bufferSize),
		errors:      make(chan error, bufferSize),
		monitorDone: make(chan struct{}, 1),
		events:      make(chan event, max(bufferSize+maxWorkers, 100)),
	}

	go pool.monitor()
	return pool, nil
}

func (p *PoolResult[T, R]) Close() {
	p.closeOnce.Do(func() {
		close(p.queue)
		close(p.queueDone)
		channels.Send(p.ctx, p.events, event{taskQueueClosed})
	})
}

func (p *PoolResult[T, R]) Wait() error {
	for {
		_, open, err := channels.Recv(p.ctx, p.queueDone)
		if err != nil {
			return err
		}
		if !open {
			break
		}
	}

	for {
		_, open, err := channels.Recv(p.ctx, p.monitorDone)
		if err != nil {
			return err
		}
		if !open {
			break
		}
	}

	p.workers.Wait()
	return nil
}

func (p *PoolResult[T, R]) Metrics() Metrics {
	return Metrics{
		Tasks: TasksMetrics{
			Enqueued: p.tasksEnqueued.Load(),
			Consumed: p.tasksConsumed.Load(),
			Done:     p.tasksDone.Load(),
			Failed:   p.tasksFailed.Load(),
		},
		Workers: WorkersMetrics{
			Alive: p.workersAlive.Load(),
		},
	}
}

func (p *PoolResult[T, R]) isQueueClosed() bool {
	select {
	case <-p.queueDone:
		return true
	default:
		return false
	}
}

func (p *PoolResult[T, R]) IsDone() bool {
	return p.isQueueClosed() && p.tasksEnqueued.Load() <= 0 && p.tasksConsumed.Load() <= 0 && p.workersAlive.Load() <= 0
}

func (p *PoolResult[T, R]) monitor() {
	// fmt.Println("Monitor has started...")
	// defer fmt.Println("Monitor has exited...")
	defer close(p.monitorDone)

lifecycle:
	for {
		event, open, err := channels.Recv(p.ctx, p.events)
		if err != nil {
			return
		}
		if !open {
			break
		}

		switch event.Type {
		case taskEnqueued:
			p.tasksEnqueued.Add(1)
			p.submitWorker(p.work)
		case taskConsumed:
			p.tasksEnqueued.Add(-1)
			p.tasksConsumed.Add(1)
		case taskDone:
			p.tasksConsumed.Add(-1)
			p.tasksDone.Add(1)
			if p.IsDone() {
				break lifecycle
			}
		case taskFailed:
			p.tasksConsumed.Add(-1)
			p.tasksFailed.Add(1)
			if p.IsDone() {
				break lifecycle
			}
		case taskQueueClosed:
			if p.IsDone() {
				break lifecycle
			}
		}
	}
}

func (p *PoolResult[T, R]) RecvTasks(tasks <-chan T) error {
	if tasks == nil {
		return errors.NewError(errors.ErrInvalidArgument, errors.ErrNilReference, "The given channel is nil.")
	}

	for {
		task, open, err := channels.Recv(p.ctx, tasks)
		if err != nil {
			return errors.NewError(nil, err, "Could not read the task")
		}
		if !open {
			break
		}

		channels.Send(p.ctx, p.events, event{taskEnqueued})
		err = channels.Send(p.ctx, p.queue, task)
		if err != nil {
			return errors.NewError(nil, err, "Could not enqueue the task")
		}
	}

	return nil
}

func (p *PoolResult[T, R]) tryCreateGoroutine(worker func(ctx context.Context, id string) func()) bool {
	for {
		aliveWorkers := p.workersAlive.Load()
		if aliveWorkers >= int64(p.MaxWorkers) {
			return false
		}
		if p.workersAlive.CompareAndSwap(aliveWorkers, aliveWorkers+1) {
			id := uuid.NewString()
			p.workers.Go(worker(p.ctx, id))
			return true
		}
	}
}

func (p *PoolResult[T, R]) submitWorker(work WorkerFuncResult[T, R]) bool {
	return p.tryCreateGoroutine(func(ctx context.Context, id string) func() {
		return func() {
			// fmt.Printf("Worker %s stared...\n", id)
			// defer fmt.Printf("Worker %s finished...\n", id)
			defer p.workersAlive.Add(-1)
			defer func() {
				if r := recover(); r != nil {
					channels.Send(p.ctx, p.events, event{taskFailed})
					channels.Send(p.ctx, p.errors, errors.NewError(errors.ErrPanicRecovered, nil, "Worker %s has panic: %v", id, r))
				}
			}()
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
						// fmt.Printf("[Worker %s]: Queue is closed!\n", id)
						return
					}
					channels.Send(p.ctx, p.events, event{taskConsumed})
					res, err := work(p.ctx, task)
					if err != nil {
						channels.Send(p.ctx, p.events, event{taskFailed})
						channels.Send(ctx, p.errors, err)
					} else {
						err := channels.Send(ctx, p.results, res)
						if err != nil {
							channels.Send(p.ctx, p.events, event{taskFailed})
							channels.Send(ctx, p.errors, err)
						}
						channels.Send(p.ctx, p.events, event{taskDone})
					}
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
