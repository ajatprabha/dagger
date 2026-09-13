package dagger

import (
	"context"
)

// StepFunc helps implement Step in place.
type StepFunc[S any] func(ctx context.Context, state S) error

func (f StepFunc[S]) Exec(ctx context.Context, state S) error { return f(ctx, state) }

var _ Step[any] = (*StepFunc[any])(nil)

// NewStep is a helper function to create a StepFunc without explicit mention of generic S.
func NewStep[S any](f func(ctx context.Context, state S) error) StepFunc[S] { return f }
