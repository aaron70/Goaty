package channels

import (
	"context"
	"errors"
	"testing"

	goatyerrors "github.com/aaron70/goaty/errors"
)

func TestSend(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() (context.Context, context.CancelFunc, chan string, string)
		wantErr func(error) bool
	}{
		{
			name: "successful send",
			setup: func() (context.Context, context.CancelFunc, chan string, string) {
				return context.Background(), func() {}, make(chan string, 1), "hello"
			},
			wantErr: func(err error) bool { return err == nil },
		},
		{
			name: "context cancelled",
			setup: func() (context.Context, context.CancelFunc, chan string, string) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel, make(chan string), "hello"
			},
			wantErr: func(err error) bool { return errors.Is(err, context.Canceled) },
		},
		{
			name: "nil channel",
			setup: func() (context.Context, context.CancelFunc, chan string, string) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel, nil, "hello"
			},
			wantErr: func(err error) bool { return errors.Is(err, context.Canceled) },
		},
		{
			name: "closed channel",
			setup: func() (context.Context, context.CancelFunc, chan string, string) {
				ch := make(chan string, 1)
				close(ch)
				return context.Background(), func() {}, ch, "hello"
			},
			wantErr: func(err error) bool { return errors.Is(err, goatyerrors.PanicRecoveredError) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel, ch, val := tt.setup()
			defer cancel()

			err := Send(ctx, ch, val)

			if !tt.wantErr(err) {
				t.Errorf("Send() error = %v, want satisfying condition", err)
			}

			if tt.name == "closed channel" && err != nil {
				var e *goatyerrors.Error
				if errors.As(err, &e) && e.Cause != nil {
					if e.Cause.Error() != "send on closed channel" {
						t.Errorf("expected cause 'send on closed channel', got %q", e.Cause.Error())
					}
				}
			}
		})
	}
}

func TestRecv(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() (context.Context, context.CancelFunc, chan string)
		wantVal   string
		wantOpen  bool
		wantErrFn func(error) bool
	}{
		{
			name: "successful receive",
			setup: func() (context.Context, context.CancelFunc, chan string) {
				ch := make(chan string, 1)
				ch <- "hello"
				return context.Background(), func() {}, ch
			},
			wantVal:  "hello",
			wantOpen: true,
			wantErrFn: func(err error) bool { return err == nil },
		},
		{
			name: "closed channel",
			setup: func() (context.Context, context.CancelFunc, chan string) {
				ch := make(chan string, 1)
				close(ch)
				return context.Background(), func() {}, ch
			},
			wantVal:  "",
			wantOpen: false,
			wantErrFn: func(err error) bool { return err == nil },
		},
		{
			name: "context cancelled",
			setup: func() (context.Context, context.CancelFunc, chan string) {
				ch := make(chan string, 1)
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel, ch
			},
			wantVal:  "",
			wantOpen: false,
			wantErrFn: func(err error) bool { return errors.Is(err, context.Canceled) },
		},
		{
			name: "nil channel",
			setup: func() (context.Context, context.CancelFunc, chan string) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel, nil
			},
			wantVal:  "",
			wantOpen: false,
			wantErrFn: func(err error) bool { return errors.Is(err, context.Canceled) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel, ch := tt.setup()
			defer cancel()

			got, open, err := Recv(ctx, ch)

			if !tt.wantErrFn(err) {
				t.Errorf("Recv() error = %v, want satisfying condition", err)
			}
			if got != tt.wantVal {
				t.Errorf("Recv() got = %q, want %q", got, tt.wantVal)
			}
			if open != tt.wantOpen {
				t.Errorf("Recv() open = %v, want %v", open, tt.wantOpen)
			}
		})
	}
}

func TestDrain(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() (context.Context, context.CancelFunc, chan string)
		wantErr func(error) bool
	}{
		{
			name: "drain values",
			setup: func() (context.Context, context.CancelFunc, chan string) {
				ch := make(chan string, 3)
				ch <- "a"
				ch <- "b"
				ch <- "c"
				close(ch)
				return context.Background(), func() {}, ch
			},
			wantErr: func(err error) bool { return err == nil },
		},
		{
			name: "drain empty closed",
			setup: func() (context.Context, context.CancelFunc, chan string) {
				ch := make(chan string, 1)
				close(ch)
				return context.Background(), func() {}, ch
			},
			wantErr: func(err error) bool { return err == nil },
		},
		{
			name: "nil channel",
			setup: func() (context.Context, context.CancelFunc, chan string) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel, nil
			},
			wantErr: func(err error) bool { return errors.Is(err, context.Canceled) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel, ch := tt.setup()
			defer cancel()

			err := Drain(ctx, ch)

			if !tt.wantErr(err) {
				t.Errorf("Drain() error = %v, want satisfying condition", err)
			}
		})
	}
}
