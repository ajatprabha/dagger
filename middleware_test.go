package dagger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testLogMiddleware[S any](w io.Writer, prefix string) MiddlewareFunc[S] {
	return func(next Step[S], info Info) Step[S] {
		return NewStep(func(ctx context.Context, state S) error {
			name := info.Name
			if sn, ok := name.(ScopedName); ok {
				name = fmtStr(sn.Name())
			}
			if gsn, ok := name.(GenericScopedName); ok {
				name = fmtStr(fmt.Sprintf(
					"%s[%s]",
					gsn.StepScopedName().Name(),
					gsn.TypeScopedName().Name(),
				))
			}
			_, _ = fmt.Fprintf(w, "%s: Starting step %s\n", prefix, name)

			defer func() { _, _ = fmt.Fprintf(w, "%s: %s done\n", prefix, name) }()

			return next.Exec(ctx, state)
		})
	}
}

func TestMiddlewareFunc_Wrap(t *testing.T) {
	t.Run("Single", func(t *testing.T) {
		buf := new(bytes.Buffer)

		mw := testLogMiddleware[struct{}](buf, "L1")

		steps := Series(
			NewStep(func(ctx context.Context, state struct{}) error { return nil }),
		)

		step := mw.Wrap(steps)

		err := step.Exec(context.TODO(), struct{}{})
		assert.NoError(t, err)
		assert.Equal(t, `L1: Starting step seriesStep[struct {}]
L1: seriesStep[struct {}] done
`, buf.String())
	})
}

func TestMiddlewareChain_Wrap(t *testing.T) {
	t.Run("Stacked", func(t *testing.T) {
		buf := new(bytes.Buffer)

		chain := NewChain(
			testLogMiddleware[struct{}](buf, "L1"),
			testLogMiddleware[struct{}](buf, "L2"),
		)

		steps := Series(
			NewStep(func(ctx context.Context, state struct{}) error { return nil }),
		)

		step := chain.Wrap(steps)

		err := step.Exec(context.TODO(), struct{}{})
		assert.NoError(t, err)
		assert.Equal(t, `L1: Starting step seriesStep[struct {}]
L2: Starting step seriesStep[struct {}]
L2: seriesStep[struct {}] done
L1: seriesStep[struct {}] done
`, buf.String())
	})
}

func Test_canSkipMiddleware(t *testing.T) {
	testcases := []struct {
		name string
		step Step[struct{}]
	}{
		{
			name: "If",
			step: If(alwaysTrue, NewStep(func(context.Context, struct{}) error { return nil })),
		},
		{
			name: "IfNot",
			step: IfNot(alwaysTrue, NewStep(func(context.Context, struct{}) error { return nil })),
		},
		{
			name: "IfElse",
			step: IfElse(alwaysTrue,
				NewStep(func(context.Context, struct{}) error { return nil }),
				NewStep(func(context.Context, struct{}) error { return nil }),
			),
		},
		{
			name: "Result",
			step: Result(
				NewStep(func(context.Context, struct{}) error { return nil }),
				OnSuccess(NewStep(func(context.Context, struct{}) error { return nil })),
				OnError(NewStep(func(context.Context, struct{}) error { return nil })),
			),
		},
		{
			name: "Series",
			step: Series(
				NewStep(func(context.Context, struct{}) error { return nil }),
				NewStep(func(context.Context, struct{}) error { return nil }),
			),
		},
		{
			name: "Continue",
			step: Continue(
				NewStep(func(context.Context, struct{}) error { return nil }),
				NewStep(func(context.Context, struct{}) error { return nil }),
			),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			f, ok := tc.step.(middlewareSkipper)
			assert.True(t, ok)
			assert.True(t, f.CanSkipMiddleware())
		})
	}
}
