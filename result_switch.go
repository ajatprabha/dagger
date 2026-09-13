package dagger

import (
	"context"
)

// FailureSelector is used to define closures that act as
// branch selector for Result's failure Step(s).
type FailureSelector[S any] func(ctx context.Context, err error) bool

// SwitchCase represents a case in a switch statement for error handling
type SwitchCase[S any] interface {
	match(ctx context.Context, err error) (Step[S], bool)
	Unwrap() Step[S]
}

// switchCase implements SwitchCase for conditional error handling
type switchCase[S any] struct {
	selector FailureSelector[S]
	step     Step[S]
}

func (c switchCase[S]) match(ctx context.Context, err error) (Step[S], bool) {
	if c.selector(ctx, err) {
		return c.step, true
	}
	return nil, false
}

func (c switchCase[S]) Unwrap() Step[S] {
	return c.step
}

// switchDefault implements SwitchCase for default error handling
type switchDefault[S any] struct{ step Step[S] }

func (d switchDefault[S]) match(_ context.Context, _ error) (Step[S], bool) {
	return d.step, true
}

func (d switchDefault[S]) Unwrap() Step[S] {
	return d.step
}

// Case creates a SwitchCase that executes a step when the predicate returns true
func Case[S any](predicate FailureSelector[S], step Step[S]) SwitchCase[S] {
	return switchCase[S]{selector: predicate, step: step}
}

// DefaultCase creates a SwitchCase that executes a step when no other case matches
func DefaultCase[S any](step Step[S]) SwitchCase[S] {
	return switchDefault[S]{step: step}
}

// switchHandler implements resultFailureHandler for switch-based error handling
type switchHandler[S any] struct {
	cases []SwitchCase[S]
}

func (s *switchHandler[S]) selectStep(ctx context.Context, err error) Step[S] {
	for _, c := range s.cases {
		if step, ok := c.match(ctx, err); ok {
			return step
		}
	}
	return nil
}

func (s *switchHandler[S]) Unwrap() []Step[S] {
	steps := make([]Step[S], 0, len(s.cases))
	for _, c := range s.cases {
		steps = append(steps, c.Unwrap())
	}
	return steps
}

// Switch creates a ResultOption that handles multiple failure scenarios
// Usage:
//
//	dagger.Result(mainStep, dagger.Switch(
//	  dagger.Case(errPredicate, errorStep),
//	  dagger.DefaultCase(defaultStep)
//	))
func Switch[S any](cases ...SwitchCase[S]) ResultOption[S] {
	return resultOptionFunc[S](func(cfg *resultConfig[S]) {
		cfg.failureHandler = &switchHandler[S]{cases: cases}
	})
}
