package gopts

import (
	"testing"

	"github.com/aaron70/goaty/errors"
	"github.com/stretchr/testify/require"
)

type genericConfig struct {
	Values []any
}

type genericOption func(*genericConfig) error

func withValue(v any) genericOption {
	return func(gc *genericConfig) error {
		gc.Values = append(gc.Values, v)
		return nil
	}
}

func withError(err error) genericOption {
	return func(gc *genericConfig) error {
		gc.Values = append(gc.Values, err)
		return err
	}
}

func TestApply(t *testing.T) {
	testCases := []struct {
		Name     string
		Options  []genericOption
		Config   genericConfig
		Expected genericConfig
		Err      error
	}{
		{
			Name:     "One option is applied",
			Config:   genericConfig{},
			Expected: genericConfig{Values: []any{"First Value"}},
			Options:  []genericOption{withValue("First Value")},
		},
		{
			Name:     "Multiple option are applied",
			Config:   genericConfig{},
			Expected: genericConfig{Values: []any{"First Value", "Second Value", "Third Value"}},
			Options:  []genericOption{withValue("First Value"), withValue("Second Value"), withValue("Third Value")},
		},
		{
			Name:     "One option returns an error",
			Config:   genericConfig{},
			Expected: genericConfig{Values: []any{"First Value" }},
			Options:  []genericOption{withValue("First Value"), withError(gerrs.New("Some Error")), withValue("Second Value"), withValue("Third Value")},
			Err: gerrs.NewError(gerrs.ErrInvalidArgument, gerrs.New("Some Error"), ""),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			err := Apply(&tt.Config, tt.Options...)
			if tt.Err != nil {
				require.ErrorIs(t, err, gerrs.ErrInvalidArgument)
				require.EqualErrorf(t, err, tt.Err.Error(), "The returned error is not equal to the expected error: %s", tt.Err)
			} else {
				require.Equal(t, tt.Expected, tt.Config, "The config must be equal to the expected config")
				require.NoError(t, err, "No error is expected, but got: %s", err)
			}
		})
	}
}

func TestApplyEmpty(t *testing.T) {
	testCases := []struct {
		Name     string
		Options  []genericOption
		Expected genericConfig
		Err      error
	}{
		{
			Name:     "One option is applied",
			Expected: genericConfig{Values: []any{"First Value"}},
			Options:  []genericOption{withValue("First Value")},
		},
		{
			Name:     "Multiple option are applied",
			Expected: genericConfig{Values: []any{"First Value", "Second Value", "Third Value"}},
			Options:  []genericOption{withValue("First Value"), withValue("Second Value"), withValue("Third Value")},
		},
		{
			Name:     "One option returns an error",
			Expected: genericConfig{Values: []any{"First Value" }},
			Options:  []genericOption{withValue("First Value"), withError(gerrs.New("Some Error")), withValue("Second Value"), withValue("Third Value")},
			Err: gerrs.NewError(gerrs.ErrInvalidArgument, gerrs.New("Some Error"), ""),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			config, err := ApplyZero(tt.Options...)
			if tt.Err != nil {
				require.ErrorIs(t, err, gerrs.ErrInvalidArgument)
				require.EqualErrorf(t, err, tt.Err.Error(), "The returned error is not equal to the expected error: %s", tt.Err)
			} else {
				require.Equal(t, tt.Expected, config, "The config must be equal to the expected config")
				require.NoError(t, err, "No error is expected, but got: %s", err)
			}
		})
	}
}
