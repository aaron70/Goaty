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
	eventsHandled  atomic.Int64

	workers   sync.WaitGroup
	closeOnce sync.Once
}

type poolResultOption func(*poolResultConfig) error

type poolResultConfig struct {
	MaxWorkers   int
	BufferSize   int
	IdleDuration time.Duration
	KeepAlive    bool
}

func NewPoolResultWithMaxWorkers(maxWorkers int) poolResultOption {
	return func(prc *poolResultConfig) error {
		if maxWorkers <= 0 {
			return errors.NewError(errors.ErrInvalidArgument, nil, "maxWorkers must be a number greater than 0")
		}
		prc.MaxWorkers = maxWorkers
		return nil
	}
}

func NewPoolResultWithBufferSize(bufferSize int) poolResultOption {
	return func(prc *poolResultConfig) error {
		if bufferSize < 0 {
			return errors.NewError(errors.ErrInvalidArgument, nil, "bufferSize must be 0 or a positive number")
		}
		prc.BufferSize = bufferSize
		return nil
	}
}

// The idle duration should be greater than the duration that takes each record to be produced, to avoid creating a worker for each record.
func NewPoolResultWithIdleDuration(idleDuration time.Duration) poolResultOption {
	return func(prc *poolResultConfig) error {
		minIdleDuration := time.Millisecond
		if idleDuration < minIdleDuration {
			return errors.NewError(errors.ErrInvalidArgument, nil, "idleDuration must be greater than %s. Although %s is the minimum duration is not recommended to use too low values as the idle timer could trigger before the job could read the task, leaving the task in the queue forever.", minIdleDuration, minIdleDuration)
		}
		prc.IdleDuration = idleDuration
		return nil
	}
}

func NewPoolResultWithKeepAlive(KeepAlive bool) poolResultOption {
	return func(prc *poolResultConfig) error {
		prc.KeepAlive = KeepAlive
		return nil
	}
}

func NewPoolResult[T, R any](ctx context.Context, work WorkerFuncResult[T, R], options ...poolResultOption) (*PoolResult[T, R], error) {
	if ctx == nil {
		return nil, errors.NewError(errors.ErrInvalidArgument, errors.ErrNilReference, "Context should not be nil, please use content.Background() or similar if no need a context.")
	}

	if work == nil {
		return nil, errors.NewError(errors.ErrInvalidArgument, errors.ErrNilReference, "The work function can't be nil, the work function provides the ability to workers to process the tasks.")
	}

	config := &poolResultConfig{
		MaxWorkers:   25,
		BufferSize:   0,
		IdleDuration: time.Second,
		KeepAlive:    false,
	}

	for _, option := range options {
		if err := option(config); err != nil {
			return nil, err
		}
	}

	pool := &PoolResult[T, R]{
		MaxWorkers:   config.MaxWorkers,
		IdleDuration: config.IdleDuration,
		KeepAlive:    config.KeepAlive,

		ctx:  ctx,
		work: work,

		queueDone:   make(chan struct{}, 1),
		queue:       make(chan T, config.BufferSize),
		results:     make(chan R, config.BufferSize),
		errors:      make(chan error, config.BufferSize),
		monitorDone: make(chan struct{}, 1),
		events:      make(chan event, config.BufferSize),
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
	defer close(p.monitorDone)
	defer close(p.events)
	defer func() {
		p.workers.Wait()
		close(p.results)
		close(p.errors)
	}()

	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

lifecycle:
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			if p.tasksEnqueued.Load() > 0 && p.workersIdle.Load() <= 0 {
				p.submitWorker(p.work)
			}
		case event, open := <-p.events:
			if !open {
				break
			}
			p.eventsHandled.Add(1)

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
				case workerCreatedState: // no-op
				case workerDoneState: // Shouldn't happen, DONE is a final state
				case workerFailedState: // Shouldn't happen, FAILED is a final state
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
					if p.IsDone() {
						break lifecycle
					}
				case workerFailedState:
					p.workersFailed.Add(1)
					p.workersAlive.Add(-1)
					if p.IsDone() {
						break lifecycle
					}
				}
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
					if !timer.Stop() {
						<-timer.C
					}
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
