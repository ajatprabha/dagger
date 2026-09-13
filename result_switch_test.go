package dagger

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwitchHandler(t *testing.T) {
	ctx := context.Background()
	step1Ran := false
	step2Ran := false
	step1 := NewStep(func(_ context.Context, _ struct{}) error {
		step1Ran = true
		return nil
	})
	step2 := NewStep(func(_ context.Context, _ struct{}) error {
		step2Ran = true
		return nil
	})

	errTarget := errors.New("target error")

	handler := &switchHandler[struct{}]{
		cases: []SwitchCase[struct{}]{
			Case(func(_ context.Context, err error) bool { return errors.Is(err, errTarget) }, step1),
			DefaultCase[struct{}](step2),
		},
	}

	t.Run("matches specific case", func(t *testing.T) {
		step1Ran = false
		selected := handler.selectStep(ctx, errTarget)
		assert.NotNil(t, selected)
		assert.NoError(t, selected.Exec(ctx, struct{}{}))
		assert.True(t, step1Ran)
	})

	t.Run("falls back to default case", func(t *testing.T) {
		step2Ran = false
		selected := handler.selectStep(ctx, errors.New("other error"))
		assert.NotNil(t, selected)
		assert.NoError(t, selected.Exec(ctx, struct{}{}))
		assert.True(t, step2Ran)
	})

	t.Run("no match returns nil when no default case", func(t *testing.T) {
		noDefaultHandler := &switchHandler[struct{}]{
			cases: []SwitchCase[struct{}]{
				Case(func(_ context.Context, err error) bool { return errors.Is(err, errTarget) }, step1),
			},
		}
		selected := noDefaultHandler.selectStep(ctx, errors.New("other error"))
		assert.Nil(t, selected)
	})

	t.Run("unwraps all case steps", func(t *testing.T) {
		steps := handler.Unwrap()
		assert.Len(t, steps, 2)
	})
}

func TestResult_Switch(t *testing.T) {
	t.Run("Switch with Case and Default", func(t *testing.T) {
		networkErrorCalled, validationErrorCalled, defaultErrorCalled := 0, 0, 0

		networkErr := errors.New("network error")
		validationErr := errors.New("validation error")

		networkErrorStep := NewStep(func(ctx context.Context, state struct{}) error {
			networkErrorCalled++
			return nil
		})
		validationErrorStep := NewStep(func(ctx context.Context, state struct{}) error {
			validationErrorCalled++
			return nil
		})
		defaultErrorStep := NewStep(func(ctx context.Context, state struct{}) error {
			defaultErrorCalled++
			return nil
		})

		isNetworkError := func(ctx context.Context, err error) bool {
			return err.Error() == "network error"
		}
		isValidationError := func(ctx context.Context, err error) bool {
			return err.Error() == "validation error"
		}

		// Test network error case
		mainStepNetworkErr := NewStep(func(ctx context.Context, state struct{}) error { return networkErr })
		err := Result(mainStepNetworkErr, Switch[struct{}](
			Case(isNetworkError, networkErrorStep),
			Case(isValidationError, validationErrorStep),
			DefaultCase(defaultErrorStep),
		)).Exec(context.TODO(), struct{}{})

		assert.NoError(t, err)
		assert.Equal(t, 1, networkErrorCalled)
		assert.Equal(t, 0, validationErrorCalled)
		assert.Equal(t, 0, defaultErrorCalled)

		// Test validation error case
		mainStepValidationErr := NewStep(func(ctx context.Context, state struct{}) error { return validationErr })
		err = Result(mainStepValidationErr, Switch[struct{}](
			Case(isNetworkError, networkErrorStep),
			Case(isValidationError, validationErrorStep),
			DefaultCase(defaultErrorStep),
		)).Exec(context.TODO(), struct{}{})

		assert.NoError(t, err)
		assert.Equal(t, 1, networkErrorCalled)
		assert.Equal(t, 1, validationErrorCalled)
		assert.Equal(t, 0, defaultErrorCalled)

		// Test default case
		mainStepUnknownErr := NewStep(func(ctx context.Context, state struct{}) error {
			return errors.New("unknown error")
		})
		err = Result(mainStepUnknownErr, Switch[struct{}](
			Case(isNetworkError, networkErrorStep),
			Case(isValidationError, validationErrorStep),
			DefaultCase(defaultErrorStep),
		)).Exec(context.TODO(), struct{}{})

		assert.NoError(t, err)
		assert.Equal(t, 1, networkErrorCalled)
		assert.Equal(t, 1, validationErrorCalled)
		assert.Equal(t, 1, defaultErrorCalled)
	})

	t.Run("OnError with simple step", func(t *testing.T) {
		errorHandlerCalled := 0
		errorStep := NewStep(func(ctx context.Context, state struct{}) error {
			errorHandlerCalled++
			return nil
		})

		mainStepWithError := NewStep(func(ctx context.Context, state struct{}) error {
			return errors.New("simple error")
		})

		err := Result(mainStepWithError, OnError(errorStep)).Exec(context.TODO(), struct{}{})

		assert.NoError(t, err)
		assert.Equal(t, 1, errorHandlerCalled)
	})
}
