package dagger

import (
	"context"
)

// OnError creates a ResultOption that sets a single failure step
func OnError[S any](step Step[S]) ResultOption[S] {
	return resultOptionFunc[S](func(cfg *resultConfig[S]) {
		cfg.failureHandler = &singleStepHandler[S]{step: step}
	})
}

// StepWithErr is used to define closures that act as Step with error.
type StepWithErr[S any] func(ctx context.Context, state S, resErr error) error

// NewResultErrStep creates a new Step that also has access to the error from
// the Result Step's main Step.
func NewResultErrStep[S any](f func(ctx context.Context, state S, resErr error) error) StepWithErr[S] {
	return f
}

func (f StepWithErr[S]) Exec(ctx context.Context, state S) error {
	var s Step[S] = StepFunc[S](f.exec)

	c, ok := ctx.Value(middlewareKey).(MiddlewareChain[S])
	if ok {
		si := stepInfo(s)
		si.CanSkip = true
		s = c.apply(s, si)
	}

	return s.Exec(ctx, state)
}

func (s *resultStep[S]) handleErr(ctx context.Context, state S, err error) error {
	if step := s.failureHandler.selectStep(ctx, err); step != nil {
		return execWithContext(resultErrToContext(ctx, err), step, state)
	}

	return err
}

func (f StepWithErr[S]) exec(ctx context.Context, state S) error {
	return f(ctx, state, resultErrFromContext(ctx))
}

// singleStepHandler wraps a single step to implement resultFailureHandler
type singleStepHandler[S any] struct{ step Step[S] }

func (s *singleStepHandler[S]) selectStep(_ context.Context, _ error) Step[S] { return s.step }

func (s *singleStepHandler[S]) Unwrap() Step[S] { return s.step }

type resultCtxKey int

const resultErrKey resultCtxKey = iota

func resultErrToContext(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, resultErrKey, err)
}

func resultErrFromContext(ctx context.Context) error {
	err, _ := ctx.Value(resultErrKey).(error)
	return err
}
