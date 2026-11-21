package errors

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestError_NewError(t *testing.T) {
	sentinel := io.EOF
	cause := io.ErrClosedPipe
	err := NewError(sentinel, cause, "something went wrong: %d", 42)

	var e *Error
	require.ErrorAs(t, err, &e, "expected *Error type")

	assert.Equal(t, sentinel, e.Sentinel)
	assert.Equal(t, cause, e.Cause)
	assert.Equal(t, "something went wrong: 42", e.Message)
}

func TestError_NewError_NilSentinelAndCause(t *testing.T) {
	err := NewError(nil, nil, "only message")

	var e *Error
	require.ErrorAs(t, err, &e, "expected *Error type")

	assert.Nil(t, e.Sentinel)
	assert.Nil(t, e.Cause)
	assert.Equal(t, "only message", e.Message)
}

func TestError_New(t *testing.T) {
	err := New("simple error: %d", 1)

	var e *Error
	require.ErrorAs(t, err, &e, "expected *Error type")

	assert.Nil(t, e.Sentinel)
	assert.Nil(t, e.Cause)
	assert.Equal(t, "simple error: 1", e.Message)
}

func TestError_New_NoArgs(t *testing.T) {
	err := New("no args")

	var e *Error
	require.ErrorAs(t, err, &e, "expected *Error type")

	assert.Equal(t, "no args", e.Message)
}

func TestError_Wrap(t *testing.T) {
	sentinel := io.EOF
	cause := io.ErrClosedPipe
	err := Wrap(sentinel, cause)

	var e *Error
	require.ErrorAs(t, err, &e, "expected *Error type")

	assert.Equal(t, sentinel, e.Sentinel)
	assert.Equal(t, cause, e.Cause)
	assert.Empty(t, e.Message)
}

func TestError_Wrap_NilCause(t *testing.T) {
	err := Wrap(io.EOF, nil)

	var e *Error
	require.ErrorAs(t, err, &e, "expected *Error type")

	assert.Nil(t, e.Cause)
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
			assert.Equal(t, tt.want, e.Error())
		})
	}
}

func TestError_Unwrap_ReturnsCause(t *testing.T) {
	cause := io.EOF
	e := Error{Cause: cause}

	assert.Equal(t, cause, e.Unwrap())
}

func TestError_Unwrap_NilCause(t *testing.T) {
	e := Error{}

	assert.Nil(t, e.Unwrap())
}

func TestError_Is_NonErrorTargetViaSentinel(t *testing.T) {
	e := Error{Sentinel: io.EOF, Cause: io.ErrClosedPipe}

	assert.True(t, errors.Is(&e, io.EOF), "expected errors.Is(io.EOF) to match via Sentinel")
}

func TestError_Is_NonErrorTargetViaCause(t *testing.T) {
	e := Error{Sentinel: io.EOF, Cause: io.ErrClosedPipe}

	assert.True(t, errors.Is(&e, io.ErrClosedPipe), "expected errors.Is(io.ErrClosedPipe) to match via Cause")
}

func TestError_Is_NonErrorTargetNoMatch(t *testing.T) {
	e := Error{Sentinel: io.EOF, Cause: io.ErrClosedPipe}

	assert.False(t, errors.Is(&e, io.ErrNoProgress), "expected errors.Is(io.ErrNoProgress) to be false")
}

func TestError_Is_WithWrappedCauseChain(t *testing.T) {
	inner := io.EOF
	middle := fmt.Errorf("middle: %w", inner)

	e := Error{Cause: middle}

	assert.True(t, errors.Is(&e, io.EOF), "expected errors.Is(io.EOF) to traverse cause chain")
}

func TestError_Is_ErrorTargetMatchingSentinels(t *testing.T) {
	e1 := &Error{Sentinel: io.EOF, Cause: io.ErrClosedPipe}
	e2 := &Error{Sentinel: io.EOF, Cause: io.ErrNoProgress}

	assert.True(t, errors.Is(e1, e2), "expected *Error with matching Sentinels to be equal")
}

func TestError_Is_ErrorTargetMatchingCauses(t *testing.T) {
	cause := fmt.Errorf("wrapped: %w", io.EOF)
	e1 := &Error{Sentinel: io.ErrClosedPipe, Cause: cause}
	e2 := &Error{Sentinel: io.ErrNoProgress, Cause: cause}

	assert.True(t, errors.Is(e1, e2), "expected *Error with matching Causes to be equal")
}

func TestError_Is_ErrorTargetNoMatch(t *testing.T) {
	e1 := &Error{Sentinel: io.EOF, Cause: io.ErrClosedPipe}
	e2 := &Error{Sentinel: io.ErrNoProgress, Cause: io.EOF}

	assert.False(t, errors.Is(e1, e2), "expected *Error with no matching Sentinel or Cause to not match")
}

func TestError_Is_BothEmpty(t *testing.T) {
	e1 := &Error{}
	e2 := &Error{}

	assert.True(t, errors.Is(e1, e2), "expected two empty *Error to match")
}

func TestPanicRecoveredError_String(t *testing.T) {
	assert.Equal(t, "PanicRecovered", ErrPanicRecovered.Error())
}

func TestPanicRecoveredError_Identity(t *testing.T) {
	assert.True(t, errors.Is(ErrPanicRecovered, ErrPanicRecovered), "expected PanicRecoveredError to match itself")
}

func TestWrap_PanicRecoveredError(t *testing.T) {
	err := Wrap(ErrPanicRecovered, io.EOF)

	assert.True(t, errors.Is(err, ErrPanicRecovered), "expected Wrap(PanicRecoveredError, io.EOF) to match PanicRecoveredError")
	assert.True(t, errors.Is(err, io.EOF), "expected Wrap(PanicRecoveredError, io.EOF) to match io.EOF via Cause")
}

func TestError_Is_NilReceiver(t *testing.T) {
	var e *Error
	assert.Nil(t, e)
}
