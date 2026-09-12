package dagger

// IfNot Step takes in a Selector and runs the thenStep, iff Selector returns false.
func IfNot[S any](condition Selector[S], thenStep Step[S]) Step[S] {
	return &ifStep[S]{
		condition: func(state S) bool { return !condition(state) },
		thenStep:  thenStep,
	}
}
