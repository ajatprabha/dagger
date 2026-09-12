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
