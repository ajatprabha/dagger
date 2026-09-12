package dagger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func alwaysTrue(_ struct{}) bool  { return true }
func alwaysFalse(_ struct{}) bool { return false }

func TestIf(t *testing.T) {
	stepRan := false
	step := NewStep(func(ctx context.Context, state struct{}) error {
		stepRan = true
		return nil
	})

	err := If(alwaysFalse, step).Exec(context.TODO(), struct{}{})
	assert.NoError(t, err)
	assert.False(t, stepRan)

	err = If(alwaysTrue, step).Exec(context.TODO(), struct{}{})
	assert.NoError(t, err)
	assert.True(t, stepRan)
}
