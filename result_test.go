package dagger

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResult(t *testing.T) {
	t.Run("NilMainStepPanics", func(t *testing.T) {
		assert.Panics(t, func() { Result[struct{}](nil) })
	})

	t.Run("at least one branch required", func(t *testing.T) {
		assert.Panics(t, func() { Result(NewStep(func(_ context.Context, _ struct{}) error { return nil })) })
	})

	t.Run("SuccessBranch", func(t *testing.T) {
		success, failure := 0, 0

		ms := NewStep(func(ctx context.Context, state struct{}) error { return nil })
		ss := NewStep(func(ctx context.Context, state struct{}) error { success++; return nil })
		fs := NewStep(func(ctx context.Context, state struct{}) error { failure++; return nil })

		err := Result(ms, OnSuccess(ss), OnError(fs)).Exec(context.TODO(), struct{}{})
		assert.NoError(t, err)
		assert.Equal(t, 1, success)
		assert.Equal(t, 0, failure)
	})

	t.Run("FailureSingleBranch", func(t *testing.T) {
		success, failure := 0, 0

		ms := NewStep(func(ctx context.Context, state struct{}) error { return errors.New("failure") })
		ss := NewStep(func(ctx context.Context, state struct{}) error { success++; return nil })
		fs := NewStep(func(ctx context.Context, state struct{}) error { failure++; return nil })

		err := Result(ms, OnSuccess(ss), OnError(fs)).Exec(context.TODO(), struct{}{})
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
				branch1Selected++
				return true
			}
			return false
		}
		branch2Selector := func(ctx context.Context, err error) bool {
			if errors.Is(err, err2) {
				branch2Selected++
				return true
			}
			return false
		}

		ss := NewStep(func(ctx context.Context, state struct{}) error { success++; return nil })
		fs := NewStep(func(ctx context.Context, state struct{}) error { failure++; return nil })
		ds := NewStep(func(ctx context.Context, state struct{}) error { defSelected++; return nil })

		t.Run("DefaultBranch", func(t *testing.T) {
			ms := NewStep(func(ctx context.Context, state struct{}) error { return errors.New("error random") })

			err := Result(ms, OnSuccess(ss), Switch[struct{}](
				Case(branch1Selector, fs),
				Case(branch2Selector, fs),
				DefaultCase(ds),
			)).Exec(context.TODO(), struct{}{})

			assert.NoError(t, err)
			assert.Equal(t, 0, success)
			assert.Equal(t, 0, failure)
			assert.Equal(t, 0, branch1Selected)
			assert.Equal(t, 0, branch2Selected)
			assert.Equal(t, 1, defSelected)
		})

		t.Run("Branch1", func(t *testing.T) {
			ms := NewStep(func(ctx context.Context, state struct{}) error { return err1 })

			err := Result(ms, OnSuccess(ss), Switch[struct{}](
				Case(branch1Selector, fs),
				Case(branch2Selector, fs),
				DefaultCase(ds),
			)).Exec(context.TODO(), struct{}{})

			assert.NoError(t, err)
			assert.Equal(t, 0, success)
			assert.Equal(t, 1, failure)
			assert.Equal(t, 1, branch1Selected)
			assert.Equal(t, 0, branch2Selected)
			assert.Equal(t, 1, defSelected)
		})

		t.Run("Branch2", func(t *testing.T) {
			ms := NewStep(func(ctx context.Context, state struct{}) error { return err2 })

			err := Result(ms, OnSuccess(ss), Switch[struct{}](
				Case(branch1Selector, fs),
				Case(branch2Selector, fs),
				DefaultCase(ds),
			)).Exec(context.TODO(), struct{}{})

			assert.NoError(t, err)
			assert.Equal(t, 0, success)
			assert.Equal(t, 2, failure)
			assert.Equal(t, 1, branch1Selected)
			assert.Equal(t, 1, branch2Selected)
			assert.Equal(t, 1, defSelected)
		})

		t.Run("NoBranchMatch", func(t *testing.T) {
			ms := NewStep(func(ctx context.Context, state struct{}) error { return errors.New("random") })

			err := Result(ms, OnSuccess(ss), Switch[struct{}](
				Case(func(ctx context.Context, err error) bool { return false }, fs), // no match
			)).Exec(context.TODO(), struct{}{})

			assert.Error(t, err)
			assert.Equal(t, 0, success)
			assert.Equal(t, 2, failure)
			assert.Equal(t, 1, branch1Selected)
			assert.Equal(t, 1, branch2Selected)
			assert.Equal(t, 1, defSelected)
		})
	})
}
