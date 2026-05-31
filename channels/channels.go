package channels

import (
	"context"
	"fmt"
	"sync"

	"github.com/aaron70/goaty/errors"
)

func Send[T any](ctx context.Context, ch chan<- T, value T) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Wrap(errors.ErrPanicRecovered, fmt.Errorf("%+v", r))
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case ch <- value:
		return nil
	}
}

func Recv[T any](ctx context.Context, ch <-chan T) (T, bool, error) {
	var zero T
	select {
	case <-ctx.Done():
		return zero, false, ctx.Err()
	case value, open := <-ch:
		return value, open, nil
	}
}

func Drain[T any](ctx context.Context, ch <-chan T) error {
	var err error
	open := true
	for open {
		_, open, err = Recv(ctx, ch)
		if err != nil {
			return err
		}
	}
	return nil
}

func Merge[T any](ctx context.Context, buffer int, channels ...<-chan T) chan T {
	var wg sync.WaitGroup
	out := make(chan T, buffer)

	for _, ch := range channels {
		wg.Go(func() {
			for {
				v, open, err := Recv(ctx, ch)
				if err != nil || !open {
					return
				}
				err = Send(ctx, out, v)
				if err != nil {
					return
				}
			}
		})
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
