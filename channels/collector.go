package channels

import (
	"context"
	"sync"
	"time"
)

type BatchCollector[I any] struct {
	FlushAfter time.Duration
	C          chan []I

	batch     []I
	ticker    *time.Ticker
	seeker    int
	m         sync.Mutex
	flusher   sync.Once
	done      chan struct{}
	closeOnce sync.Once
}

func NewBatchCollector[I any](batchSize int, flushAfter time.Duration) (*BatchCollector[I], error) {
	return &BatchCollector[I]{
		FlushAfter: flushAfter,
		C:          make(chan []I),
		batch:      make([]I, batchSize),
		ticker:     time.NewTicker(flushAfter),
		done:       make(chan struct{}),
	}, nil
}

func (c *BatchCollector[I]) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
	})
}

func (c *BatchCollector[I]) add(ctx context.Context, value I) {
	go c.flusher.Do(func() {
		defer close(c.C)
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.done:
				c.flush(ctx)
				return
			case <-c.ticker.C:
				c.flush(ctx)
			}
		}
	})
	if c.seeker == 0 {
		c.ticker.Reset(c.FlushAfter)
	}
	c.batch[c.seeker] = value
	c.seeker++
	if c.seeker >= cap(c.batch) {
		c.flush(ctx)
	}
}

func (c *BatchCollector[I]) Add(ctx context.Context, value I) {
	c.m.Lock()
	defer c.m.Unlock()
	c.add(ctx, value)
}

func (c *BatchCollector[I]) flush(ctx context.Context) {
	if c.seeker > 0 {
		batch := make([]I, c.seeker)
		copy(batch, c.batch[:c.seeker])
		Send(ctx, c.C, batch)
		c.seeker = 0
	}
	c.ticker.Reset(c.FlushAfter)
}

func (c *BatchCollector[I]) Flush(ctx context.Context) {
	c.m.Lock()
	defer c.m.Unlock()
	c.flush(ctx)
}

func (c *BatchCollector[I]) Collect(ctx context.Context, in <-chan I) {
	for {
		v, open, err := Recv(ctx, in)
		if !open || err != nil {
			c.Close()
			return
		}
		c.Add(ctx, v)
	}
}
