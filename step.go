package dagger

import (
	"context"
	"errors"
	"fmt"
)

// Step is a unit of work to be performed in the DAG.
// It can either be a part of a vertex, or an operation
// to reach to another Vertex, making the Step an edge.
type Step[S any] interface {
	// Exec is responsible to perform the actual operation.
	Exec(ctx context.Context, state S) error
}

// StepFunc helps implement Step in place.
type StepFunc[S any] func(ctx context.Context, state S) error

func (f StepFunc[S]) Exec(ctx context.Context, state S) error { return f(ctx, state) }

var _ Step[any] = (*StepFunc[any])(nil)

// Selector is used to define closures that act as
// branch selector for Step(s).
type Selector[S any] func(state S) bool

type ifStep[S any] struct {
	condition Selector[S]
	thenStep  Step[S]
}

var _ middlewareSkipper = (*ifStep[any])(nil)

func (s *ifStep[S]) canSkip() bool {
	return true
}

func (s *ifStep[S]) Exec(ctx context.Context, state S) error {
	if s.condition(state) {
		return execWithContext(ctx, s.thenStep, state)
	}

	return nil
}

func (s *ifStep[S]) Unwrap() Step[S] { return s.thenStep }

// If Step takes in a Selector and runs the thenStep, iff Selector returns true.
func If[S any](condition Selector[S], thenStep Step[S]) Step[S] {
	return &ifStep[S]{condition: condition, thenStep: thenStep}
}

// IfNot Step takes in a Selector and runs the thenStep, iff Selector returns false.
func IfNot[S any](condition Selector[S], thenStep Step[S]) Step[S] {
	return &ifStep[S]{condition: func(state S) bool { return !condition(state) }, thenStep: thenStep}
}

type ifElseStep[S any] struct {
	condition Selector[S]
	thenStep  Step[S]
	elseStep  Step[S]
}

var _ middlewareSkipper = (*ifElseStep[any])(nil)

func (s *ifElseStep[S]) canSkip() bool {
	return true
}

func (s *ifElseStep[S]) Exec(ctx context.Context, state S) error {
	if s.condition(state) {
		return execWithContext(ctx, s.thenStep, state)
	}

	return execWithContext(ctx, s.elseStep, state)
}

func (s *ifElseStep[S]) Unwrap() []Step[S] { return []Step[S]{s.thenStep, s.elseStep} }

// IfElse takes in a Selector and
//   - executes the thenStep, if the Selector returns true
//   - executes the elseStep, if the Selector returns false
func IfElse[S any](condition Selector[S], thenStep, elseStep Step[S]) Step[S] {
	return &ifElseStep[S]{condition: condition, thenStep: thenStep, elseStep: elseStep}
}

type seriesStep[S any] struct {
	steps []Step[S]
}

var _ middlewareSkipper = (*seriesStep[any])(nil)

func (s *seriesStep[S]) canSkip() bool {
	return true
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

// Series Step executes the given steps one-by-one in sequence,
// if any Step returns an error, Series also returns that same
// error and skips the remaining Step(s).
func Series[S any](steps ...Step[S]) Step[S] {
	return &seriesStep[S]{steps: steps}
}

type continueStep[S any] struct {
	steps []Step[S]
}

var _ middlewareSkipper = (*continueStep[any])(nil)

func (s *continueStep[S]) canSkip() bool {
	return true
}

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

// Continue Step executes the given steps one-by-one in sequence.
// It executes all steps, accumulates all errors encountered and returns
// them using `errors.Join()`.
// This step is particularly helpful when we want to run certain steps in an order,
// but not stop execution if any step returns an error.
func Continue[S any](steps ...Step[S]) Step[S] {
	return &continueStep[S]{steps: steps}
}

// NewStep is a helper function to create a StepFunc without explicit mention of generic S.
func NewStep[S any](f func(ctx context.Context, state S) error) StepFunc[S] { return f }
