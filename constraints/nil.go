package constraints

import "github.com/aaron70/goaty/errors"

func NotZero[T comparable](v T) error {
	var zero T
	if v == zero {
		return errors.NewError(errors.ErrConstraintViolation, nil, "The given value is a zero value")
	}
	return nil
}
