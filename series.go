package dagger

import (
	"context"
)

// Series Step executes the given steps one-by-one in sequence,
// if any Step returns an error, Series also returns that same
// error and skips the remaining Step(s).
func Series[S any](steps ...Step[S]) Step[S] {
	return &seriesStep[S]{steps: steps}
}

func (s *seriesStep[S]) Exec(ctx context.Context, state S) error {
	for _, step := range s.steps {
		if err := execWithContext(ctx, step, state); err != nil {
			return err
		}
	}

	return nil
}

func (s *seriesStep[S]) Unwrap() []Step[S] { return s.steps }

type seriesStep[S any] struct{ steps []Step[S] }

var _ middlewareSkipper = (*seriesStep[any])(nil)

func (s *seriesStep[S]) CanSkipMiddleware() bool { return true }
