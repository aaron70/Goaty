package errors

import "errors"

var (
	PanicRecoveredError error = errors.New("PanicRecovered")
)
