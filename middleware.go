package dagger

import (
	"fmt"
)

type middleware[S any] interface {
	apply(next Step[S], info Info) Step[S]
}

type middlewareSkipper interface{ canSkip() bool }

// Info contains information about the Step.
type Info struct {
	// Name is the name of the Step.
	Name fmt.Stringer
	// CanSkip indicates if the Step can be skipped by the middleware.
	CanSkip bool
}

// MiddlewareFunc allows you wrap a Step with another Step.
// The next Step is passed as an argument to the function.
// The info argument contains information about the Step.
type MiddlewareFunc[S any] func(next Step[S], info Info) Step[S]

// MiddlewareChain allows you to wrap a Step with a
// chain of middlewares, the execution happens in order.
type MiddlewareChain[S any] []middleware[S]

//nolint:unused
func (mwf MiddlewareFunc[S]) apply(next Step[S], info Info) Step[S] {
	return mwf(next, info)
}

func (mwc MiddlewareChain[S]) apply(next Step[S], info Info) Step[S] {
	for i := len(mwc) - 1; i >= 0; i-- {
		mw := mwc[i]
		next = mw.apply(next, info)
	}

	return next
}

func NewChain[S any](mws ...MiddlewareFunc[S]) MiddlewareChain[S] {
	mwc := make(MiddlewareChain[S], len(mws))

	for i, mw := range mws {
		mwc[i] = mw
	}

	return mwc
}

// Wrap applies the middleware chain to the provided Step.
func (mwc MiddlewareChain[S]) Wrap(s Step[S]) Step[S] { return mwc.apply(s, stepInfo(s)) }

func stepInfo[S any](s Step[S]) Info {
	return Info{
		Name:    StepName(s),
		CanSkip: canSkip(s),
	}
}

func canSkip[S any](s Step[S]) bool {
	skipper, ok := s.(middlewareSkipper)
	if ok {
		return skipper.canSkip()
	}

	return false
}
