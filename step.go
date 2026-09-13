package dagger

import (
	"context"
)

// Step is a unit of work to be performed in the DAG.
// It can either be a part of a vertex, or an operation
// to reach to another Vertex, making the Step an edge.
type Step[S any] interface {
	// Exec is responsible to perform the actual operation.
	Exec(ctx context.Context, state S) error
}

// Selector is used to define closures that act as
// branch selector for Step(s).
type Selector[S any] func(state S) bool
