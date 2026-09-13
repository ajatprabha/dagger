package dagger

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testNoopStep(_ context.Context, _ struct{}) error        { return nil }
func testElseErrStep(_ context.Context, _ struct{}) error     { return errors.New("else error") }
func testSeriesErrStep(_ context.Context, _ struct{}) error   { return errors.New("series error") }
func testContinueErrStep(_ context.Context, _ struct{}) error { return errors.New("continue error") }
func testFailureErrStep(_ context.Context, _ struct{}) error  { return errors.New("failure") }
func testInnerErrStep(_ context.Context, _ struct{}) error    { return errors.New("inner error") }
func testResultErrStep(_ context.Context, _ struct{}) error   { return errors.New("result error") }
func testBranchErrStep(_ context.Context, _ struct{}) error   { return errors.New("branch error") }

func TestPrintDAG(t *testing.T) {
	sharedStep := NewStep(testNoopStep)
	tests := []struct {
		name           string
		startStep      Step[struct{}]
		expectedOutput string
	}{
		{
			name:      "simple StepFunc",
			startStep: NewStep(testNoopStep),
			expectedOutput: `
dagger:testNoopStep`,
		},
		{
			name: "If step",
			startStep: If(
				func(s struct{}) bool { return true },
				NewStep(testNoopStep),
			),
			expectedOutput: `
dagger:ifStep[struct {}]
    └── dagger:testNoopStep`,
		},
		{
			name: "IfElse step",
			startStep: IfElse(
				func(s struct{}) bool { return true },
				NewStep(testNoopStep),
				NewStep(testElseErrStep),
			),
			expectedOutput: `
dagger:ifElseStep[struct {}]
    ├── dagger:testNoopStep
    └── dagger:testElseErrStep`,
		},
		{
			name: "Series step with multiple children",
			startStep: Series(
				NewStep(testNoopStep),
				NewStep(testSeriesErrStep),
				sharedStep, // Shared reference
			),
			expectedOutput: `
dagger:seriesStep[struct {}]
    ├── dagger:testNoopStep
    ├── dagger:testSeriesErrStep
    └── dagger:testNoopStep`,
		},
		{
			name: "Continue step with multiple children",
			startStep: Continue(
				sharedStep,
				NewStep(testContinueErrStep),
				sharedStep,
			),
			expectedOutput: `
dagger:continueStep[struct {}]
    ├── dagger:testNoopStep
    ├── dagger:testContinueErrStep
    └── dagger:testNoopStep`,
		},
		{
			name: "Result step",
			startStep: Result(
				sharedStep,
				OnSuccess(sharedStep),
				OnError(NewStep(testFailureErrStep)),
			),
			expectedOutput: `
dagger:resultStep[struct {}]
    ├── dagger:testNoopStep [main]
    ├── dagger:testNoopStep [success]
    └── dagger:testFailureErrStep [failure]`,
		},
		{
			name: "complex nested structure",
			startStep: Series(
				If(func(s struct{}) bool { return true },
					Result(
						sharedStep,
						OnSuccess(NewStep(testNoopStep)),
						OnError(NewStep(testInnerErrStep)),
					),
				),
				Continue(
					sharedStep,
					NewStep(testContinueErrStep),
				),
				IfElse(
					func(s struct{}) bool { return false },
					NewStep(testNoopStep),
					Result(
						NewStep(testResultErrStep),
						Switch[struct{}](
							Case(
								func(ctx context.Context, err error) bool {
									return true
								},
								NewStep(testBranchErrStep),
							),
							Case(
								func(ctx context.Context, err error) bool { return false },
								sharedStep,
							),
							DefaultCase(Result(
								sharedStep,
								OnError(NewStep(testNoopStep)),
							)),
						),
					),
				),
				sharedStep,
			),
			expectedOutput: `
dagger:seriesStep[struct {}]
    ├── dagger:ifStep[struct {}]
    │   └── dagger:resultStep[struct {}]
    │       ├── dagger:testNoopStep [main]
    │       ├── dagger:testNoopStep [success]
    │       └── dagger:testInnerErrStep [failure]
    ├── dagger:continueStep[struct {}]
    │   ├── dagger:testNoopStep
    │   └── dagger:testContinueErrStep
    ├── dagger:ifElseStep[struct {}]
    │   ├── dagger:testNoopStep
    │   └── dagger:resultStep[struct {}]
    │       ├── dagger:testResultErrStep [main]
    │       ├── nil [success]
    │       ├── dagger:testBranchErrStep [failure]
    │       ├── dagger:testNoopStep [failure]
    │       └── dagger:resultStep[struct {}] [failure]
    │           ├── dagger:testNoopStep
    │           ├── nil
    │           └── dagger:testNoopStep
    └── dagger:testNoopStep`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput, err := PrintString(tc.startStep)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedOutput[1:], actualOutput)
		})
	}
}

func TestPrintDAG_SharedReferences(t *testing.T) {
	sharedRefTests := []struct {
		name      string
		buildStep func() Step[struct{}]
	}{
		{
			name: "self-referencing ifStep creates reference",
			buildStep: func() Step[struct{}] {
				step := &ifStep[struct{}]{
					condition: func(s struct{}) bool { return true },
				}
				// Create self-reference - should create reference node
				// checkDAGCycles will prevent infinite loops but
				// printing should show reference for easier debugging
				step.thenStep = step
				return step
			},
		},
	}

	for _, tc := range sharedRefTests {
		t.Run(tc.name, func(t *testing.T) {
			step := tc.buildStep()

			output, err := PrintString(step, WithReferences())

			assert.NoError(t, err)
			assert.Contains(t, output, "(ref to ")
		})
	}
}

// mockWriter is a writer that always returns an error for testing
type mockWriter struct {
	errorAfter int
	written    int
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	m.written += len(p)
	if m.written >= m.errorAfter {
		return 0, errors.New("mock write error")
	}
	return len(p), nil
}

func TestPrintDAG_WriterErrors(t *testing.T) {
	tests := []struct {
		name      string
		startStep Step[struct{}]
	}{
		{
			name:      "writer error during printing",
			startStep: NewStep(func(_ context.Context, _ struct{}) error { return nil }),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockW := &mockWriter{errorAfter: 10}
			err := Print(tc.startStep, WithWriter(mockW))

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "mock write error")
		})
	}
}

func TestPrintDAG_NilStep(t *testing.T) {
	tests := []struct {
		name           string
		step           Step[struct{}]
		expectedOutput string
	}{
		{
			name:           "nil step input",
			step:           nil,
			expectedOutput: "nil",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := PrintString(tc.step)

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedOutput, out)
		})
	}
}

func TestPrint_ResultStepFormatting(t *testing.T) {
	t.Run("prints result step branches with branch labels", func(t *testing.T) {
		main := NewStep(func(_ context.Context, _ struct{}) error { return nil })
		success := NewStep(func(_ context.Context, _ struct{}) error { return nil })
		failure := NewStep(func(_ context.Context, _ struct{}) error { return nil })

		step := Result(main, OnSuccess(success), OnError(failure))

		buf := &bytes.Buffer{}
		opts := defaultPrintOptions()
		opts.writer = buf

		dp := &dagPrinter[struct{}]{
			opts:    opts,
			visited: make(map[string]bool),
		}

		err := dp.printWithLabel(main, "", false, " [main]")
		assert.NoError(t, err)

		rs, ok := step.(*resultStep[struct{}])
		assert.True(t, ok)
		err = rs.printDAG(dp, "", true, false)
		assert.NoError(t, err)
		assert.NotEmpty(t, buf.String())
	})
}

func TestPrintOptions(t *testing.T) {
	step := Series(NewStep(testNoopStep), NewStep(testElseErrStep))

	t.Run("WithCompactSymbols", func(t *testing.T) {
		out, err := PrintString(step, WithCompactSymbols())
		assert.NoError(t, err)
		assert.Contains(t, out, "|- ")
		assert.Contains(t, out, "`- ")
	})

	t.Run("WithSymbols", func(t *testing.T) {
		out, err := PrintString(step, WithSymbols("+-- ", "\\-- ", "|   ", "    "))
		assert.NoError(t, err)
		assert.Contains(t, out, "+-- ")
		assert.Contains(t, out, "\\-- ")
	})

	t.Run("WithIndent", func(t *testing.T) {
		out, err := PrintString(step, WithIndent("  "))
		assert.NoError(t, err)
		assert.NotEmpty(t, out)
	})

	t.Run("WithoutLabels", func(t *testing.T) {
		resStep := Result(NewStep(testNoopStep), OnSuccess(NewStep(testNoopStep)))
		out, err := PrintString(resStep, WithoutLabels())
		assert.NoError(t, err)
		assert.NotContains(t, out, "[main]")
		assert.NotContains(t, out, "[success]")
	})
}
