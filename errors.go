package dagger

import "fmt"

// ErrCycle indicates that a cycle was detected in the DAG.
type ErrCycle struct{ stepName fmt.Stringer }

func (e *ErrCycle) Error() string {
	return fmt.Sprintf("dagger: cycle detected at step '%s'", e.stepName)
}

// ErrInvalid indicates that the Executor is invalid.
type ErrInvalid struct{ err error }

func (e *ErrInvalid) Error() string { return e.err.Error() }

func (e *ErrInvalid) Unwrap() error { return e.err }
