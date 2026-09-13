package dagger

import (
	"context"
	"errors"
	"fmt"
)

// Continue Step executes the given steps one-by-one in sequence.
// It executes all steps, accumulates all errors encountered and returns
// them using `errors.Join()`. This step is particularly helpful when we want
// to run certain steps in an order, but not stop execution if any step returns
// an error.
func Continue[S any](steps ...Step[S]) Step[S] {
	return &continueStep[S]{steps: steps}
}

type continueStep[S any] struct{ steps []Step[S] }

var _ middlewareSkipper = (*continueStep[any])(nil)

func (s *continueStep[S]) CanSkipMiddleware() bool { return true }

func (s *continueStep[S]) Exec(ctx context.Context, state S) error {
	var err error

	for _, step := range s.steps {
		if stepErr := execWithContext(ctx, step, state); stepErr != nil {
			err = errors.Join(err, fmt.Errorf("error executing step %s: %w", StepName(step), stepErr))
		}
	}

	return err
}

func (s *continueStep[S]) Unwrap() []Step[S] { return s.steps }
