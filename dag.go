// Package dagger helps in working with DAGs by providing constructs
// that help in implementing them. It also provides utilities
// related to DAGs.
package dagger

import (
	"context"
	"fmt"
)

// Executor is the main struct that holds the DAG and the middlewares.
type Executor[S any] struct {
	start       Step[S]
	middlewares MiddlewareChain[S]
}

// New validates a Step and makes sure it does have any cycles.
func New[S any](startStep Step[S]) (*Executor[S], error) {
	err := checkDAGCycles(startStep)
	if err != nil {
		return nil, &ErrInvalid{err: err}
	}

	return &Executor[S]{
		start:       startStep,
		middlewares: make(MiddlewareChain[S], 0),
	}, nil
}

// Use adds the given MiddlewareFunc(s) to the Executor.
func (e *Executor[S]) Use(mwf ...MiddlewareFunc[S]) {
	for _, m := range mwf {
		e.middlewares = append(e.middlewares, m)
	}
}

func (e *Executor[S]) Exec(ctx context.Context, state S) error {
	s := e.middlewares.apply(e.start, stepInfo(e.start))

	return s.Exec(withMiddlewares(ctx, e.middlewares), state)
}

type ctxKey int

const (
	middlewareKey ctxKey = iota
)

func withMiddlewares[S any](ctx context.Context, chain MiddlewareChain[S]) context.Context {
	return context.WithValue(ctx, middlewareKey, chain)
}

// execWithContext runs the given stage with MiddlewareChain in context.
// Meta Step(s) must use this function to call Step.Exec.
func execWithContext[S any](ctx context.Context, step Step[S], state S) error {
	s := step

	c, ok := ctx.Value(middlewareKey).(MiddlewareChain[S])
	if ok {
		s = c.apply(step, stepInfo(s))
	}

	return s.Exec(ctx, state)
}

// checkDAGCycles takes a step and checks for cycles.
// It errors out if it encounters a cycle.
func checkDAGCycles[S any](step Step[S]) error {
	visited := make(map[string]struct{})
	return checkDAGRecursive(step, visited)
}

func checkDAGRecursive[S any](step Step[S], visited map[string]struct{}) error {
	name := StepName(step)
	ptr := fmt.Sprintf("%p", step)

	if _, found := visited[ptr]; found {
		return &ErrCycle{stepName: name}
	}

	visited[ptr] = struct{}{}

	// TODO: Handle stepWithErr
	switch s := step.(type) {
	case interface{ Unwrap() Step[S] }:
		return checkDAGRecursive(s.Unwrap(), visited)
	case interface{ Unwrap() []Step[S] }:
		for _, childStep := range s.Unwrap() {
			if err := checkDAGRecursive(childStep, visited); err != nil {
				return err
			}
		}
	}

	delete(visited, ptr)
	return nil
}
