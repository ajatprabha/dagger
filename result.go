package dagger

import (
	"context"
)

// Result executes the mainStep and uses the returned value to
//   - execute successStep, if the returned error is nil and OnSuccess option is provided
//   - execute failureHandler, if the returned error is not nil and OnError/Switch option is provided
//
// Usage:
//
//	// Simple error handling
//	dagger.Result(mainStep, dagger.OnError(failureStep))
//	dagger.Result(mainStep, dagger.OnSuccess(successStep))
//	dagger.Result(mainStep, dagger.OnSuccess(successStep), dagger.OnError(failureStep))
//
//	// Complex multi-error handling
//	dagger.Result(mainStep, dagger.Switch(
//	  dagger.Case(func(ctx context.Context, err error) bool { return isNetworkError(err) }, networkErrorStep),
//	  dagger.Case(func(ctx context.Context, err error) bool { return isValidationError(err) }, validationErrorStep),
//	  dagger.DefaultCase(genericErrorStep),
//	))
//
// Note: If the failureHandler returns a nil Step, Result's Step.Exec
// returns the mainStep's error.
func Result[S any](mainStep Step[S], opts ...ResultOption[S]) Step[S] {
	config := &resultConfig[S]{}

	for _, opt := range opts {
		opt.apply(config)
	}

	if mainStep == nil {
		// panicking is acceptable here since this is a programming error
		// and should be caught during development rather than at runtime,
		// DAGs are typically constructed during application initialization.
		panic("mainStep must not be nil")
	}

	if config.successStep == nil && config.failureHandler == nil {
		// panicking is acceptable here since this is a programming error
		// and should be caught during development rather than at runtime,
		// DAGs are typically constructed during application initialization.
		panic("use dagger.OnSuccess, dagger.OnError or dagger.Switch option(s), at least one is required")
	}

	return &resultStep[S]{
		mainStep:       mainStep,
		successStep:    config.successStep,
		failureHandler: config.failureHandler,
	}
}

// OnSuccess creates a ResultOption that sets the success step
func OnSuccess[S any](step Step[S]) ResultOption[S] {
	return resultOptionFunc[S](func(cfg *resultConfig[S]) { cfg.successStep = step })
}

type resultStep[S any] struct {
	mainStep       Step[S]
	successStep    Step[S]
	failureHandler resultFailureHandler[S]
}

var _ middlewareSkipper = (*resultStep[any])(nil)

func (s *resultStep[S]) CanSkipMiddleware() bool {
	return true
}

func (s *resultStep[S]) Exec(ctx context.Context, state S) error {
	if err := execWithContext(ctx, s.mainStep, state); err != nil {
		return s.handleErr(ctx, state, err)
	}

	if s.successStep == nil {
		return nil
	}

	return execWithContext(ctx, s.successStep, state)
}

func (s *resultStep[S]) Unwrap() []Step[S] {
	steps := make([]Step[S], 0, 3)
	steps = append(steps, s.mainStep, s.successStep)

	if s.failureHandler != nil {
		steps = append(steps, unwrapper[S](s.failureHandler)...)
	}

	return steps
}
