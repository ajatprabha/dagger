package dagger

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStepWithErr_Exec(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		type useState struct{ indent int }

		errCount := 0

		ss := NewResultErrStep(func(ctx context.Context, state useState, err error) error {
			if err != nil {
				errCount++
			}

			return err
		})
		ms := NewStep(func(ctx context.Context, state useState) error { return errors.New("random") })

		step := Result(
			ms,
			OnSuccess(ss),
			Switch[useState](
				Case(func(_ context.Context, err error) bool { return err != nil }, ss),
			),
		)

		dag, err := New(step)
		assert.NoError(t, err)

		buf := new(bytes.Buffer)
		dag.Use(func(next Step[useState], info Info) Step[useState] {
			return NewStep(func(ctx context.Context, state useState) error {
				if info.CanSkip {
					return next.Exec(ctx, useState{indent: state.indent + 1})
				}

				buf.WriteString(strings.Repeat("\t", state.indent-1))
				buf.WriteString(info.Name.String())
				buf.WriteString("\n")

				return next.Exec(ctx, useState{indent: state.indent + 1})
			})
		})

		err = dag.Exec(context.TODO(), useState{})
		assert.Error(t, err)
		assert.Equal(t, 1, errCount)
		assert.Equal(t, `dagger:TestStepWithErr_Exec.func1.2
dagger:StepWithErr[useState·4]
`, buf.String())
	})
}

func TestOnError(t *testing.T) {
	t.Run("executes failure step on error", func(t *testing.T) {
		failureRan := false
		mainErr := errors.New("main error")

		mainStep := NewStep(func(_ context.Context, _ struct{}) error { return mainErr })
		failureStep := NewStep(func(_ context.Context, _ struct{}) error {
			failureRan = true
			return nil
		})

		dagStep := Result(mainStep, OnError(failureStep))
		err := dagStep.Exec(context.Background(), struct{}{})
		assert.NoError(t, err)
		assert.True(t, failureRan)
	})

	t.Run("singleStepHandler unwrap and selection", func(t *testing.T) {
		executed := false
		dummyStep := NewStep(func(_ context.Context, _ struct{}) error {
			executed = true
			return nil
		})
		handler := &singleStepHandler[struct{}]{step: dummyStep}

		selected := handler.selectStep(context.Background(), errors.New("any error"))
		assert.NotNil(t, selected)
		assert.NoError(t, selected.Exec(context.Background(), struct{}{}))
		assert.True(t, executed)

		unwrapped := handler.Unwrap()
		assert.NotNil(t, unwrapped)
	})
}
