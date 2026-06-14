package concurrency

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"text/tabwriter"
	"time"

	"github.com/aaron70/goaty/channels"
	"github.com/aaron70/goaty/errors"
	"github.com/google/uuid"
)

type Metrics struct {
	tasksEnqueued  int64
	tasksConsumed  int64
	tasksDone      int64
	tasksDelivered int64
	tasksFailed    int64

	workersAlive   int64
	workersCreated int64
	workersRunning int64
	workersIdle    int64
	workersDone    int64
	workersFailed  int64
}

type WorkerFuncReturn[T, R any] func(ctx context.Context, task T) (R, error)
type WorkerFunc[T, R any] func(ctx context.Context, task T, sendResult func(R), sendError func(error))

func ToWorkerFunc[T, R any](worker WorkerFuncReturn[T, R]) WorkerFunc[T, R] {
	return func(ctx context.Context, task T, sendResult func(R), sendError func(error)) {
		res, err := worker(ctx, task)
		if err != nil {
			sendError(err)
		} else {
			sendResult(res)
		}
	}
}

type PoolOption func(*PoolConfig) error

type PoolConfig struct {
	MaxWorkers   int
	MinWorkers   int
	BufferSize   int
	IdleDuration time.Duration
	KeepAlive    bool
}

type Pool[T, R any] struct {
	MaxWorkers   int
	MinWorkers   int
	IdleDuration time.Duration
	KeepAlive    bool

	ctx  context.Context
	work WorkerFunc[T, R]

	queueDone chan struct{}
	queue     chan T
	results   chan R
	errors    chan error

	tasksEnqueued  atomic.Int64
	tasksConsumed  atomic.Int64
	tasksDone      atomic.Int64
	tasksDelivered atomic.Int64
	tasksFailed    atomic.Int64
	workersAlive   atomic.Int64
	workersCreated atomic.Int64
	workersRunning atomic.Int64
	workersIdle    atomic.Int64
	workersDone    atomic.Int64
	workersFailed  atomic.Int64

	workers          sync.WaitGroup
	closeOnce        sync.Once
	closeResultsOnce sync.Once
}

func NewPoolWithMaxWorkers(maxWorkers int) PoolOption {
	return func(prc *PoolConfig) error {
		if maxWorkers <= 0 {
			return errors.NewError(errors.ErrInvalidArgument, nil, "maxWorkers must be a number greater than 0")
		}
		if maxWorkers < prc.MinWorkers {
			return errors.NewError(errors.ErrInvalidArgument, nil, "maxWorkers must be greater or equals than the minWorkers (%d)", prc.MinWorkers)
		}
		prc.MaxWorkers = maxWorkers
		return nil
	}
}

func NewPoolWithMinWorkers(minWorkers int) PoolOption {
	return func(prc *PoolConfig) error {
		if minWorkers < 0 {
			return errors.NewError(errors.ErrInvalidArgument, nil, "minWorkers must be a number greater than 0")
		}
		if minWorkers > prc.MaxWorkers {
			return errors.NewError(errors.ErrInvalidArgument, nil, "minWorkers must be less or equals than the maxWorkers (%d)", prc.MaxWorkers)
		}
		prc.MinWorkers = minWorkers
		return nil
	}
}

func NewPoolWithBufferSize(bufferSize int) PoolOption {
	return func(prc *PoolConfig) error {
		if bufferSize < 0 {
			return errors.NewError(errors.ErrInvalidArgument, nil, "bufferSize must be 0 or a positive number")
		}
		prc.BufferSize = bufferSize
		return nil
	}
}

// The idle duration should be greater than the duration that takes each record to be produced, to avoid creating a worker for each record.
func NewPoolWithIdleDuration(idleDuration time.Duration) PoolOption {
	return func(prc *PoolConfig) error {
		minIdleDuration := time.Millisecond
		if idleDuration < minIdleDuration {
			return errors.NewError(errors.ErrInvalidArgument, nil, "idleDuration must be greater than %s. Although %s is the minimum duration is not recommended to use too low values as the idle timer could trigger before the job could read the task, leaving the task in the queue forever.", minIdleDuration, minIdleDuration)
		}
		prc.IdleDuration = idleDuration
		return nil
	}
}

func NewPoolWithKeepAlive(KeepAlive bool) PoolOption {
	return func(prc *PoolConfig) error {
		prc.KeepAlive = KeepAlive
		return nil
	}
}

func NewPool[T, R any](ctx context.Context, work WorkerFunc[T, R], options ...PoolOption) (*Pool[T, R], error) {
	if ctx == nil {
		return nil, errors.NewError(errors.ErrInvalidArgument, errors.ErrNilReference, "Context should not be nil, please use content.Background() or similar if no need a context.")
	}

	if work == nil {
		return nil, errors.NewError(errors.ErrInvalidArgument, errors.ErrNilReference, "The work function can't be nil, the work function provides the ability to workers to process the tasks.")
	}

	config := &PoolConfig{
		MinWorkers:   0,
		MaxWorkers:   math.MaxInt,
		BufferSize:   0,
		IdleDuration: time.Second,
		KeepAlive:    false,
	}

	for _, option := range options {
		if err := option(config); err != nil {
			return nil, err
		}
	}

	pool := &Pool[T, R]{
		MinWorkers:   config.MinWorkers,
		MaxWorkers:   config.MaxWorkers,
		IdleDuration: config.IdleDuration,
		KeepAlive:    config.KeepAlive,

		ctx:  ctx,
		work: work,

		queueDone: make(chan struct{}, 1),
		queue:     make(chan T, config.BufferSize),
		results:   make(chan R, config.BufferSize),
		errors:    make(chan error, config.BufferSize),
	}

	for range pool.MinWorkers {
		pool.tryCreateWorker(pool.createWorker)
	}

	return pool, nil
}

func (p *Pool[T, R]) publishResult(ctx context.Context, res R) error {
	err := channels.Send(ctx, p.results, res)
	if err != nil {
		channels.Send(ctx, p.errors, err)
		return err
	}
	return nil
}

func (p *Pool[T, R]) publishError(ctx context.Context, err error) {
	channels.Send(ctx, p.errors, err)
}

func (p *Pool[T, R]) tryCreateWorker(worker func(ctx context.Context, id string) func()) bool {
	for {
		aliveWorkers := p.workersAlive.Load()
		if aliveWorkers >= int64(p.MaxWorkers) || (aliveWorkers >= int64(p.MinWorkers) && p.workersIdle.Load() > 0) {
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

func (p *Pool[T, R]) createWorker(ctx context.Context, _ string) func() {
	return func() {
		var state *atomic.Int64
		updateState := func(newState *atomic.Int64) {
			if state == newState {
				return
			}
			if state != nil {
				state.Add(-1)
			}
			state = newState
			newState.Add(1)
		}
		defer func() {
			if r := recover(); r != nil {
				updateState(&p.workersFailed)
				p.publishError(ctx, errors.NewError(errors.ErrPanicRecovered, nil, "%v", r))
			} else {
				updateState(&p.workersDone)
			}
			p.workersAlive.Add(-1)
		}()

		timer := time.NewTimer(p.IdleDuration) // TODO: Consider using a sync.pool of timers
		if p.KeepAlive {
			timer.C = nil // Will block for ever the <-time.C case
		}

		sendErr := func(err error) {
			p.taskFailed()
			p.publishError(ctx, err)
		}

		sendResult := func(res R) {
			p.taskDone()
			err := p.publishResult(ctx, res)
			if err != nil {
				p.taskFailed()
			} else {
				p.taskDelivered()
			}
		}

	lifecycle:
		for {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(p.IdleDuration)
			updateState(&p.workersIdle)
			select {
			case <-ctx.Done():
				err := ctx.Err()
				if err != nil {
					p.publishError(ctx, err)
				}
				break lifecycle
			case task, open := <-p.queue:
				if !open {
					break lifecycle
				}
				updateState(&p.workersRunning)
				p.taskConsumed()

				p.work(ctx, task, sendResult, sendErr)
			case <-timer.C:
				if p.KeepAlive || p.workersAlive.Load() <= int64(p.MinWorkers) {
					continue
				}
				timer.Stop()
				break lifecycle
			}
		}
	}
}

func (p *Pool[T, R]) taskConsumed() {
	p.tasksEnqueued.Add(-1)
	p.tasksConsumed.Add(1)
}

func (p *Pool[T, R]) taskDone() {
	p.tasksConsumed.Add(-1)
	p.tasksDone.Add(1)
}

func (p *Pool[T, R]) taskDelivered() {
	p.tasksDone.Add(-1)
	p.tasksDelivered.Add(1)
}

func (p *Pool[T, R]) taskFailed() {
	p.tasksConsumed.Add(-1)
	p.tasksFailed.Add(1)
}

func (p *Pool[T, R]) ProduceTasks(n int, producer func(index int) T) error {
	var wg sync.WaitGroup
	tasks := make(chan T)

	if producer == nil {
		producer = func(index int) T { var z T; return z }
	}

	wg.Go(func() {
		defer close(tasks)
		for i := range n {
			if err := channels.Send(p.ctx, tasks, producer(i)); err != nil {
				return
			}
		}
	})

	err := p.RecvTasks(tasks)
	wg.Wait()
	return err
}

func (p *Pool[T, R]) RecvTasks(tasks <-chan T) error {
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

		p.tryCreateWorker(p.createWorker)
		p.tasksEnqueued.Add(1)
		err = channels.Send(p.ctx, p.queue, task)
		if err != nil {
			p.tasksEnqueued.Add(-1)
			return errors.NewError(nil, err, "Could not enqueue the task")
		}
	}

	return nil
}

func (p *Pool[T, R]) ResultsErr() (<-chan R, <-chan error) {
	return p.results, p.errors
}

func (p *Pool[T, R]) closeResults() {
	p.closeResultsOnce.Do(func() {
		close(p.results)
		close(p.errors)
	})
}

func (p *Pool[T, R]) Close() {
	p.closeOnce.Do(func() {
		close(p.queue)
		close(p.queueDone)
	})
	go p.Wait()
}

func (p *Pool[T, R]) Wait() error {
	defer p.closeResults()
	for {
		_, open, err := channels.Recv(p.ctx, p.queueDone)
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

func (p *Pool[T, R]) IsDone() bool {
	return channels.IsDone(p.queueDone) && p.tasksEnqueued.Load() <= 0 && p.tasksConsumed.Load() <= 0 && p.workersAlive.Load() <= 0
}

func (p *Pool[T, R]) Metrics() Metrics {
	return Metrics{
		tasksEnqueued:  p.tasksEnqueued.Load(),
		tasksConsumed:  p.tasksConsumed.Load(),
		tasksDone:      p.tasksDone.Load(),
		tasksDelivered: p.tasksDelivered.Load(),
		tasksFailed:    p.tasksFailed.Load(),

		workersAlive:   p.workersAlive.Load(),
		workersCreated: p.workersCreated.Load(),
		workersRunning: p.workersRunning.Load(),
		workersIdle:    p.workersIdle.Load(),
		workersDone:    p.workersDone.Load(),
		workersFailed:  p.workersFailed.Load(),
	}
}

func PrintPools[T, R any](names []string, pools ...*Pool[T, R]) {
	fmt.Print("\033[H\033[2J")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	row := func(label string, values []string) {
		fmt.Fprintf(w, "%s\t%s\n", label, strings.Join(values, "\t"))
	}

	getMetric := func(fn func(*Pool[T, R]) any) []string {
		s := make([]string, len(pools))
		for i, pool := range pools {
			s[i] = fmt.Sprintf("%v", fn(pool))
		}
		return s
	}

	// Header
	fmt.Fprintf(w, "Metric\t%s\n", strings.Join(names, "\t"))
	fmt.Fprintf(w, "------\t%s\n", strings.Join(make([]string, len(names)), "\t"))
	// row("Queue is Done: ", getMetric(func(p *Pool[T, R]) any { return p.IsDone() }))
	// row("Queue accpets tasks: ", getMetric(func(p *Pool[T, R]) any { return p.AcceptsTasks() }))
	// fmt.Fprintf(w, "------\t%s\n", strings.Join(make([]string, len(names)), "\t"))

	// Task rows
	row("Tasks Enqueued", getMetric(func(p *Pool[T, R]) any { return p.Metrics().tasksEnqueued }))
	row("Tasks Consumed", getMetric(func(p *Pool[T, R]) any { return p.Metrics().tasksConsumed }))
	row("Tasks Done", getMetric(func(p *Pool[T, R]) any { return p.Metrics().tasksDone }))
	row("Tasks Delivered", getMetric(func(p *Pool[T, R]) any { return p.Metrics().tasksDelivered }))
	row("Tasks Failed", getMetric(func(p *Pool[T, R]) any { return p.Metrics().tasksFailed }))
	fmt.Fprintf(w, "------\t%s\n", strings.Join(make([]string, len(names)), "\t"))

	// Worker rows
	row("Workers Created", getMetric(func(p *Pool[T, R]) any { return p.Metrics().workersCreated }))
	row("Workers Alive", getMetric(func(p *Pool[T, R]) any { return p.Metrics().workersAlive }))
	row("Workers Running", getMetric(func(p *Pool[T, R]) any { return p.Metrics().workersRunning }))
	row("Workers Idle", getMetric(func(p *Pool[T, R]) any { return p.Metrics().workersIdle }))
	row("Workers Done", getMetric(func(p *Pool[T, R]) any { return p.Metrics().workersDone }))
	row("Workers Failed", getMetric(func(p *Pool[T, R]) any { return p.Metrics().workersFailed }))

	w.Flush()
}
