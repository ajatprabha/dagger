package dagger

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testErrStep = errors.New("step error")

type testState struct{}

func alwaysTrue(_ testState) bool  { return true }
func alwaysFalse(_ testState) bool { return false }

func TestIf(t *testing.T) {
	stepRan := false
	step := NewStep(func(ctx context.Context, state testState) error {
		stepRan = true
		return nil
	})

	err := If(alwaysFalse, step).Exec(context.TODO(), testState{})
	assert.NoError(t, err)
	assert.False(t, stepRan)

	err = If(alwaysTrue, step).Exec(context.TODO(), testState{})
	assert.NoError(t, err)
	assert.True(t, stepRan)
}

func TestIfNot(t *testing.T) {
	stepRan := false
	step := NewStep(func(ctx context.Context, state testState) error {
		stepRan = true
		return nil
	})

	err := IfNot(alwaysTrue, step).Exec(context.TODO(), testState{})
	assert.NoError(t, err)
	assert.False(t, stepRan)

	err = IfNot(alwaysFalse, step).Exec(context.TODO(), testState{})
	assert.NoError(t, err)
	assert.True(t, stepRan)
}

func TestIfElse(t *testing.T) {
	count := 0
	is := NewStep(func(ctx context.Context, state testState) error {
		count++
		return nil
	})
	es := NewStep(func(ctx context.Context, state testState) error {
		count += 2
		return nil
	})

	err := IfElse(alwaysTrue, is, es).Exec(context.TODO(), testState{})
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	err = IfElse(alwaysFalse, is, es).Exec(context.TODO(), testState{})
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestResult(t *testing.T) {
	t.Run("SuccessBranch", func(t *testing.T) {
		success, failure := 0, 0

		ss := NewStep(func(ctx context.Context, state testState) error { success++; return nil })
		fs := NewStep(func(ctx context.Context, state testState) error { failure++; return nil })
		ms := NewStep(func(ctx context.Context, state testState) error { return nil })

		err := Result(ms, ss, func(ctx context.Context, state testState, err error) Step[testState] {
			return fs
		}).Exec(context.TODO(), testState{})
		assert.NoError(t, err)
		assert.Equal(t, 1, success)
		assert.Equal(t, 0, failure)
	})

	t.Run("FailureBranch", func(t *testing.T) {
		success, failure := 0, 0

		ss := NewStep(func(ctx context.Context, state testState) error { success++; return nil })
		fs := NewStep(func(ctx context.Context, state testState) error { failure++; return nil })
		ms := NewStep(func(ctx context.Context, state testState) error { return testErrStep })

		err := Result(ms, ss, func(ctx context.Context, state testState, err error) Step[testState] {
			return fs
		}).Exec(context.TODO(), testState{})
		assert.NoError(t, err)
		assert.Equal(t, 0, success)
		assert.Equal(t, 1, failure)
	})
}

func TestSeries(t *testing.T) {
	appendStepIn := func(res *[]string) func(string) Step[testState] {
		return func(name string) Step[testState] {
			return NewStep(func(ctx context.Context, _ testState) error {
				*res = append(*res, name)
				return nil
			})
		}
	}

	t.Run("Success", func(t *testing.T) {
		var res []string
		appendStep := appendStepIn(&res)

		err := Series(
			appendStep("s1"),
			appendStep("s2"),
			appendStep("s3"),
		).Exec(context.TODO(), testState{})
		assert.NoError(t, err)
		assert.Equal(t, []string{"s1", "s2", "s3"}, res)
	})

	t.Run("OneStepErrorsOut", func(t *testing.T) {
		var res []string
		appendStep := appendStepIn(&res)

		err := Series(
			appendStep("s1"),
			NewStep(func(ctx context.Context, state testState) error {
				return testErrStep
			}),
			appendStep("s3"),
		).Exec(context.TODO(), testState{})
		assert.ErrorIs(t, err, testErrStep)
		assert.Equal(t, []string{"s1"}, res)
	})
}

func TestContinue(t *testing.T) {
	appendStepIn := func(res *[]string) func(string) Step[testState] {
		return func(name string) Step[testState] {
			return NewStep(func(ctx context.Context, _ testState) error {
				*res = append(*res, name)
				return nil
			})
		}
	}

	t.Run("Success", func(t *testing.T) {
		var res []string
		appendStep := appendStepIn(&res)

		err := Continue(
			appendStep("s1"),
			appendStep("s2"),
			appendStep("s3"),
		).Exec(context.TODO(), testState{})
		assert.NoError(t, err)
		assert.Equal(t, []string{"s1", "s2", "s3"}, res)
	})

	t.Run("Failure", func(t *testing.T) {
		var res []string
		appendStep := appendStepIn(&res)
		notFoundStep := errors.New("not found")

		err := Continue(
			appendStep("s1"),
			NewStep(func(ctx context.Context, state testState) error {
				return testErrStep
			}),
			appendStep("s3"),
			NewStep(func(ctx context.Context, state testState) error {
				return notFoundStep
			}),
		).Exec(context.TODO(), testState{})

		assert.Error(t, err)
		assert.ErrorIs(t, err, testErrStep)
		assert.ErrorIs(t, err, notFoundStep)
		assert.Equal(t, []string{"s1", "s3"}, res)
	})
}

func Test_canSkip(t *testing.T) {
	testcases := []struct {
		name string
		step Step[testState]
	}{
		{
			name: "If",
			step: If(alwaysTrue, NewStep(func(context.Context, testState) error { return nil })),
		},
		{
			name: "IfNot",
			step: IfNot(alwaysTrue, NewStep(func(context.Context, testState) error { return nil })),
		},
		{
			name: "IfElse",
			step: IfElse(alwaysTrue,
				NewStep(func(context.Context, testState) error { return nil }),
				NewStep(func(context.Context, testState) error { return nil }),
			),
		},
		{
			name: "Result",
			step: Result(
				NewStep(func(context.Context, testState) error { return nil }),
				NewStep(func(context.Context, testState) error { return nil }),
				func(context.Context, testState, error) Step[testState] {
					return NewStep(func(context.Context, testState) error { return nil })
				},
			),
		},
		{
			name: "Series",
			step: Series(
				NewStep(func(context.Context, testState) error { return nil }),
				NewStep(func(context.Context, testState) error { return nil }),
			),
		},
		{
			name: "Continue",
			step: Continue(
				NewStep(func(context.Context, testState) error { return nil }),
				NewStep(func(context.Context, testState) error { return nil }),
			),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			f, canSkip := tc.step.(middlewareSkipper)
			assert.True(t, canSkip)
			f.canSkip()
		})
	}
}
