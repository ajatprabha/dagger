package dagger

import (
	"context"
)

// IfElse takes in a Selector and
//   - executes the thenStep, if the Selector returns true
//   - executes the elseStep, if the Selector returns false
func IfElse[S any](condition Selector[S], thenStep, elseStep Step[S]) Step[S] {
	return &ifElseStep[S]{condition: condition, thenStep: thenStep, elseStep: elseStep}
}

func (s *ifElseStep[S]) Exec(ctx context.Context, state S) error {
	if s.condition(state) {
		return execWithContext(ctx, s.thenStep, state)
	}

	return execWithContext(ctx, s.elseStep, state)
}

func (s *ifElseStep[S]) Unwrap() []Step[S] { return []Step[S]{s.thenStep, s.elseStep} }

type ifElseStep[S any] struct {
	condition Selector[S]
	thenStep  Step[S]
	elseStep  Step[S]
}

var _ middlewareSkipper = (*ifElseStep[any])(nil)

func (s *ifElseStep[S]) CanSkipMiddleware() bool { return true }
