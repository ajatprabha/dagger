package dagger

import (
	"context"
)

// If Step takes in a Selector and runs the thenStep, iff Selector returns true.
func If[S any](condition Selector[S], thenStep Step[S]) Step[S] {
	return &ifStep[S]{condition: condition, thenStep: thenStep}
}

type ifStep[S any] struct {
	condition Selector[S]
	thenStep  Step[S]
}

var _ middlewareSkipper = (*ifStep[any])(nil)

func (s *ifStep[S]) CanSkipMiddleware() bool { return true }

func (s *ifStep[S]) Exec(ctx context.Context, state S) error {
	if s.condition(state) {
		return execWithContext(ctx, s.thenStep, state)
	}

	return nil
}

func (s *ifStep[S]) Unwrap() Step[S] { return s.thenStep }
