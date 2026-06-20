package resilience

import (
	"context"
	stderr "errors"
	"testing"
	"time"

	goatyerrors "github.com/aaron70/goaty/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTestFail = stderr.New("test error")

func TestNewRetryableWithResult(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		r, err := NewRetryableWithResult[string]()
		require.NoError(t, err)
		assert.Equal(t, 0, r.Config.MaxRetries)

		shouldRetry, err := r.Config.ShouldRetry(1, errTestFail)
		assert.True(t, shouldRetry)
		assert.NoError(t, err)

		assert.Equal(t, time.Duration(0), r.Config.BackOffFunc(1, errTestFail))
	})

	t.Run("all valid options", func(t *testing.T) {
		r, err := NewRetryableWithResult[string](
			NewRetryableWithMaxRetries(5),
			NewRetryableWithShouldRetry(func(retry int, err error) (bool, error) { return false, nil }),
			NewRetryableWithBackOffFunc(func(retry int, err error) time.Duration { return time.Second }),
			NewRetryableWithOnRetry(func(retry int, err error) {}),
			NewRetryableWithOnFailure(func(retry int, err error) {}),
			NewRetryableWithOnSuccess(func(retry int) {}),
		)
		require.NoError(t, err)
		assert.Equal(t, 5, r.Config.MaxRetries)
	})

	t.Run("negative MaxRetries", func(t *testing.T) {
		_, err := NewRetryableWithResult[string](
			NewRetryableWithMaxRetries(-1),
		)
		require.Error(t, err)
		assert.True(t, stderr.Is(err, goatyerrors.ErrInvalidArgument))
	})

	t.Run("nil ShouldRetry", func(t *testing.T) {
		_, err := NewRetryableWithResult[string](
			NewRetryableWithShouldRetry(nil),
		)
		require.Error(t, err)
		assert.True(t, stderr.Is(err, goatyerrors.ErrInvalidArgument))
		assert.True(t, stderr.Is(err, goatyerrors.ErrNilReference))
	})

	t.Run("nil BackOffFunc", func(t *testing.T) {
		_, err := NewRetryableWithResult[string](
			NewRetryableWithBackOffFunc(nil),
		)
		require.Error(t, err)
		assert.True(t, stderr.Is(err, goatyerrors.ErrInvalidArgument))
		assert.True(t, stderr.Is(err, goatyerrors.ErrNilReference))
	})

	t.Run("nil OnRetry", func(t *testing.T) {
		_, err := NewRetryableWithResult[string](
			NewRetryableWithOnRetry(nil),
		)
		require.Error(t, err)
		assert.True(t, stderr.Is(err, goatyerrors.ErrInvalidArgument))
		assert.True(t, stderr.Is(err, goatyerrors.ErrNilReference))
	})

	t.Run("nil OnFailure", func(t *testing.T) {
		_, err := NewRetryableWithResult[string](
			NewRetryableWithOnFailure(nil),
		)
		require.Error(t, err)
		assert.True(t, stderr.Is(err, goatyerrors.ErrInvalidArgument))
		assert.True(t, stderr.Is(err, goatyerrors.ErrNilReference))
	})

	t.Run("nil OnSuccess", func(t *testing.T) {
		_, err := NewRetryableWithResult[string](
			NewRetryableWithOnSuccess(nil),
		)
		require.Error(t, err)
		assert.True(t, stderr.Is(err, goatyerrors.ErrInvalidArgument))
		assert.True(t, stderr.Is(err, goatyerrors.ErrNilReference))
	})
}

func TestRetryableWithResult_RetryWithResult(t *testing.T) {
	t.Run("immediate success", func(t *testing.T) {
		var onSuccessCall int
		r, err := NewRetryableWithResult[string](
			NewRetryableWithMaxRetries(3),
			NewRetryableWithOnSuccess(func(retry int) {
				onSuccessCall = retry
			}),
		)
		require.NoError(t, err)

		result, err := r.RetryWithResult(t.Context(), func(ctx context.Context) (string, error) {
			return "ok", nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "ok", result)
		assert.Equal(t, 0, onSuccessCall)
	})

	t.Run("success after retries", func(t *testing.T) {
		var (
			attempt   int
			retries   []int
			onSuccess int
		)
		r, err := NewRetryableWithResult[string](
			NewRetryableWithMaxRetries(3),
			NewRetryableWithBackOffFunc(func(retry int, err error) time.Duration {
				return time.Millisecond
			}),
			NewRetryableWithOnRetry(func(retry int, err error) {
				retries = append(retries, retry)
			}),
			NewRetryableWithOnSuccess(func(retry int) {
				onSuccess = retry
			}),
		)
		require.NoError(t, err)

		result, err := r.RetryWithResult(t.Context(), func(ctx context.Context) (string, error) {
			attempt++
			if attempt < 3 {
				return "", errTestFail
			}
			return "ok", nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "ok", result)
		assert.Equal(t, []int{1, 2}, retries)
		assert.Equal(t, 2, onSuccess)
	})

	t.Run("exhausted", func(t *testing.T) {
		var (
			attempts  int
			failureErr error
		)
		r, err := NewRetryableWithResult[string](
			NewRetryableWithMaxRetries(2),
			NewRetryableWithBackOffFunc(func(retry int, err error) time.Duration {
				return time.Millisecond
			}),
			NewRetryableWithOnFailure(func(retry int, err error) {
				failureErr = err
			}),
		)
		require.NoError(t, err)

		result, err := r.RetryWithResult(t.Context(), func(ctx context.Context) (string, error) {
			attempts++
			return "", errTestFail
		})
		require.Error(t, err)
		assert.Empty(t, result)
		assert.Equal(t, 3, attempts)
		require.NotNil(t, failureErr)
		assert.True(t, stderr.Is(failureErr, ErrMaxRetriesExhausted))
	})

	t.Run("cancel retry", func(t *testing.T) {
		var failureErr error
		r, err := NewRetryableWithResult[string](
			NewRetryableWithMaxRetries(5),
			NewRetryableWithShouldRetry(func(retry int, err error) (bool, error) {
				return retry <= 1, nil
			}),
			NewRetryableWithBackOffFunc(func(retry int, err error) time.Duration {
				return time.Millisecond
			}),
			NewRetryableWithOnFailure(func(retry int, err error) {
				failureErr = err
			}),
		)
		require.NoError(t, err)

		result, err := r.RetryWithResult(t.Context(), func(ctx context.Context) (string, error) {
			return "", errTestFail
		})
		require.Error(t, err)
		assert.Empty(t, result)
		assert.True(t, stderr.Is(err, errTestFail))
		require.NotNil(t, failureErr)
		assert.True(t, stderr.Is(failureErr, ErrRetryCanceled))
	})

	t.Run("context cancelled during backoff", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		r, err := NewRetryableWithResult[string](
			NewRetryableWithMaxRetries(3),
			NewRetryableWithBackOffFunc(func(retry int, err error) time.Duration {
				return time.Hour
			}),
		)
		require.NoError(t, err)

		go func() {
			time.Sleep(time.Millisecond)
			cancel()
		}()

		result, err := r.RetryWithResult(ctx, func(ctx context.Context) (string, error) {
			return "", errTestFail
		})
		require.Error(t, err)
		assert.True(t, stderr.Is(err, context.Canceled))
		assert.Empty(t, result)
	})

	t.Run("nil context defaults to Background", func(t *testing.T) {
		r, err := NewRetryableWithResult[string](
			NewRetryableWithMaxRetries(1),
		)
		require.NoError(t, err)

		result, err := r.RetryWithResult(nil, func(ctx context.Context) (string, error) {
			return "ok", nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "ok", result)
	})
}

func TestRetryWithResult(t *testing.T) {
	t.Run("immediate success", func(t *testing.T) {
		result, err := RetryWithResult(
			t.Context(),
			func(ctx context.Context) (string, error) {
				return "ok", nil
			},
			NewRetryableWithMaxRetries(3),
		)
		assert.NoError(t, err)
		assert.Equal(t, "ok", result)
	})

	t.Run("success after retries", func(t *testing.T) {
		var attempt int
		result, err := RetryWithResult(
			t.Context(),
			func(ctx context.Context) (string, error) {
				attempt++
				if attempt < 2 {
					return "", errTestFail
				}
				return "ok", nil
			},
			NewRetryableWithMaxRetries(3),
			NewRetryableWithBackOffFunc(func(retry int, err error) time.Duration {
				return time.Millisecond
			}),
		)
		assert.NoError(t, err)
		assert.Equal(t, "ok", result)
	})

	t.Run("exhausted", func(t *testing.T) {
		var attempt int
		result, err := RetryWithResult(
			t.Context(),
			func(ctx context.Context) (string, error) {
				attempt++
				return "", errTestFail
			},
			NewRetryableWithMaxRetries(2),
			NewRetryableWithBackOffFunc(func(retry int, err error) time.Duration {
				return time.Millisecond
			}),
		)
		require.Error(t, err)
		assert.Empty(t, result)
		assert.Equal(t, 3, attempt)
		assert.True(t, stderr.Is(err, ErrMaxRetriesExhausted))
	})

	t.Run("cancel retry", func(t *testing.T) {
		result, err := RetryWithResult(
			t.Context(),
			func(ctx context.Context) (string, error) {
				return "", errTestFail
			},
			NewRetryableWithMaxRetries(5),
			NewRetryableWithShouldRetry(func(retry int, err error) (bool, error) {
				return retry <= 1, nil
			}),
			NewRetryableWithBackOffFunc(func(retry int, err error) time.Duration {
				return time.Millisecond
			}),
		)
		require.Error(t, err)
		assert.Empty(t, result)
		assert.True(t, stderr.Is(err, errTestFail))
	})

	t.Run("should retry error", func(t *testing.T) {
		errShouldRetry := stderr.New("should retry error")
		result, err := RetryWithResult(
			t.Context(),
			func(ctx context.Context) (string, error) {
				return "", errTestFail
			},
			NewRetryableWithMaxRetries(5),
			NewRetryableWithShouldRetry(func(retry int, err error) (bool, error) {
				return false, errShouldRetry
			}),
		)
		require.Error(t, err)
		assert.Empty(t, result)
		assert.True(t, stderr.Is(err, errTestFail))
	})

	t.Run("invalid option", func(t *testing.T) {
		result, err := RetryWithResult[string](
			t.Context(),
			func(ctx context.Context) (string, error) {
				return "ok", nil
			},
			NewRetryableWithMaxRetries(-1),
		)
		require.Error(t, err)
		assert.Empty(t, result)
		assert.True(t, stderr.Is(err, goatyerrors.ErrInvalidArgument))
	})

	t.Run("nil context defaults to Background", func(t *testing.T) {
		result, err := RetryWithResult(
			nil,
			func(ctx context.Context) (string, error) {
				return "ok", nil
			},
			NewRetryableWithMaxRetries(1),
		)
		assert.NoError(t, err)
		assert.Equal(t, "ok", result)
	})
}

func TestRetry(t *testing.T) {
	t.Run("immediate success", func(t *testing.T) {
		err := Retry(
			t.Context(),
			func(ctx context.Context) error {
				return nil
			},
			NewRetryableWithMaxRetries(3),
		)
		assert.NoError(t, err)
	})

	t.Run("success after retries", func(t *testing.T) {
		var attempt int
		err := Retry(
			t.Context(),
			func(ctx context.Context) error {
				attempt++
				if attempt < 2 {
					return errTestFail
				}
				return nil
			},
			NewRetryableWithMaxRetries(3),
			NewRetryableWithBackOffFunc(func(retry int, err error) time.Duration {
				return time.Millisecond
			}),
		)
		assert.NoError(t, err)
	})

	t.Run("exhausted", func(t *testing.T) {
		var attempt int
		err := Retry(
			t.Context(),
			func(ctx context.Context) error {
				attempt++
				return errTestFail
			},
			NewRetryableWithMaxRetries(2),
			NewRetryableWithBackOffFunc(func(retry int, err error) time.Duration {
				return time.Millisecond
			}),
		)
		require.Error(t, err)
		assert.Equal(t, 3, attempt)
		assert.True(t, stderr.Is(err, ErrMaxRetriesExhausted))
	})

	t.Run("cancel retry", func(t *testing.T) {
		err := Retry(
			t.Context(),
			func(ctx context.Context) error {
				return errTestFail
			},
			NewRetryableWithMaxRetries(5),
			NewRetryableWithShouldRetry(func(retry int, err error) (bool, error) {
				return retry <= 1, nil
			}),
			NewRetryableWithBackOffFunc(func(retry int, err error) time.Duration {
				return time.Millisecond
			}),
		)
		require.Error(t, err)
		assert.True(t, stderr.Is(err, errTestFail))
	})

	t.Run("invalid option", func(t *testing.T) {
		err := Retry(
			t.Context(),
			func(ctx context.Context) error {
				return nil
			},
			NewRetryableWithMaxRetries(-1),
		)
		require.Error(t, err)
		assert.True(t, stderr.Is(err, goatyerrors.ErrInvalidArgument))
	})

	t.Run("nil context defaults to Background", func(t *testing.T) {
		err := Retry(
			nil,
			func(ctx context.Context) error {
				return nil
			},
			NewRetryableWithMaxRetries(1),
		)
		assert.NoError(t, err)
	})
}
