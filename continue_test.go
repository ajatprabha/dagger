package dagger

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContinue(t *testing.T) {
	appendStepIn := func(res *[]string) func(string) Step[struct{}] {
		return func(name string) Step[struct{}] {
			return NewStep(func(ctx context.Context, _ struct{}) error {
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
		).Exec(context.TODO(), struct{}{})
		assert.NoError(t, err)
		assert.Equal(t, []string{"s1", "s2", "s3"}, res)
	})

	t.Run("Failure", func(t *testing.T) {
		var res []string
		appendStep := appendStepIn(&res)
		testErrStep := errors.New("step error")
		notFoundStep := errors.New("not found")

		err := Continue(
			appendStep("s1"),
			NewStep(func(ctx context.Context, state struct{}) error {
				return testErrStep
			}),
			appendStep("s3"),
			NewStep(func(ctx context.Context, state struct{}) error {
				return notFoundStep
			}),
		).Exec(context.TODO(), struct{}{})

		assert.Error(t, err)
		assert.ErrorIs(t, err, testErrStep)
		assert.ErrorIs(t, err, notFoundStep)
		assert.Equal(t, []string{"s1", "s3"}, res)
	})
}
