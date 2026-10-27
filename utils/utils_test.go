package utils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMust_NoError(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		require.Equal(t, "value", Must("value", nil))
	})

	t.Run("int", func(t *testing.T) {
		require.Equal(t, 42, Must(42, nil))
	})

	t.Run("bool", func(t *testing.T) {
		require.True(t, Must(true, nil))
	})

	t.Run("slice", func(t *testing.T) {
		require.Equal(t, []string{"a", "b"}, Must([]string{"a", "b"}, nil))
	})

	t.Run("pointer", func(t *testing.T) {
		v := &struct{ Name string }{Name: "goaty"}
		require.Same(t, v, Must(v, nil))
	})
}

func TestMust_ZeroValueWithNilError(t *testing.T) {
	require.Equal(t, "", Must("", nil))
	require.Equal(t, 0, Must(0, nil))
	require.False(t, Must(false, nil))
}

func TestMust_ErrorPanics(t *testing.T) {
	err := errors.New("some error")

	require.PanicsWithValue(t, err, func() {
		Must("value", err)
	})
}
