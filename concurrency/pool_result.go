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
	taskEnqueued       = "TASK_ENQUEUED"
	taskConsumed       = "TASK_CONSUMED"
	taskDone           = "TASK_DONE"
	taskFailed         = "TASK_FAILED"
	taskQueueClosed    = "TASK_QUEUE_CLOSED"
	workerStateChanged = "WORKER_STATE_CHANGED"
)

type workerState string

const (
	prevState = "prev_state"
	currState = "curr_state"
)

const (
	workerCreatedState workerState = "CREATED"
	workerRunningState workerState = "RUNNING"
	workerIdleState    workerState = "IDLE"
	workerDoneState    workerState = "DONE"
	workerFailedState  workerState = "FAILED"
)

type event struct {
	Type eventType
	Data map[string]any
}

func newEventWithData(eventType eventType, data map[string]any) event {
	return event{
		Type: eventType,
		Data: data,
	}
}

func newEvent(eventType eventType) event {
	return newEventWithData(eventType, make(map[string]any))
}

type TasksMetrics struct {
	Enqueued int64
	Consumed int64
	Done     int64
	Failed   int64
}

type WorkersMetrics struct {
	Alive   int64
	Created int64
	Running int64
	Idle    int64
	Done    int64
	Failed  int64
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
	KeepAlive    bool

	ctx  context.Context
	work WorkerFuncResult[T, R]

	monitorDone chan struct{}
	events      chan event
	queueDone   chan struct{}
	queue       chan T
	results     chan R
	errors      chan error

	tasksEnqueued  atomic.Int64
	tasksConsumed  atomic.Int64
	tasksDone      atomic.Int64
	tasksFailed    atomic.Int64
	workersAlive   atomic.Int64
	workersCreated atomic.Int64
	workersRunning atomic.Int64
	workersIdle    atomic.Int64
	workersDone    atomic.Int64
	workersFailed  atomic.Int64

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

	maxWorkers := 3 
	bufferSize := 1000
	idleDuration := time.Second
	keepAlive := true

	pool := &PoolResult[T, R]{
		MaxWorkers:   maxWorkers,
		BufferSize:   bufferSize,
		IdleDuration: idleDuration,
		KeepAlive:    keepAlive,

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
		channels.Send(p.ctx, p.events, newEvent(taskQueueClosed))
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
			Alive:   p.workersAlive.Load(),
			Created: p.workersCreated.Load(),
			Running: p.workersRunning.Load(),
			Idle:    p.workersIdle.Load(),
			Done:    p.workersDone.Load(),
			Failed:  p.workersFailed.Load(),
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
	defer close(p.results)
	defer close(p.errors)

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
			if p.workersIdle.Load() <= 0 {
				p.submitWorker(p.work)
			}
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
		case workerStateChanged:
			prev := event.Data[prevState].(workerState)
			curr := event.Data[currState].(workerState)
			switch prev {
			case workerRunningState:
				p.workersRunning.Add(-1)
			case workerIdleState:
				p.workersIdle.Add(-1)
			}
			switch curr {
			case workerRunningState:
				p.workersRunning.Add(1)
			case workerIdleState:
				p.workersIdle.Add(1)
			case workerDoneState:
				p.workersDone.Add(1)
				p.workersAlive.Add(-1)
			case workerFailedState:
				p.workersIdle.Add(1)
				p.workersAlive.Add(-1)
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

		channels.Send(p.ctx, p.events, newEvent(taskEnqueued))
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
			p.workersCreated.Add(1)
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
			// defer channels.Send(p.ctx, p.events, newEventWithData(workerStateChanged, map[string]any{ prevState: }))
			defer func() {
				if r := recover(); r != nil {
					channels.Send(p.ctx, p.events, newEventWithData(workerStateChanged, map[string]any{prevState: workerRunningState, currState: workerFailedState}))
					channels.Send(p.ctx, p.events, newEvent(taskFailed))
					channels.Send(p.ctx, p.errors, errors.NewError(errors.ErrPanicRecovered, nil, "Worker %s has panic: %v", id, r))
				}
			}()

			channels.Send(p.ctx, p.events, newEventWithData(workerStateChanged, map[string]any{prevState: workerCreatedState, currState: workerIdleState}))

			timer := time.NewTimer(p.IdleDuration)
			if p.KeepAlive {
				timer.C = nil // Will block for ever the <-time.C case
			}
			done := true
		lifecycle:
			for {
				select {
				case <-ctx.Done():
					err := ctx.Err()
					if err != nil {
						_ = channels.Send(p.ctx, p.errors, err)
					}
					channels.Send(p.ctx, p.events, newEventWithData(workerStateChanged, map[string]any{prevState: workerIdleState, currState: workerDoneState}))
					break lifecycle

				case task, open := <-p.queue:
					done = false
					if !open {
						// fmt.Printf("[Worker %s]: Queue is closed!\n", id)
						channels.Send(p.ctx, p.events, newEventWithData(workerStateChanged, map[string]any{prevState: workerIdleState, currState: workerDoneState}))
						break lifecycle
					}
					channels.Send(p.ctx, p.events, newEventWithData(workerStateChanged, map[string]any{prevState: workerIdleState, currState: workerRunningState}))
					channels.Send(p.ctx, p.events, newEvent(taskConsumed))
					res, err := work(p.ctx, task)
					if err != nil {
						channels.Send(p.ctx, p.events, newEvent(taskFailed))
						channels.Send(ctx, p.errors, err)
					} else {
						err := channels.Send(ctx, p.results, res)
						if err != nil {
							channels.Send(p.ctx, p.events, newEvent(taskFailed))
							channels.Send(ctx, p.errors, err)
						}
						channels.Send(p.ctx, p.events, newEvent(taskDone))
					}
					done = true
					timer.Reset(p.IdleDuration)
					channels.Send(p.ctx, p.events, newEventWithData(workerStateChanged, map[string]any{prevState: workerRunningState, currState: workerIdleState}))
				case <-timer.C:
					if !done || p.KeepAlive {
						continue
					}
					timer.Stop()
					channels.Send(p.ctx, p.events, newEventWithData(workerStateChanged, map[string]any{prevState: workerIdleState, currState: workerDoneState}))
					break lifecycle
				}
			}
		}
	})
}

func (p *PoolResult[T, R]) ResultsErr() (<-chan R, <-chan error) {
	return p.results, p.errors
}
