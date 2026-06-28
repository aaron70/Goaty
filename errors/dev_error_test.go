package errors

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDevError_NewDevError(t *testing.T) {
	sentinel := io.EOF
	cause := io.ErrClosedPipe
	err := NewDevError(sentinel, cause, "something went wrong: %d", 42)

	var e *DevError
	require.ErrorAs(t, err, &e, "expected *DevError type")

	assert.Equal(t, sentinel, e.Sentinel)
	assert.Equal(t, cause, e.Cause)
	assert.Equal(t, "something went wrong: 42", e.Message)
	assert.Contains(t, e.file, "dev_error_test.go")
	assert.Greater(t, e.line, 0)
	assert.NotEmpty(t, e.stack)
}

func TestDevError_NewDevError_NilSentinelAndCause(t *testing.T) {
	err := NewDevError(nil, nil, "only message")

	var e *DevError
	require.ErrorAs(t, err, &e, "expected *DevError type")

	assert.Nil(t, e.Sentinel)
	assert.Nil(t, e.Cause)
	assert.Equal(t, "only message", e.Message)
}

func TestDevError_NewDev(t *testing.T) {
	err := NewDev("simple error: %d", 1)

	var e *DevError
	require.ErrorAs(t, err, &e, "expected *DevError type")

	assert.Nil(t, e.Sentinel)
	assert.Nil(t, e.Cause)
	assert.Equal(t, "simple error: 1", e.Message)
}

func TestDevError_NewDev_NoArgs(t *testing.T) {
	err := NewDev("no args")

	var e *DevError
	require.ErrorAs(t, err, &e, "expected *DevError type")

	assert.Equal(t, "no args", e.Message)
}

func TestDevError_FileAndLineCaptured(t *testing.T) {
	err := NewDev("capture test")

	var e *DevError
	require.ErrorAs(t, err, &e, "expected *DevError type")

	assert.Contains(t, e.file, "dev_error_test.go")
	assert.Greater(t, e.line, 0)
}

func TestDevError_StackTrace_ContainsCaller(t *testing.T) {
	err := NewDev("stack trace test")

	var e *DevError
	require.ErrorAs(t, err, &e, "expected *DevError type")

	st := e.StackTrace()
	assert.Contains(t, st, "TestDevError_StackTrace_ContainsCaller")
	assert.Contains(t, st, "dev_error_test.go")
}

func TestDevError_ErrorFormat(t *testing.T) {
	tests := []struct {
		name     string
		sentinel error
		message  string
		cause    error
	}{
		{
			name: "only file and line",
		},
		{
			name:     "only sentinel",
			sentinel: io.EOF,
		},
		{
			name:    "only message",
			message: "something failed",
		},
		{
			name:  "only cause",
			cause: io.EOF,
		},
		{
			name:     "sentinel and message",
			sentinel: io.EOF,
			message:  "something failed",
		},
		{
			name:     "sentinel and cause",
			sentinel: io.EOF,
			cause:    io.ErrClosedPipe,
		},
		{
			name:     "sentinel, message, and cause",
			sentinel: io.EOF,
			message:  "something failed",
			cause:    io.ErrClosedPipe,
		},
		{
			name:    "message and cause",
			message: "something failed",
			cause:   io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := DevError{
				Sentinel: tt.sentinel,
				Message:  tt.message,
				Cause:    tt.cause,
				file:     "test.go",
				line:     42,
				stack:    []uintptr{},
			}

			lines := strings.SplitN(e.Error(), "\n", 2)
			firstLine := lines[0]

			prefix := "test.go:42"
			assert.True(t, strings.HasPrefix(firstLine, prefix),
				"expected %q to start with %q", firstLine, prefix)
		})
	}
}

func TestDevError_ErrorFormatChain(t *testing.T) {
	tests := []struct {
		name     string
		sentinel error
		message  string
		cause    error
		want     string
	}{
		{
			name: "only file and line",
			want: "test.go:42",
		},
		{
			name:     "only sentinel",
			sentinel: io.EOF,
			want:     "test.go:42: EOF",
		},
		{
			name:    "only message",
			message: "something failed",
			want:    "test.go:42: something failed",
		},
		{
			name:  "only cause",
			cause: io.EOF,
			want:  "test.go:42: EOF",
		},
		{
			name:     "sentinel and message",
			sentinel: io.EOF,
			message:  "something failed",
			want:     "test.go:42: EOF: something failed",
		},
		{
			name:     "sentinel and cause",
			sentinel: io.EOF,
			cause:    io.ErrClosedPipe,
			want:     "test.go:42: EOF: io: read/write on closed pipe",
		},
		{
			name:     "sentinel, message, and cause",
			sentinel: io.EOF,
			message:  "something failed",
			cause:    io.ErrClosedPipe,
			want:     "test.go:42: EOF: something failed: io: read/write on closed pipe",
		},
		{
			name:    "message and cause",
			message: "something failed",
			cause:   io.EOF,
			want:    "test.go:42: something failed: EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := DevError{
				Sentinel: tt.sentinel,
				Message:  tt.message,
				Cause:    tt.cause,
				file:     "test.go",
				line:     42,
				stack:    []uintptr{},
			}

			firstLine, _, _ := strings.Cut(e.Error(), "\n")
			assert.Equal(t, tt.want, firstLine)
		})
	}
}

func TestDevError_ErrorFormat_EmptyMessage(t *testing.T) {
	e := DevError{
		Sentinel: io.EOF,
		Message:  "",
		Cause:    io.ErrClosedPipe,
		file:     "test.go",
		line:     42,
		stack:    []uintptr{},
	}

	firstLine, _, _ := strings.Cut(e.Error(), "\n")
	assert.Equal(t, "test.go:42: EOF: io: read/write on closed pipe", firstLine)
}

func TestDevError_ErrorFormat_BlankMessage(t *testing.T) {
	e := DevError{
		Sentinel: io.EOF,
		Message:  "   ",
		Cause:    io.ErrClosedPipe,
		file:     "test.go",
		line:     42,
		stack:    []uintptr{},
	}

	firstLine, _, _ := strings.Cut(e.Error(), "\n")
	assert.Equal(t, "test.go:42: EOF: io: read/write on closed pipe", firstLine)
}

func TestDevError_ErrorFormat_WithStack(t *testing.T) {
	// Use a real DevError to get a real stack
	err := NewDev("stack test")

	var e *DevError
	require.ErrorAs(t, err, &e)

	errStr := e.Error()
	lines := strings.SplitN(errStr, "\n", 2)
	require.Len(t, lines, 2)

	firstLine := lines[0]
	assert.Contains(t, firstLine, "dev_error_test.go")

	stackPart := lines[1]
	assert.NotEmpty(t, stackPart)
	assert.Contains(t, stackPart, "TestDevError_ErrorFormat_WithStack")
	assert.Contains(t, stackPart, "dev_error_test.go")
}

func TestDevError_Unwrap_ReturnsCause(t *testing.T) {
	cause := io.EOF
	e := DevError{Cause: cause}

	assert.Equal(t, cause, e.Unwrap())
}

func TestDevError_Unwrap_NilCause(t *testing.T) {
	e := DevError{}

	assert.Nil(t, e.Unwrap())
}

func TestDevError_Is_NonErrorTargetViaSentinel(t *testing.T) {
	e := DevError{Sentinel: io.EOF, Cause: io.ErrClosedPipe}

	assert.True(t, errors.Is(&e, io.EOF), "expected errors.Is(io.EOF) to match via Sentinel")
}

func TestDevError_Is_NonErrorTargetViaCause(t *testing.T) {
	e := DevError{Sentinel: io.EOF, Cause: io.ErrClosedPipe}

	assert.True(t, errors.Is(&e, io.ErrClosedPipe), "expected errors.Is(io.ErrClosedPipe) to match via Cause")
}

func TestDevError_Is_NonErrorTargetNoMatch(t *testing.T) {
	e := DevError{Sentinel: io.EOF, Cause: io.ErrClosedPipe}

	assert.False(t, errors.Is(&e, io.ErrNoProgress), "expected errors.Is(io.ErrNoProgress) to be false")
}

func TestDevError_Is_WithWrappedCauseChain(t *testing.T) {
	inner := io.EOF
	middle := fmt.Errorf("middle: %w", inner)

	e := DevError{Cause: middle}

	assert.True(t, errors.Is(&e, io.EOF), "expected errors.Is(io.EOF) to traverse cause chain")
}

func TestDevError_Is_DevErrorTargetMatchingSentinels(t *testing.T) {
	e1 := &DevError{Sentinel: io.EOF, Cause: io.ErrClosedPipe}
	e2 := &DevError{Sentinel: io.EOF, Cause: io.ErrNoProgress}

	assert.True(t, errors.Is(e1, e2), "expected *DevError with matching Sentinels to be equal")
}

func TestDevError_Is_DevErrorTargetMatchingCauses(t *testing.T) {
	cause := fmt.Errorf("wrapped: %w", io.EOF)
	e1 := &DevError{Sentinel: io.ErrClosedPipe, Cause: cause}
	e2 := &DevError{Sentinel: io.ErrNoProgress, Cause: cause}

	assert.True(t, errors.Is(e1, e2), "expected *DevError with matching Causes to be equal")
}

func TestDevError_Is_DevErrorTargetNoMatch(t *testing.T) {
	e1 := &DevError{Sentinel: io.EOF, Cause: io.ErrClosedPipe}
	e2 := &DevError{Sentinel: io.ErrNoProgress, Cause: io.EOF}

	assert.False(t, errors.Is(e1, e2), "expected *DevError with no matching Sentinel or Cause to not match")
}

func TestDevError_Is_BothEmpty(t *testing.T) {
	e1 := &DevError{}
	e2 := &DevError{}

	assert.True(t, errors.Is(e1, e2), "expected two empty *DevError to match")
}

func TestDevError_Is_NilReceiver(t *testing.T) {
	var e *DevError
	assert.Nil(t, e)
}

func TestDevError_StackTrace_ReturnsEmptyForNoStack(t *testing.T) {
	e := DevError{}

	assert.Empty(t, e.StackTrace())
}

func TestDevError_NilSentinelAndCauseViaNewDev(t *testing.T) {
	err := NewDev("dev message")

	assert.NotNil(t, err)

	var e *DevError
	require.ErrorAs(t, err, &e)

	assert.Nil(t, e.Sentinel)
	assert.Nil(t, e.Cause)
	assert.Equal(t, "dev message", e.Message)
}

func TestDevError_SentinelChain(t *testing.T) {
	err := NewDevError(ErrInvalidArgument, io.EOF, "value must be positive")

	var e *DevError
	require.ErrorAs(t, err, &e)

	assert.True(t, errors.Is(e, ErrInvalidArgument))
	assert.True(t, errors.Is(e, io.EOF))
	assert.False(t, errors.Is(e, ErrNotFound))
}
