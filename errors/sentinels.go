package errors

import "errors"

var (
	ErrPanicRecovered      error = errors.New("PanicRecovered")
	ErrNotFound            error = errors.New("NotFound")
	ErrConflict            error = errors.New("Conflict")
	ErrInvalidArgument     error = errors.New("InvalidArgument")
	ErrNilReference        error = errors.New("NilReference")
	ErrConstraintViolation error = errors.New("ConstraintViolation")
)
