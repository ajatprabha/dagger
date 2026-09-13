package dagger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIfElse(t *testing.T) {
	count := 0
	is := NewStep(func(ctx context.Context, state struct{}) error {
		count++
		return nil
	})
	es := NewStep(func(ctx context.Context, state struct{}) error {
		count += 2
		return nil
	})

	err := IfElse(alwaysTrue, is, es).Exec(context.TODO(), struct{}{})
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	err = IfElse(alwaysFalse, is, es).Exec(context.TODO(), struct{}{})
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}
