package errors

import (
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestError_NewError(t *testing.T) {
	sentinel := io.EOF
	cause := io.ErrClosedPipe
	err := NewError(sentinel, cause, "something went wrong: %d", 42)

	var e *Error
	if !errors.As(err, &e) {
		t.Fatal("expected *Error type")
	}

	if e.Sentinel != sentinel {
		t.Errorf("Sentinel = %v, want %v", e.Sentinel, sentinel)
	}
	if e.Cause != cause {
		t.Errorf("Cause = %v, want %v", e.Cause, cause)
	}
	if e.Message != "something went wrong: 42" {
		t.Errorf("Message = %q, want %q", e.Message, "something went wrong: 42")
	}
}

func TestError_NewError_NilSentinelAndCause(t *testing.T) {
	err := NewError(nil, nil, "only message")

	var e *Error
	if !errors.As(err, &e) {
		t.Fatal("expected *Error type")
	}

	if e.Sentinel != nil {
		t.Errorf("Sentinel = %v, want nil", e.Sentinel)
	}
	if e.Cause != nil {
		t.Errorf("Cause = %v, want nil", e.Cause)
	}
	if e.Message != "only message" {
		t.Errorf("Message = %q, want %q", e.Message, "only message")
	}
}

func TestError_New(t *testing.T) {
	err := New("simple error: %d", 1)

	var e *Error
	if !errors.As(err, &e) {
		t.Fatal("expected *Error type")
	}

	if e.Sentinel != nil {
		t.Errorf("Sentinel = %v, want nil", e.Sentinel)
	}
	if e.Cause != nil {
		t.Errorf("Cause = %v, want nil", e.Cause)
	}
	if e.Message != "simple error: 1" {
		t.Errorf("Message = %q, want %q", e.Message, "simple error: 1")
	}
}

func TestError_New_NoArgs(t *testing.T) {
	err := New("no args")

	var e *Error
	if !errors.As(err, &e) {
		t.Fatal("expected *Error type")
	}

	if e.Message != "no args" {
		t.Errorf("Message = %q, want %q", e.Message, "no args")
	}
}

func TestError_Wrap(t *testing.T) {
	sentinel := io.EOF
	cause := io.ErrClosedPipe
	err := Wrap(sentinel, cause)

	var e *Error
	if !errors.As(err, &e) {
		t.Fatal("expected *Error type")
	}

	if e.Sentinel != sentinel {
		t.Errorf("Sentinel = %v, want %v", e.Sentinel, sentinel)
	}
	if e.Cause != cause {
		t.Errorf("Cause = %v, want %v", e.Cause, cause)
	}
	if e.Message != "" {
		t.Errorf("Message = %q, want empty", e.Message)
	}
}

func TestError_Wrap_NilCause(t *testing.T) {
	err := Wrap(io.EOF, nil)

	var e *Error
	if !errors.As(err, &e) {
		t.Fatal("expected *Error type")
	}

	if e.Cause != nil {
		t.Errorf("Cause = %v, want nil", e.Cause)
	}
}

func TestError_ErrorFormat(t *testing.T) {
	tests := []struct {
		name     string
		sentinel error
		message  string
		cause    error
		want     string
	}{
		{
			name: "all empty",
			want: "",
		},
		{
			name:     "only sentinel",
			sentinel: io.EOF,
			want:     "EOF",
		},
		{
			name:    "only message",
			message: "something failed",
			want:    "something failed",
		},
		{
			name:  "only cause",
			cause: io.EOF,
			want:  "EOF",
		},
		{
			name:     "sentinel and message",
			sentinel: io.EOF,
			message:  "something failed",
			want:     "EOF: something failed",
		},
		{
			name:     "sentinel and cause",
			sentinel: io.EOF,
			cause:    io.ErrClosedPipe,
			want:     "EOF: io: read/write on closed pipe",
		},
		{
			name:     "sentinel, message, and cause",
			sentinel: io.EOF,
			message:  "something failed",
			cause:    io.ErrClosedPipe,
			want:     "EOF: something failed: io: read/write on closed pipe",
		},
		{
			name:    "message and cause",
			message: "something failed",
			cause:   io.EOF,
			want:    "something failed: EOF",
		},
		{
			name:     "message blank with sentinel and cause",
			sentinel: io.EOF,
			message:  "   ",
			cause:    io.ErrClosedPipe,
			want:     "EOF: io: read/write on closed pipe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Error{
				Sentinel: tt.sentinel,
				Message:  tt.message,
				Cause:    tt.cause,
			}
			if got := e.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestError_Unwrap_ReturnsCause(t *testing.T) {
	cause := io.EOF
	e := Error{Cause: cause}

	if got := e.Unwrap(); got != cause {
		t.Errorf("Unwrap() = %v, want %v", got, cause)
	}
}

func TestError_Unwrap_NilCause(t *testing.T) {
	e := Error{}
	if got := e.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func TestError_Is_NonErrorTargetViaSentinel(t *testing.T) {
	e := Error{Sentinel: io.EOF, Cause: io.ErrClosedPipe}
	if !errors.Is(&e, io.EOF) {
		t.Error("expected Is(io.EOF) to match via Sentinel")
	}
}

func TestError_Is_NonErrorTargetViaCause(t *testing.T) {
	e := Error{Sentinel: io.EOF, Cause: io.ErrClosedPipe}
	if !errors.Is(&e, io.ErrClosedPipe) {
		t.Error("expected Is(io.ErrClosedPipe) to match via Cause")
	}
}

func TestError_Is_NonErrorTargetNoMatch(t *testing.T) {
	e := Error{Sentinel: io.EOF, Cause: io.ErrClosedPipe}
	if errors.Is(&e, io.ErrNoProgress) {
		t.Error("expected Is(io.ErrNoProgress) to be false")
	}
}

func TestError_Is_WithWrappedCauseChain(t *testing.T) {
	inner := io.EOF
	middle := fmt.Errorf("middle: %w", inner)

	e := Error{Cause: middle}

	if !errors.Is(&e, io.EOF) {
		t.Error("expected Is(io.EOF) to traverse cause chain")
	}
}

func TestError_Is_ErrorTargetMatchingSentinels(t *testing.T) {
	e1 := &Error{Sentinel: io.EOF, Cause: io.ErrClosedPipe}
	e2 := &Error{Sentinel: io.EOF, Cause: io.ErrNoProgress}

	if !errors.Is(e1, e2) {
		t.Error("expected *Error with matching Sentinels to be equal")
	}
}

func TestError_Is_ErrorTargetMatchingCauses(t *testing.T) {
	cause := fmt.Errorf("wrapped: %w", io.EOF)
	e1 := &Error{Sentinel: io.ErrClosedPipe, Cause: cause}
	e2 := &Error{Sentinel: io.ErrNoProgress, Cause: cause}

	if !errors.Is(e1, e2) {
		t.Error("expected *Error with matching Causes to be equal")
	}
}

func TestError_Is_ErrorTargetNoMatch(t *testing.T) {
	e1 := &Error{Sentinel: io.EOF, Cause: io.ErrClosedPipe}
	e2 := &Error{Sentinel: io.ErrNoProgress, Cause: io.EOF}

	if errors.Is(e1, e2) {
		t.Error("expected *Error with no matching Sentinel or Cause to not match (NOTE: may expose bug where e.Cause is compared to itself)")
	}
}

func TestError_Is_BothEmpty(t *testing.T) {
	e1 := &Error{}
	e2 := &Error{}

	if !errors.Is(e1, e2) {
		t.Error("expected two empty *Error to match")
	}
}

func TestPanicRecoveredError_String(t *testing.T) {
	if PanicRecoveredError.Error() != "PanicRecovered" {
		t.Errorf("PanicRecoveredError.Error() = %q, want %q", PanicRecoveredError.Error(), "PanicRecovered")
	}
}

func TestPanicRecoveredError_Identity(t *testing.T) {
	if !errors.Is(PanicRecoveredError, PanicRecoveredError) {
		t.Error("expected PanicRecoveredError to match itself")
	}
}

func TestWrap_PanicRecoveredError(t *testing.T) {
	err := Wrap(PanicRecoveredError, io.EOF)

	if !errors.Is(err, PanicRecoveredError) {
		t.Error("expected Wrap(PanicRecoveredError, io.EOF) to match PanicRecoveredError")
	}

	if !errors.Is(err, io.EOF) {
		t.Error("expected Wrap(PanicRecoveredError, io.EOF) to match io.EOF via Cause")
	}
}

func TestError_Is_NilReceiver(t *testing.T) {
	var e *Error
	if e != nil {
		t.Fatal("expected nil receiver")
	}
}
