package dagger_test

import (
	"context"
	"fmt"

	"github.com/ajatprabha/dagger"
)

type exampleState struct {
	id string
}

func ExampleNew() {
	dag, err := dagger.New(
		dagger.NewStep(func(ctx context.Context, state exampleState) error {
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}

	if err := dag.Exec(context.Background(), exampleState{}); err != nil {
		panic(err)
	}
}

func ExampleIf() {
	idIsExample := func(state exampleState) bool { return state.id == "example" }

	dag, err := dagger.New(
		dagger.If(
			idIsExample,
			// only executed if idIsExample returns true
			dagger.NewStep(func(ctx context.Context, state exampleState) error {
				return nil
			}),
		),
	)
	if err != nil {
		panic(err)
	}

	if err := dag.Exec(context.Background(), exampleState{id: "example"}); err != nil {
		panic(err)
	}
}

func ExampleIfNot() {
	idIsExample := func(state exampleState) bool { return state.id == "example" }

	dag, err := dagger.New(
		dagger.IfNot(
			idIsExample,
			// only executed if idIsExample returns false
			dagger.NewStep(func(ctx context.Context, state exampleState) error {
				return nil
			}),
		),
	)
	if err != nil {
		panic(err)
	}

	if err := dag.Exec(context.Background(), exampleState{id: "not-example"}); err != nil {
		panic(err)
	}
}

func ExampleIfElse() {
	idIsExample := func(state exampleState) bool { return state.id == "example" }
	skipValidation := func(ctx context.Context, state exampleState) error { return nil }
	validate := func(ctx context.Context, state exampleState) error { return nil }

	dag, err := dagger.New(
		dagger.IfElse(
			idIsExample,
			// only executed if idIsExample returns true
			dagger.NewStep(skipValidation),
			// only executed if idIsExample returns false
			dagger.NewStep(validate),
		),
	)
	if err != nil {
		panic(err)
	}

	if err := dag.Exec(context.Background(), exampleState{id: "not-example"}); err != nil {
		panic(err)
	}
}

func ExampleResult() {
	createResource := func(ctx context.Context, state exampleState) error { return nil }
	reportSuccess := func(ctx context.Context, state exampleState) error { return nil }
	reportFailure := func(ctx context.Context, state exampleState) error { return nil }

	// Tip: Create a type alias like this to avoid using go generic syntax everywhere.
	// type exampleStateStep = dagger.Step[exampleState]

	dag, err := dagger.New(
		dagger.Result(
			// Result first executes the main Step
			dagger.NewStep(createResource),
			// It will then run the success Step, if main Step returned no error
			dagger.NewStep(reportSuccess),
			// Otherwise, it will run the success Step, if main Step returned an error
			dagger.NewStep(reportFailure),
		),
	)
	if err != nil {
		panic(err)
	}

	if err := dag.Exec(context.Background(), exampleState{id: "example"}); err != nil {
		panic(err)
	}
}

func ExampleSeries() {
	validateResource := func(ctx context.Context, state exampleState) error { return nil }
	createResource := func(ctx context.Context, state exampleState) error { return nil }
	reportSuccess := func(ctx context.Context, state exampleState) error { return nil }

	dag, err := dagger.New(
		// Series executes all steps in sequence and returns early if any Step returns error.
		dagger.Series(
			dagger.NewStep(validateResource),
			dagger.NewStep(createResource),
			dagger.NewStep(reportSuccess),
		),
	)
	if err != nil {
		panic(err)
	}

	if err := dag.Exec(context.Background(), exampleState{id: "example"}); err != nil {
		panic(err)
	}
}

func ExampleContinue() {
	validateResource := func(ctx context.Context, state exampleState) error { return nil }
	createResource := func(ctx context.Context, state exampleState) error { return nil }
	reportSuccess := func(ctx context.Context, state exampleState) error { return nil }

	dag, err := dagger.New(
		// Continue executes all steps in sequence ignoring any error returned by a step.
		// The error returned is a combination of all errors returned by the steps.
		dagger.Continue(
			dagger.NewStep(validateResource),
			dagger.NewStep(createResource),
			dagger.NewStep(reportSuccess),
		),
	)
	if err != nil {
		panic(err)
	}

	if err := dag.Exec(context.Background(), exampleState{id: "example"}); err != nil {
		panic(err)
	}
}

type exampleStepStruct struct{}

func (s exampleStepStruct) Exec(_ context.Context, _ exampleState) error { return nil }

func exampleStepFunc(_ context.Context, _ exampleState) error { return nil }

func ExampleStepName() {
	structName := dagger.StepName(&exampleStepStruct{})
	stepFuncName := dagger.StepName(dagger.NewStep(exampleStepFunc))

	fmt.Println(structName, stepFuncName)

	// Output:
	// dagger_test:exampleStepStruct dagger_test:exampleStepFunc
}

func ExampleScopedName_String() {
	stepFn := dagger.NewStep(exampleStepFunc)
	stepName := dagger.StepName(stepFn)
	scopedName, _ := stepName.(dagger.ScopedName)
	fmt.Println(scopedName.String())

	// Output:
	// dagger_test:exampleStepFunc
}

func ExampleScopedName_Module() {
	stepFn := dagger.NewStep(exampleStepFunc)
	stepName := dagger.StepName(stepFn)
	scopedName, _ := stepName.(dagger.ScopedName)
	fmt.Println(scopedName.Module())

	// Output:
	// github.com/ajatprabha
}

func ExampleScopedName_Name() {
	stepFn := dagger.NewStep(exampleStepFunc)
	stepName := dagger.StepName(stepFn)
	scopedName, _ := stepName.(dagger.ScopedName)
	fmt.Println(scopedName.Name())

	// Output:
	// exampleStepFunc
}

func ExampleScopedName_Package() {
	stepFn := dagger.NewStep(exampleStepFunc)
	stepName := dagger.StepName(stepFn)
	scopedName, _ := stepName.(dagger.ScopedName)
	fmt.Println(scopedName.Package())

	// Output:
	// dagger_test
}

func ExampleScopedName_PackagePath() {
	stepFn := dagger.NewStep(exampleStepFunc)
	stepName := dagger.StepName(stepFn)
	scopedName, _ := stepName.(dagger.ScopedName)
	fmt.Println(scopedName.PackagePath())

	// Output:
	// github.com/ajatprabha/dagger_test
}

type exampleTypedStep[S any] struct{}

func (s exampleTypedStep[S]) Exec(ctx context.Context, state exampleState) error { return nil }

func ExampleGenericScopedName_String() {
	stepName := dagger.StepName(&exampleTypedStep[exampleState]{})
	gsn, _ := stepName.(dagger.GenericScopedName)
	fmt.Println(gsn.String())

	// Output:
	// dagger_test:exampleTypedStep[exampleState]
}

func ExampleGenericScopedName_StepScopedName() {
	stepName := dagger.StepName(&exampleTypedStep[exampleState]{})
	gsn, _ := stepName.(dagger.GenericScopedName)
	fmt.Println(gsn.StepScopedName())

	// Output:
	// dagger_test:exampleTypedStep
}

func ExampleGenericScopedName_TypeScopedName() {
	stepName := dagger.StepName(&exampleTypedStep[int]{})
	gsn, _ := stepName.(dagger.GenericScopedName)
	fmt.Println(gsn.TypeScopedName())

	// Output:
	// int
}
