package channels

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goatyerrors "github.com/aaron70/goaty/errors"
)

func TestSend(t *testing.T) {
	t.Run("successful send", func(t *testing.T) {
		ch := make(chan string, 1)
		err := Send(t.Context(), ch, "hello")
		assert.NoError(t, err)
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := Send(ctx, make(chan string), "hello")
		assert.ErrorIs(t, err, context.Canceled)
		cancel()
	})

	t.Run("nil channel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := Send(ctx, nil, "hello")
		assert.ErrorIs(t, err, context.Canceled)
		cancel()
	})

	t.Run("closed channel", func(t *testing.T) {
		ch := make(chan string, 1)
		close(ch)
		err := Send(t.Context(), ch, "hello")
		require.ErrorIs(t, err, goatyerrors.PanicRecoveredError)

		var e *goatyerrors.Error
		if assert.ErrorAs(t, err, &e) {
			assert.Equal(t, "send on closed channel", e.Cause.Error())
		}
	})
}

func TestRecv(t *testing.T) {
	t.Run("successful receive", func(t *testing.T) {
		ch := make(chan string, 1)
		ch <- "hello"
		got, open, err := Recv(t.Context(), ch)
		assert.NoError(t, err)
		assert.Equal(t, "hello", got)
		assert.True(t, open)
	})

	t.Run("closed channel", func(t *testing.T) {
		ch := make(chan string, 1)
		close(ch)
		got, open, err := Recv(t.Context(), ch)
		assert.NoError(t, err)
		assert.Equal(t, "", got)
		assert.False(t, open)
	})

	t.Run("context cancelled", func(t *testing.T) {
		ch := make(chan string, 1)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		got, open, err := Recv(ctx, ch)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, "", got)
		assert.False(t, open)
		cancel()
	})

	t.Run("nil channel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		got, open, err := Recv[string](ctx, nil)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, "", got)
		assert.False(t, open)
		cancel()
	})
}

func TestDrain(t *testing.T) {
	t.Run("drain values", func(t *testing.T) {
		ch := make(chan string, 3)
		ch <- "a"
		ch <- "b"
		ch <- "c"
		close(ch)
		err := Drain(t.Context(), ch)
		assert.NoError(t, err)
	})

	t.Run("drain empty closed", func(t *testing.T) {
		ch := make(chan string, 1)
		close(ch)
		err := Drain(t.Context(), ch)
		assert.NoError(t, err)
	})

	t.Run("nil channel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := Drain[string](ctx, nil)
		assert.ErrorIs(t, err, context.Canceled)
		cancel()
	})
}
