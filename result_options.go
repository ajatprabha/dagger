package dagger

import (
	"context"
)

// ResultOption defines a functional option for configuring a Result step
type ResultOption[S any] interface{ apply(*resultConfig[S]) }

// resultConfig holds the configuration for a Result step
type resultConfig[S any] struct {
	successStep    Step[S]
	failureHandler resultFailureHandler[S]
}

type resultOptionFunc[S any] func(*resultConfig[S])

func (f resultOptionFunc[S]) apply(cfg *resultConfig[S]) { f(cfg) }

// resultFailureHandler is used to define entities that act as
// failure handler for Result Step.
type resultFailureHandler[S any] interface {
	// selectStep is used to select the Step to be executed
	// based on the error returned by the mainStep.
	selectStep(ctx context.Context, err error) Step[S]
}
