package resilience

import (
	"context"
	"time"

	"github.com/aaron70/goaty/errors"
)

var ErrMaxRetriesExhausted = errors.New("MaxRetriesExhausted")
var ErrRetryCanceled = errors.New("RetryCanceled")

type RetryConfig struct {
	MaxRetries  int
	ShouldRetry func(retry int, err error) (bool, error)
	BackOffFunc func(retry int, err error) time.Duration
	OnRetry     func(retry int, err error)
	OnFailure   func(retry int, err error)
	OnSuccess   func(retry int)
}

type RetryOption func(*RetryConfig) error

type RetryableFucWithResult[T any] func(ctx context.Context) (T, error)
type RetryableFuc func(ctx context.Context) error

type RetryableWithResult[T any] struct {
	Config RetryConfig
	timer  *time.Timer
}

func NewRetryableWithResult[T any](options ...RetryOption) (*RetryableWithResult[T], error) {
	config := &RetryConfig{
		ShouldRetry: func(retry int, err error) (bool, error) { return true, nil },
		BackOffFunc: func(retry int, err error) time.Duration { return 0 },
		OnRetry:     func(retry int, err error) {},
		OnFailure:   func(retry int, err error) {},
		OnSuccess:   func(retry int) {},
	}

	for _, option := range options {
		if err := option(config); err != nil {
			return nil, err
		}
	}

	return &RetryableWithResult[T]{
		Config: *config,
	}, nil
}

func NewRetryableWithMaxRetries(maxRetries int) RetryOption {
	return func(config *RetryConfig) error {
		if maxRetries < 0 {
			return errors.NewError(errors.ErrInvalidArgument, nil, "maxRetries must be a number greater or equal to 0")
		}
		config.MaxRetries = maxRetries
		return nil
	}
}

func NewRetryableWithShouldRetry(shouldRetry func(retry int, err error) (bool, error)) RetryOption {
	return func(config *RetryConfig) error {
		if shouldRetry == nil {
			return errors.NewError(errors.ErrInvalidArgument, errors.ErrNilReference, "ShouldRetry must not be nil")
		}
		config.ShouldRetry = shouldRetry
		return nil
	}
}

func NewRetryableWithBackOffFunc(backOffFunc func(retry int, err error) time.Duration) RetryOption {
	return func(config *RetryConfig) error {
		if backOffFunc == nil {
			return errors.NewError(errors.ErrInvalidArgument, errors.ErrNilReference, "BackOffFunc must not be nil")
		}
		config.BackOffFunc = backOffFunc
		return nil
	}
}

func NewRetryableWithOnRetry(onRetry func(retry int, err error)) RetryOption {
	return func(config *RetryConfig) error {
		if onRetry == nil {
			return errors.NewError(errors.ErrInvalidArgument, errors.ErrNilReference, "OnRetry must not be nil")
		}
		config.OnRetry = onRetry
		return nil
	}
}

func NewRetryableWithOnFailure(onFailure func(retry int, err error)) RetryOption {
	return func(config *RetryConfig) error {
		if onFailure == nil {
			return errors.NewError(errors.ErrInvalidArgument, errors.ErrNilReference, "OnFailure must not be nil")
		}
		config.OnFailure = onFailure
		return nil
	}
}

func NewRetryableWithOnSuccess(onSuccess func(retry int)) RetryOption {
	return func(config *RetryConfig) error {
		if onSuccess == nil {
			return errors.NewError(errors.ErrInvalidArgument, errors.ErrNilReference, "OnSuccess must not be nil")
		}
		config.OnSuccess = onSuccess
		return nil
	}
}

func (r *RetryableWithResult[T]) RetryWithResult(ctx context.Context, f RetryableFucWithResult[T]) (T, error) {
	var (
		zero T
		err  error
	)
	if ctx == nil {
		ctx = context.Background()
	}

	for attempt := range r.Config.MaxRetries + 1 {
		retryContext, cancel := context.WithCancel(ctx)
		res, err := f(retryContext)

		if err == nil {
			cancel()
			r.onSuccess(attempt)
			return res, nil
		}

		retryCount := attempt + 1
		if shouldRetry, cause := r.Config.ShouldRetry(retryCount, err); !shouldRetry {
			cancel()
			r.onFailure(retryCount, errors.NewError(ErrRetryCanceled, cause, "predicated failed, ShouldRetry returned false"))
			return zero, err
		}

		if err = r.onRetry(retryContext, retryCount, err); err != nil {
			cancel()
			return zero, err
		}

		cancel()
	}
	err = errors.NewError(ErrMaxRetriesExhausted, err, "Max retries %d exhausted", r.Config.MaxRetries)
	r.onFailure(r.Config.MaxRetries, err)
	return zero, err
}

func (r RetryableWithResult[T]) onSuccess(retry int) {
	r.Config.OnSuccess(retry)
}

func (r *RetryableWithResult[T]) onRetry(ctx context.Context, retry int, err error) error {
	backOffDuration := r.Config.BackOffFunc(retry, err)
	if r.timer == nil {
		r.timer = time.NewTimer(backOffDuration)
	} else {
		r.timer.Reset(backOffDuration)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.timer.C:
		r.Config.OnRetry(retry, err)
		return nil
	}
}

func (r RetryableWithResult[T]) onFailure(retry int, err error) {
	r.Config.OnFailure(retry, err)
}

func RetryWithResult[T any](ctx context.Context, f RetryableFucWithResult[T], options ...RetryOption) (T, error) {
	var zero T
	retryable, err := NewRetryableWithResult[T](options...)
	if err != nil {
		return zero, err
	}
	return retryable.RetryWithResult(ctx, f)
}

func Retry(ctx context.Context, f RetryableFuc, options ...RetryOption) error {
	retryable, err := NewRetryableWithResult[any](options...)
	if err != nil {
		return err
	}
	_, err = retryable.RetryWithResult(ctx, func(ctx context.Context) (any, error) {
		return nil, f(ctx)
	})
	return err
}
