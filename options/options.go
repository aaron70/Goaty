package options

import "github.com/aaron70/goaty/errors"

func Apply[T any, O ~func(*T) error](config *T, options ...O) error {
	for _, option := range options {
		if err := option(config); err != nil {
			return errors.NewError(errors.ErrInvalidArgument, err, "")
		}
	}
	return nil
}

func ApplyZero[T any, O ~func(*T) error](options ...O) (T, error) {
	var config T
	err := Apply(&config, options...)
	return config, err
}
