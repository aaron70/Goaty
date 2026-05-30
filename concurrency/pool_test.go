package concurrency

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aaron70/goaty/channels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPool(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		pool, err := NewPool[string, any](t.Context())
		require.NoError(t, err)
		assert.Equal(t, 25, pool.MaxWorkers)
		assert.Equal(t, 0, pool.BufferSize)
		assert.Equal(t, time.Second*3, pool.IdleDuration)
	})

	t.Run("with options", func(t *testing.T) {
		pool, err := NewPool(t.Context(),
			NewPoolWithMaxWorkers[string, any](10),
			NewPoolWithBufferSize[string, any](5),
			NewPoolWithIdleDuration[string, any](time.Minute),
		)
		require.NoError(t, err)
		assert.Equal(t, 10, pool.MaxWorkers)
		assert.Equal(t, 5, pool.BufferSize)
		assert.Equal(t, time.Minute, pool.IdleDuration)
	})
}

func TestPushTasks(t *testing.T) {
	t.Run("closed input channel", func(t *testing.T) {
		pool, err := NewPool[string, any](t.Context())
		require.NoError(t, err)

		ch := make(chan string)
		close(ch)
		err = pool.PushTasks(ch, func(ctx context.Context, task string) {})
		assert.NoError(t, err)
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		pool, err := NewPool[string, any](ctx)
		require.NoError(t, err)

		ch := make(chan string)
		err = pool.PushTasks(ch, func(ctx context.Context, task string) {})
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("processes tasks", func(t *testing.T) {
		pool, err := NewPool(t.Context(),
			NewPoolWithMaxWorkers[string, any](3),
			NewPoolWithBufferSize[string, any](3),
		)
		require.NoError(t, err)

		var mu sync.Mutex
		var got []string

		ch := make(chan string, 3)
		ch <- "a"
		ch <- "b"
		ch <- "c"
		close(ch)

		err = pool.PushTasks(ch, func(ctx context.Context, task string) {
			mu.Lock()
			got = append(got, task)
			mu.Unlock()
		})
		require.NoError(t, err)

		pool.Wait()

		assert.ElementsMatch(t, []string{"a", "b", "c"}, got)
	})
}

func TestIdleTimeout(t *testing.T) {
	pool, err := NewPool(t.Context(),
		NewPoolWithMaxWorkers[string, any](1),
		NewPoolWithIdleDuration[string, any](10*time.Millisecond),
		NewPoolWithBufferSize[string, any](1),
	)
	require.NoError(t, err)

	synch := make(chan string)
	ch := make(chan string, 1)
	ch <- "task"

	var w sync.WaitGroup
	w.Go(func() {
		err = pool.PushTasks(ch, func(ctx context.Context, task string) {
			err := channels.Send(ctx, synch, task)
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})

	<-synch
	assert.Equal(t, int32(1), pool.aliveWorkers.Load())
	assert.Eventually(t, func() bool {
		return pool.aliveWorkers.Load() == 0
	}, 100*time.Millisecond, 5*time.Millisecond)
	close(ch)
	w.Wait()
}

func TestWait(t *testing.T) {
	pool, err := NewPool(t.Context(),
		NewPoolWithBufferSize[string, any](3),
	)
	require.NoError(t, err)

	ch := make(chan string, 3)
	ch <- "a"
	ch <- "b"
	ch <- "c"
	close(ch)

	err = pool.PushTasks(ch, func(ctx context.Context, task string) {})
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		pool.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Wait() did not return within 1 second")
	}
}
