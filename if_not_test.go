package dagger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIfNot(t *testing.T) {
	stepRan := false
	step := NewStep(func(ctx context.Context, state struct{}) error {
		stepRan = true
		return nil
	})

	err := IfNot(alwaysTrue, step).Exec(context.TODO(), struct{}{})
	assert.NoError(t, err)
	assert.False(t, stepRan)

	err = IfNot(alwaysFalse, step).Exec(context.TODO(), struct{}{})
	assert.NoError(t, err)
	assert.True(t, stepRan)
}
