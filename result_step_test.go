package dagger

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResult(t *testing.T) {
	t.Run("SuccessBranch", func(t *testing.T) {
		success, failure := 0, 0

		ss := NewStep(func(ctx context.Context, state testState) error { success++; return nil })
		fs := NewStep(func(ctx context.Context, state testState) error { failure++; return nil })
		ms := NewStep(func(ctx context.Context, state testState) error { return nil })

		err := Result(ms, ss, fs).Exec(context.TODO(), testState{})
		assert.NoError(t, err)
		assert.Equal(t, 1, success)
		assert.Equal(t, 0, failure)
	})

	t.Run("FailureSingleBranch", func(t *testing.T) {
		success, failure := 0, 0

		ss := NewStep(func(ctx context.Context, state testState) error { success++; return nil })
		fs := NewStep(func(ctx context.Context, state testState) error { failure++; return nil })
		ms := NewStep(func(ctx context.Context, state testState) error { return testErrStep })

		err := Result(ms, ss, fs).Exec(context.TODO(), testState{})
		assert.NoError(t, err)
		assert.Equal(t, 0, success)
		assert.Equal(t, 1, failure)
	})

	t.Run("FailureMultipleBranch", func(t *testing.T) {
		success, failure := 0, 0

		err1 := errors.New("error 1")
		err2 := errors.New("error 2")

		branch1Selected, branch2Selected, defSelected := 0, 0, 0
		branch1Selector := func(ctx context.Context, err error) bool {
			if errors.Is(err, err1) {
				branch1Selected += 1
				return true
			}
			return false
		}
		branch2Selector := func(ctx context.Context, err error) bool {
			if errors.Is(err, err2) {
				branch2Selected += 1
				return true
			}
			return false
		}

		ss := NewStep(func(ctx context.Context, state testState) error { success++; return nil })
		fs := NewStep(func(ctx context.Context, state testState) error { failure++; return nil })
		ds := NewStep(func(ctx context.Context, state testState) error { defSelected += 1; return nil })

		t.Run("DefaultBranch", func(t *testing.T) {
			ms := NewStep(func(ctx context.Context, state testState) error { return errors.New("error random") })

			mfh := HandleMultiFailure(
				NewBranch(branch1Selector, fs),
				NewBranch(branch2Selector, fs),
				DefaultBranch(ds),
			)

			err := Result(ms, ss, mfh).Exec(context.TODO(), testState{})

			assert.NoError(t, err)
			assert.Equal(t, 0, success)
			assert.Equal(t, 0, failure)
			assert.Equal(t, 0, branch1Selected)
			assert.Equal(t, 0, branch2Selected)
			assert.Equal(t, 1, defSelected)
		})

		t.Run("Branch1", func(t *testing.T) {
			ms := NewStep(func(ctx context.Context, state testState) error { return err1 })

			mfh := HandleMultiFailure(
				NewBranch(branch1Selector, fs),
				NewBranch(branch2Selector, fs),
				DefaultBranch(ds),
			)

			err := Result(ms, ss, mfh).Exec(context.TODO(), testState{})

			assert.NoError(t, err)
			assert.Equal(t, 0, success)
			assert.Equal(t, 1, failure)
			assert.Equal(t, 1, branch1Selected)
			assert.Equal(t, 0, branch2Selected)
			assert.Equal(t, 1, defSelected)
		})

		t.Run("Branch2", func(t *testing.T) {
			ms := NewStep(func(ctx context.Context, state testState) error { return err2 })

			mfh := HandleMultiFailure(
				NewBranch(branch1Selector, fs),
				NewBranch(branch2Selector, fs),
				DefaultBranch(ds),
			)

			err := Result(ms, ss, mfh).Exec(context.TODO(), testState{})

			assert.NoError(t, err)
			assert.Equal(t, 0, success)
			assert.Equal(t, 2, failure)
			assert.Equal(t, 1, branch1Selected)
			assert.Equal(t, 1, branch2Selected)
			assert.Equal(t, 1, defSelected)
		})

		t.Run("NoBranchMatch", func(t *testing.T) {
			ms := NewStep(func(ctx context.Context, state testState) error { return errors.New("random") })

			mfh := HandleMultiFailure(
				NewBranch(
					func(ctx context.Context, err error) bool { return false }, // no match
					fs,
				),
			)

			err := Result(ms, ss, mfh).Exec(context.TODO(), testState{})

			assert.Error(t, err)
			assert.Equal(t, 0, success)
			assert.Equal(t, 2, failure)
			assert.Equal(t, 1, branch1Selected)
			assert.Equal(t, 1, branch2Selected)
			assert.Equal(t, 1, defSelected)
		})
	})
}

func TestStepWithErr_Exec(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		type useState struct{ indent int }

		errCount := 0

		ss := NewResultErrStep(func(ctx context.Context, state useState, err error) error {
			if err != nil {
				errCount++
			}

			return err
		})
		ms := NewStep(func(ctx context.Context, state useState) error { return errors.New("random") })
		mfh := HandleMultiFailure(
			NewBranch[useState](
				func(ctx context.Context, err error) bool { return true }, // no match
				ss,
			),
		)

		step := Result(
			ms,
			ss,
			mfh,
		)

		dag, err := New(step)
		assert.NoError(t, err)

		buf := new(bytes.Buffer)
		dag.Use(func(next Step[useState], info Info) Step[useState] {
			return NewStep(func(ctx context.Context, state useState) error {
				if info.CanSkip {
					return next.Exec(ctx, useState{indent: state.indent + 1})
				}

				buf.WriteString(strings.Repeat("\t", state.indent-1))
				buf.WriteString(info.Name.String())
				buf.WriteString("\n")

				return next.Exec(ctx, useState{indent: state.indent + 1})
			})
		})

		err = dag.Exec(context.TODO(), useState{})
		assert.Error(t, err)
		assert.Equal(t, 1, errCount)
		assert.Equal(t, `dagger:TestStepWithErr_Exec.func1.2
dagger:stepWithErr[useState·2]
`, buf.String())
	})
}
