package dagger

import (
	"context"
)

// FailureSelector is used to define closures that act as
// branch selector for Result's failure Step(s).
type FailureSelector[S any] func(ctx context.Context, err error) bool

// stepWithErr is used to define closures that act as Step with error.
type stepWithErr[S any] func(ctx context.Context, state S, resErr error) error

// NewResultErrStep creates a new Step that also has access to the error from
// the Result Step's main Step.
func NewResultErrStep[S any](f func(ctx context.Context, state S, resErr error) error) Step[S] {
	return stepWithErr[S](f)
}

func (f stepWithErr[S]) Exec(ctx context.Context, state S) error {
	var s Step[S] = StepFunc[S](f.exec)

	c, ok := ctx.Value(middlewareKey).(MiddlewareChain[S])
	if ok {
		si := stepInfo(s)
		si.CanSkip = true
		s = c.apply(s, si)
	}

	return s.Exec(ctx, state)
}

func (f stepWithErr[S]) exec(ctx context.Context, state S) error {
	return f(ctx, state, resultErrFromContext(ctx))
}

// ResultFailureHandler is used to define entities that act as
// failure handler for Result Step.
type ResultFailureHandler[S any] interface {
	// selectStep is used to select the Step to be executed
	// based on the error returned by the mainStep.
	selectStep(ctx context.Context, err error) Step[S]
}

type resultStep[S any] struct {
	mainStep       Step[S]
	successStep    Step[S]
	failureHandler ResultFailureHandler[S]
}

var _ middlewareSkipper = (*resultStep[any])(nil)

func (s *resultStep[S]) canSkip() bool {
	return true
}

func (s *resultStep[S]) Exec(ctx context.Context, state S) error {
	if err := execWithContext(ctx, s.mainStep, state); err != nil {
		return s.handleErr(ctx, state, err)
	}

	return execWithContext(ctx, s.successStep, state)
}

func (s *resultStep[S]) Unwrap() []Step[S] {
	return []Step[S]{
		s.mainStep,
		s.successStep,
		// TODO: Make failure handler a part of the DAG, update Unwrap to return it.
	}
}

func (s *resultStep[S]) handleErr(ctx context.Context, state S, err error) error {
	if s.failureHandler == nil {
		return err
	}

	if step := s.failureHandler.selectStep(ctx, err); step != nil {
		return execWithContext(resultErrToContext(ctx, err), step, state)
	}

	return err
}

var _ ResultFailureHandler[any] = StepFunc[any](nil)

//nolint:unused
func (f StepFunc[S]) selectStep(_ context.Context, _ error) Step[S] { return f }

// Result executes the mainStep and uses the returned value to
//   - execute successStep, if the returned error is nil
//   - execute failureHandler, if the returned error is not nil
//
// Note: The failureHandler is used to define the failure branch,
// if the failureHandler returns a nil Step, Result's Step.Exec
// returns the mainStep's error.
func Result[S any](mainStep, successStep Step[S], failureHandler ResultFailureHandler[S]) Step[S] {
	return &resultStep[S]{
		mainStep:       mainStep,
		successStep:    successStep,
		failureHandler: failureHandler,
	}
}

type failureBranch[S any] struct {
	selector FailureSelector[S]
	step     Step[S]
}

var _ FailureBranch[any] = (*failureBranch[any])(nil)

//nolint:unused
func (s *failureBranch[S]) isFailureBranch(S) {}

var _ ResultFailureHandler[any] = (*failureBranch[any])(nil)

//nolint:unused
func (s *failureBranch[S]) selectStep(ctx context.Context, err error) Step[S] {
	if s.selector(ctx, err) {
		return s.step
	}

	return nil
}

// NewBranch creates a new FailureBranch with the given FailureSelector and Step.
func NewBranch[S any](selector FailureSelector[S], step Step[S]) FailureBranch[S] {
	return &failureBranch[S]{selector: selector, step: step}
}

type defaultBranch[S any] struct {
	step Step[S]
}

var _ FailureBranch[any] = (*defaultBranch[any])(nil)

//nolint:unused
func (d *defaultBranch[S]) isFailureBranch(S) {}

var _ ResultFailureHandler[any] = (*defaultBranch[any])(nil)

//nolint:unused
func (d *defaultBranch[S]) selectStep(_ context.Context, _ error) Step[S] { return d.step }

func DefaultBranch[S any](step Step[S]) FailureBranch[S] {
	return &defaultBranch[S]{step: step}
}

// FailureBranch is used to define entities that act as branches
// in a ResultFailureHandler.
//
// Note: This is used to prevent misuse of the HandleMultiFailure function.
type FailureBranch[S any] interface{ isFailureBranch(S) }

type multiFailureHandler[S any] struct {
	branches []ResultFailureHandler[S]
}

var _ ResultFailureHandler[any] = (*multiFailureHandler[any])(nil)

//nolint:unused
func (m *multiFailureHandler[S]) selectStep(ctx context.Context, err error) Step[S] {
	for _, branch := range m.branches {
		if step := branch.selectStep(ctx, err); step != nil {
			return step
		}
	}

	return nil
}

// HandleMultiFailure takes in FailureBranch(s) and returns a ResultFailureHandler.
// It is used to handle multiple failure branches in a Result Step.
// The branches are evaluated in the order they are passed.
//
// The behavior is as follows:
//   - if a FailureBranch is eligible, it is executed and the remaining branches
//     are ignored.
//   - if no FailureBranch is eligible, the DefaultBranch is executed, if provided,
//     otherwise the mainStep's error is returned.
func HandleMultiFailure[S any](branches ...FailureBranch[S]) ResultFailureHandler[S] {
	res := make([]ResultFailureHandler[S], len(branches))

	for i, branch := range branches {
		switch b := branch.(type) {
		case *failureBranch[S]:
			res[i] = b
		case *defaultBranch[S]:
			res[i] = b
		}
	}

	return &multiFailureHandler[S]{branches: res}
}
