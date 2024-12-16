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

func TestMiddlewareChain_Wrap(t *testing.T) {
	t.Run("Stacked", func(t *testing.T) {
		buf := new(bytes.Buffer)

		chain := NewChain(
			testLogMiddleware[testState](buf, "L1"),
			testLogMiddleware[testState](buf, "L2"),
		)

		steps := Series(
			NewStep(func(ctx context.Context, state testState) error { return nil }),
		)

		step := chain.Wrap(steps)

		err := step.Exec(context.TODO(), testState{})
		assert.NoError(t, err)
		assert.Equal(t, `L1: Starting step seriesStep[testState]
L2: Starting step seriesStep[testState]
L2: seriesStep[testState] done
L1: seriesStep[testState] done
`, buf.String())
	})
}
