package errors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aaron70/goaty/validations"
)

type Error struct {
	Sentinel error
	Message  string
	Cause    error
}

func NewError(sentinel error, cause error, msg string, args ...any) error {
	return &Error{
		Sentinel: sentinel,
		Cause:    cause,
		Message:  fmt.Sprintf(msg, args...),
	}
}

func New(msg string, args ...any) error {
	return NewError(nil, nil, msg, args...)
}

func Wrap(sentinel error, cause error) error {
	return NewError(sentinel, cause, "")
}

func (e Error) Error() string {
	msg := strings.Builder{}

	if e.Sentinel != nil {
		msg.WriteString(e.Sentinel.Error())
	}

	if !validations.StrIsBlank(e.Message) {
		if msg.Len() != 0 {
			msg.WriteString(": ")
		}
		msg.WriteString(e.Message)
	}

	if e.Cause != nil {
		if msg.Len() != 0 {
			msg.WriteString(": ")
		}
		msg.WriteString(e.Cause.Error())
	}

	return msg.String()
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func (e *Error) Is(target error) bool {
    t, ok := target.(*Error)
    if !ok {
        return errors.Is(e.Sentinel, target) || errors.Is(e.Cause, target)
    }
    return errors.Is(e.Sentinel, t.Sentinel) || errors.Is(e.Cause, t.Cause)
}
