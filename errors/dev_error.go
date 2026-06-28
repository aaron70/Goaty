package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/aaron70/goaty/validations"
)

type DevError struct {
	Sentinel error
	Message  string
	Cause    error
	file     string
	line     int
	stack    []uintptr
}

func NewDevError(sentinel error, cause error, msg string, args ...any) error {
	return newDevError(2, sentinel, cause, msg, args...)
}

func NewDev(msg string, args ...any) error {
	return newDevError(2, nil, nil, msg, args...)
}

func newDevError(calldepth int, sentinel error, cause error, msg string, args ...any) error {
	var pcs [32]uintptr
	n := runtime.Callers(calldepth, pcs[:])

	_, file, line, ok := runtime.Caller(calldepth)
	if !ok {
		file = "???"
	}

	return &DevError{
		Sentinel: sentinel,
		Cause:    cause,
		Message:  fmt.Sprintf(msg, args...),
		file:     file,
		line:     line,
		stack:    pcs[:n],
	}
}

func (e DevError) Error() string {
	msg := strings.Builder{}
	msg.WriteString(fmt.Sprintf("%s:%d", e.file, e.line))

	chain := strings.Builder{}
	if e.Sentinel != nil {
		chain.WriteString(e.Sentinel.Error())
	}
	if !validations.StrIsBlank(e.Message) {
		if chain.Len() > 0 {
			chain.WriteString(": ")
		}
		chain.WriteString(e.Message)
	}
	if e.Cause != nil {
		if chain.Len() > 0 {
			chain.WriteString(": ")
		}
		chain.WriteString(e.Cause.Error())
	}

	if chain.Len() > 0 {
		msg.WriteString(": ")
		msg.WriteString(chain.String())
	}

	msg.WriteString("\n")
	msg.WriteString(formatStack(e.stack))

	return msg.String()
}

func (e DevError) StackTrace() string {
	return formatStack(e.stack)
}

func (e *DevError) Unwrap() error {
	return e.Cause
}

func (e *DevError) Is(target error) bool {
	t, ok := target.(*DevError)
	if !ok {
		return errors.Is(e.Sentinel, target) || errors.Is(e.Cause, target)
	}
	return errors.Is(e.Sentinel, t.Sentinel) || errors.Is(e.Cause, t.Cause)
}

func formatStack(stack []uintptr) string {
	if len(stack) == 0 {
		return ""
	}

	frames := runtime.CallersFrames(stack)
	msg := strings.Builder{}

	for {
		frame, more := frames.Next()
		msg.WriteString(frame.Function)
		msg.WriteString("\n")
		msg.WriteString("\t")
		msg.WriteString(frame.File)
		msg.WriteString(":")
		msg.WriteString(fmt.Sprintf("%d", frame.Line))
		msg.WriteString("\n")

		if !more {
			break
		}
	}

	return msg.String()
}
