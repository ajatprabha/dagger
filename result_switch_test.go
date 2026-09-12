package dagger

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResultSwitch(t *testing.T) {
	t.Run("switchCase implements SwitchCase", func(t *testing.T) {
		var c SwitchCase[any] = &switchCase[any]{}
		c.isSwitchCase()
	})

	t.Run("switchDefault implements SwitchCase", func(t *testing.T) {
		var d SwitchCase[any] = &switchDefault[any]{}
		d.isSwitchCase()
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
