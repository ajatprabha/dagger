package dagger

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecutor_Use(t *testing.T) {
	type useState struct{ indent int }
	type useStep = Step[useState]

	validateResource := func(ctx context.Context, state useState) error { return nil }
	createResource := func(ctx context.Context, state useState) error { return nil }
	reportSuccess := func(ctx context.Context, state useState) error { return nil }
	reportFailure := func(ctx context.Context, state useState) error { return nil }

	t.Run("Single", func(t *testing.T) {
		dag, err := New(
			Series(
				NewStep(validateResource),
				NewStep(reportSuccess),
				Result(
					NewStep(createResource),
					NewStep(reportSuccess),
					func(ctx context.Context, state useState, err error) useStep {
						return NewStep(reportFailure)
					},
				),
			),
		)

		assert.NoError(t, err)

		buf := new(bytes.Buffer)
		buf.WriteString("\n")

		dag.Use(
			func(next Step[useState], info Info) Step[useState] {
				return NewStep(func(ctx context.Context, state useState) error {
					buf.WriteString(strings.Repeat("\t", state.indent))
					buf.WriteString(info.Name.String())
					buf.WriteString("\n")
					return next.Exec(ctx, useState{indent: state.indent + 1})
				})
			},
		)

		err = dag.Exec(context.TODO(), useState{})
		assert.NoError(t, err)

		assert.Equal(t, `
dagger:seriesStep[useState·1]
	dagger:TestExecutor_Use.func1
	dagger:TestExecutor_Use.func3
	dagger:resultStep[useState·1]
		dagger:TestExecutor_Use.func2
		dagger:TestExecutor_Use.func3
`, buf.String())
	})

	t.Run("SkipMetaSteps", func(t *testing.T) {
		dag, err := New(
			Series(
				NewStep(validateResource),
				NewStep(reportSuccess),
				Result(
					NewStep(createResource),
					NewStep(reportSuccess),
					func(ctx context.Context, state useState, err error) useStep {
						return NewStep(reportFailure)
					},
				),
				Series(
					If(
						func(state useState) bool { return true },
						NewStep(func(context.Context, useState) error { return nil }),
					),
					IfNot(
						func(state useState) bool { return false },
						NewStep(func(context.Context, useState) error { return nil }),
					),
					IfElse(
						func(state useState) bool { return true },
						NewStep(func(context.Context, useState) error { return nil }),
						NewStep(func(context.Context, useState) error { return nil }),
					),
				),
				Continue(
					NewStep(func(context.Context, useState) error { return nil }),
					NewStep(func(context.Context, useState) error { return nil }),
				),
			),
		)

		assert.NoError(t, err)

		buf := new(bytes.Buffer)
		buf.WriteString("\n")

		dag.Use(
			func(next Step[useState], info Info) Step[useState] {
				return NewStep(func(ctx context.Context, state useState) error {
					if info.CanSkip {
						return next.Exec(ctx, useState{indent: state.indent + 1})
					}

					buf.WriteString(strings.Repeat("\t", state.indent-1))
					buf.WriteString(info.Name.String())
					buf.WriteString("\n")

					return next.Exec(ctx, useState{indent: state.indent + 1})
				})
			},
		)

		err = dag.Exec(context.TODO(), useState{})
		assert.NoError(t, err)

		assert.Equal(t, `
dagger:TestExecutor_Use.func1
dagger:TestExecutor_Use.func3
	dagger:TestExecutor_Use.func2
	dagger:TestExecutor_Use.func3
		dagger:TestExecutor_Use.func6.3
		dagger:TestExecutor_Use.func6.5
		dagger:TestExecutor_Use.func6.7
	dagger:TestExecutor_Use.func6.9
	dagger:TestExecutor_Use.func6.10
`, buf.String())
	})

	t.Run("Multiple", func(t *testing.T) {
		dag, err := New(
			Series(
				NewStep(validateResource),
				NewStep(reportSuccess),
				Result(
					NewStep(createResource),
					NewStep(reportSuccess),
					func(ctx context.Context, state useState, err error) useStep {
						return NewStep(reportFailure)
					},
				),
			),
		)

		assert.NoError(t, err)

		buf := new(bytes.Buffer)

		dag.Use(
			testLogMiddleware[useState](buf, "L1"),
			testLogMiddleware[useState](buf, "L2"),
		)

		err = dag.Exec(context.TODO(), useState{})
		assert.NoError(t, err)

		assert.Equal(t, `L1: Starting step seriesStep[useState·1]
L2: Starting step seriesStep[useState·1]
L1: Starting step TestExecutor_Use.func1
L2: Starting step TestExecutor_Use.func1
L2: TestExecutor_Use.func1 done
L1: TestExecutor_Use.func1 done
L1: Starting step TestExecutor_Use.func3
L2: Starting step TestExecutor_Use.func3
L2: TestExecutor_Use.func3 done
L1: TestExecutor_Use.func3 done
L1: Starting step resultStep[useState·1]
L2: Starting step resultStep[useState·1]
L1: Starting step TestExecutor_Use.func2
L2: Starting step TestExecutor_Use.func2
L2: TestExecutor_Use.func2 done
L1: TestExecutor_Use.func2 done
L1: Starting step TestExecutor_Use.func3
L2: Starting step TestExecutor_Use.func3
L2: TestExecutor_Use.func3 done
L1: TestExecutor_Use.func3 done
L2: resultStep[useState·1] done
L1: resultStep[useState·1] done
L2: seriesStep[useState·1] done
L1: seriesStep[useState·1] done
`, buf.String())
	})
}

func Test_buildDAG(t *testing.T) {
	trueCondition := func(s dummyState) bool { return true }

	step0 := NewStep(setDBState)
	step1then := &ifStep[dummyState]{
		condition: trueCondition,
		thenStep:  step0,
	}
	step1 := &ifElseStep[dummyState]{
		condition: trueCondition,
		thenStep:  step1then,
	}
	step1ContinueStep := &continueStep[dummyState]{
		steps: []Step[dummyState]{NewStep(setDBErr), NewStep(updateDB)},
	}
	step1.elseStep = step1ContinueStep

	step2 := NewStep(setDBErr)
	step3 := NewStep(deleteResource)
	step4 := NewStep(publishKafka)
	step5 := NewStep(updateDB)
	resultStep := &resultStep[dummyState]{
		mainStep:    step2,
		successStep: step3,
		failureHandler: func(ctx context.Context, state dummyState, err error) Step[dummyState] {
			return step4
		},
	}

	rootStep := &ifElseStep[dummyState]{
		condition: trueCondition,
		thenStep:  step1,
		elseStep: &seriesStep[dummyState]{
			steps: []Step[dummyState]{resultStep, step5},
		},
	}

	t.Run("acyclic dag", func(t *testing.T) {
		err := checkDAGCycles(rootStep)
		assert.NoError(t, err)
	})

	t.Run("cyclic dag", func(t *testing.T) {
		errCycle := new(ErrCycle)

		resultStep.successStep = resultStep
		_, err := New(rootStep)
		assert.ErrorAs(t, err, &errCycle)
		assert.Equal(t, "dagger:resultStep[dummyState]", errCycle.stepName.String())
		resultStep.successStep = step3

		resultStep.mainStep = rootStep.elseStep
		_, err = New(rootStep)
		assert.ErrorAs(t, err, &errCycle)
		assert.Equal(t, "dagger:seriesStep[dummyState]", errCycle.stepName.String())
		resultStep.mainStep = step2

		rootStep.thenStep = rootStep
		_, err = New(rootStep)
		assert.ErrorAs(t, err, &errCycle)
		assert.Equal(t, "dagger:ifElseStep[dummyState]", errCycle.stepName.String())
		rootStep.thenStep = step1

		rootStep.elseStep = rootStep
		_, err = New(rootStep)
		assert.ErrorAs(t, err, &errCycle)
		assert.Equal(t, "dagger:ifElseStep[dummyState]", errCycle.stepName.String())
		rootStep.thenStep = step1

		step1then.thenStep = step1then
		_, err = New(rootStep)
		assert.ErrorAs(t, err, &errCycle)
		assert.Equal(t, "dagger:ifStep[dummyState]", errCycle.stepName.String())
		step1then.thenStep = step0

		ogStep := step1ContinueStep.steps[0]
		step1ContinueStep.steps[0] = step1ContinueStep
		_, err = New(rootStep)
		assert.ErrorAs(t, err, &errCycle)
		assert.Equal(t, "dagger:continueStep[dummyState]", errCycle.stepName.String())
		step1ContinueStep.steps[0] = ogStep
	})
}

type dummyState struct{}

func setDBState(ctx context.Context, state dummyState) error {
	return nil
}

func deleteResource(ctx context.Context, state dummyState) error {
	return nil
}

func publishKafka(ctx context.Context, state dummyState) error {
	return nil
}

func setDBErr(ctx context.Context, state dummyState) error {
	return nil
}

func updateDB(ctx context.Context, state dummyState) error {
	return nil
}
