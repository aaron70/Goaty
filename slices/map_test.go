package gslices

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type point struct {
	X, Y int
}

func TestMap_TableDriven(t *testing.T) {
	double := func(i int) (int, error) { return i * 2, nil }

	tests := []struct {
		name  string
		input []int
		want  []int
	}{
		{"nil slice", nil, []int{}},
		{"empty slice", []int{}, []int{}},
		{"single element", []int{3}, []int{6}},
		{"multiple elements", []int{1, 2, 3, 4}, []int{2, 4, 6, 8}},
		{"negative numbers", []int{-1, -2, 0}, []int{-2, -4, 0}},
		{"zero values preserved", []int{0, 0, 0}, []int{0, 0, 0}},
		{"duplicates", []int{5, 5, 5}, []int{10, 10, 10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Map(tt.input, double)
			require.NoError(t, err)
			assert.NotNil(t, got, "successful Map must return a non-nil slice")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMap_EmptyInputReturnsNonNilEmptySlice(t *testing.T) {
	for _, in := range [][]int{nil, {}} {
		got, err := Map(in, func(i int) (int, error) { return i, nil })
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Len(t, got, 0)
	}
}

func TestMap_EmptyInputDoesNotCallFunc(t *testing.T) {
	calls := 0
	f := func(i int) (int, error) {
		calls++
		return i, nil
	}

	_, err := Map([]int(nil), f)
	require.NoError(t, err)
	_, err = Map([]int{}, f)
	require.NoError(t, err)

	assert.Equal(t, 0, calls)
}

func TestMap_NilFuncWithEmptyInputDoesNotPanic(t *testing.T) {
	var f func(int) (int, error)

	assert.NotPanics(t, func() {
		got, err := Map([]int{}, f)
		assert.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestMap_NilFuncWithNonEmptyInputPanics(t *testing.T) {
	var f func(int) (int, error)

	assert.Panics(t, func() {
		_, _ = Map([]int{1}, f)
	})
}

func TestMap_CallsFuncOncePerElementInOrder(t *testing.T) {
	input := []string{"a", "b", "c", "d"}
	var seen []string

	got, err := Map(input, func(s string) (string, error) {
		seen = append(seen, s)
		return s + s, nil
	})

	require.NoError(t, err)
	assert.Equal(t, input, seen, "f must be called once per element, in order")
	assert.Equal(t, []string{"aa", "bb", "cc", "dd"}, got)
}

func TestMap_ChangesElementType(t *testing.T) {
	got, err := Map([]int{1, 22, 333}, func(i int) (string, error) {
		return strconv.Itoa(i), nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"1", "22", "333"}, got)
}

func TestMap_StructElements(t *testing.T) {
	input := []point{{1, 2}, {3, 4}}
	got, err := Map(input, func(p point) (int, error) {
		return p.X + p.Y, nil
	})

	require.NoError(t, err)
	assert.Equal(t, []int{3, 7}, got)
}

func TestMap_FuncReceivesCopyOfElement(t *testing.T) {
	input := []point{{1, 2}}

	got, err := Map(input, func(p point) (point, error) {
		p.X = 100 // mutate the copy
		return p, nil
	})

	require.NoError(t, err)
	assert.Equal(t, point{1, 2}, input[0], "input must not be mutated through the copy")
	assert.Equal(t, point{100, 2}, got[0])
}

func TestMap_ResultIsIndependentOfInput(t *testing.T) {
	input := []int{1, 2, 3}

	got, err := Map(input, func(i int) (int, error) { return i, nil })
	require.NoError(t, err)

	got[0] = 999
	assert.Equal(t, []int{1, 2, 3}, input, "modifying result must not alias input")
}

func TestMap_InputIsNotMutated(t *testing.T) {
	input := []int{1, 2, 3}
	snapshot := append([]int(nil), input...)

	_, err := Map(input, func(i int) (int, error) { return i * 10, nil })
	require.NoError(t, err)

	assert.Equal(t, snapshot, input)
}

func TestMap_PointerElementsMutationVisible(t *testing.T) {
	a, b := &point{1, 1}, &point{2, 2}
	input := []*point{a, b}

	_, err := Map(input, func(p *point) (struct{}, error) {
		p.X = 0
		return struct{}{}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 0, a.X)
	assert.Equal(t, 0, b.X)
}

func TestMap_NilPointerElementsPassedThrough(t *testing.T) {
	input := []*point{nil, {1, 2}, nil}

	got, err := Map(input, func(p *point) (bool, error) {
		return p == nil, nil
	})

	require.NoError(t, err)
	assert.Equal(t, []bool{true, false, true}, got)
}

func TestMap_ErrorOnFirstElement(t *testing.T) {
	boom := errors.New("boom")
	calls := 0

	got, err := Map([]int{1, 2, 3}, func(i int) (int, error) {
		calls++
		return 0, boom
	})

	assert.ErrorIs(t, err, boom)
	assert.Nil(t, got, "result must be nil on error")
	assert.Equal(t, 1, calls, "must short-circuit after first error")
}

func TestMap_ErrorOnLastElement(t *testing.T) {
	boom := errors.New("boom")
	var seen []int

	got, err := Map([]int{1, 2, 3}, func(i int) (int, error) {
		seen = append(seen, i)
		if i == 3 {
			return 0, boom
		}
		return i, nil
	})

	assert.ErrorIs(t, err, boom)
	assert.Nil(t, got)
	assert.Equal(t, []int{1, 2, 3}, seen)
}

func TestMap_ErrorInMiddleStopsProcessing(t *testing.T) {
	boom := errors.New("boom")
	var seen []int

	got, err := Map([]int{1, 2, 3, 4, 5}, func(i int) (int, error) {
		seen = append(seen, i)
		if i == 3 {
			return 0, boom
		}
		return i, nil
	})

	assert.ErrorIs(t, err, boom)
	assert.Nil(t, got, "partial results must not leak on error")
	assert.Equal(t, []int{1, 2, 3}, seen, "elements after the failure must not be visited")
}

func TestMap_ReturnsFirstErrorWhenMultipleFail(t *testing.T) {
	first := errors.New("first")
	second := errors.New("second")

	_, err := Map([]int{1, 2, 3}, func(i int) (int, error) {
		if i == 2 {
			return 0, first
		}
		if i == 3 {
			return 0, second
		}
		return i, nil
	})

	assert.ErrorIs(t, err, first)
	assert.NotErrorIs(t, err, second)
}

func TestMap_ErrorIsReturnedUnwrapped(t *testing.T) {
	sentinel := errors.New("sentinel")
	wrapped := fmt.Errorf("wrap: %w", sentinel)

	_, err := Map([]int{1}, func(i int) (int, error) {
		return 0, wrapped
	})

	assert.Same(t, wrapped, err, "Map must return the exact error from f")
}

func TestMap_ErrorWithZeroValueResultStillReturnsNil(t *testing.T) {
	// A non-nil error with a zero-valued result must still yield a nil slice,
	// not a slice containing zero values.
	got, err := Map([]string{"x"}, func(s string) (string, error) {
		return "", errors.New("fail")
	})

	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestMap_NilErrorWithZeroValuesIsSuccess(t *testing.T) {
	got, err := Map([]int{1, 2}, func(i int) (int, error) {
		return 0, nil
	})

	require.NoError(t, err)
	assert.Equal(t, []int{0, 0}, got)
}

func TestMap_PanicInFuncPropagates(t *testing.T) {
	assert.PanicsWithValue(t, "kaboom", func() {
		_, _ = Map([]int{1, 2}, func(i int) (int, error) {
			if i == 2 {
				panic("kaboom")
			}
			return i, nil
		})
	})
}

func TestMap_LargeInput(t *testing.T) {
	const n = 100_000
	input := make([]int, n)
	for i := range input {
		input[i] = i
	}

	got, err := Map(input, func(i int) (int, error) { return i + 1, nil })

	require.NoError(t, err)
	require.Len(t, got, n)
	assert.Equal(t, 1, got[0])
	assert.Equal(t, n, got[n-1])
}
