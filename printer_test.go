package dagger

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintDAG(t *testing.T) {
	doNothingStep := func() Step[struct{}] {
		return NewStep(func(_ context.Context, _ struct{}) error { return nil })
	}
	doNothingSharedRef := doNothingStep()
	tests := []struct {
		name           string
		startStep      Step[struct{}]
		expectedOutput string
	}{
		{
			name: "simple StepFunc",
			startStep: NewStep(func(ctx context.Context, state struct{}) error {
				return nil
			}),
			expectedOutput: `
dagger:TestPrintDAG.func2`,
		},
		{
			name: "If step",
			startStep: If(
				func(s struct{}) bool { return true },
				doNothingStep(),
			),
			expectedOutput: `
dagger:ifStep[struct {}]
    └── dagger:TestPrintDAG.func1.func1`,
		},
		{
			name: "IfElse step",
			startStep: IfElse(
				func(s struct{}) bool { return true },
				doNothingStep(),
				NewStep(func(_ context.Context, _ struct{}) error {
					return errors.New("else error")
				}),
			),
			expectedOutput: `
dagger:ifElseStep[struct {}]
    ├── dagger:TestPrintDAG.func1.func1
    └── dagger:TestPrintDAG.func5`,
		},
		{
			name: "Series step with multiple children",
			startStep: Series(
				doNothingStep(),
				NewStep(func(_ context.Context, _ struct{}) error {
					return errors.New("series error")
				}),
				doNothingSharedRef, // Shared reference
			),
			expectedOutput: `
dagger:seriesStep[struct {}]
    ├── dagger:TestPrintDAG.func1.func1
    ├── dagger:TestPrintDAG.func6
    └── dagger:TestPrintDAG.func1.func1`,
		},
		{
			name: "Continue step with multiple children",
			startStep: Continue(
				doNothingSharedRef,
				NewStep(func(_ context.Context, _ struct{}) error {
					return errors.New("continue error")
				}),
				doNothingSharedRef,
			),
			expectedOutput: `
dagger:continueStep[struct {}]
    ├── dagger:TestPrintDAG.func1.func1
    ├── dagger:TestPrintDAG.func7
    └── dagger:TestPrintDAG.func1.func1`,
		},
		{
			name: "Result step",
			startStep: Result(
				doNothingSharedRef,
				OnSuccess(doNothingSharedRef),
				OnError(NewStep(func(ctx context.Context, state struct{}) error {
					return errors.New("failure")
				})),
			),
			expectedOutput: `
dagger:resultStep[struct {}]
    ├── dagger:TestPrintDAG.func1.func1 [main]
    ├── dagger:TestPrintDAG.func1.func1 [success]
    └── dagger:TestPrintDAG.func8 [failure]`,
		},
		{
			name: "complex nested structure",
			startStep: Series(
				If(func(s struct{}) bool { return true },
					Result(
						doNothingSharedRef,
						OnSuccess(NewStep(func(ctx context.Context, state struct{}) error { return nil })),
						OnError(NewStep(func(ctx context.Context, state struct{}) error {
							return errors.New("inner error")
						})),
					),
				),
				Continue(
					doNothingSharedRef,
					NewStep(func(_ context.Context, _ struct{}) error {
						return errors.New("continue error")
					}),
				),
				IfElse(
					func(s struct{}) bool { return false },
					doNothingStep(),
					Result(
						NewStep(func(_ context.Context, _ struct{}) error {
							return errors.New("result error")
						}),
						Switch[struct{}](
							Case(
								func(ctx context.Context, err error) bool {
									return true
								},
								NewStep(func(_ context.Context, _ struct{}) error {
									return errors.New("branch error")
								}),
							),
							Case(
								func(ctx context.Context, err error) bool { return false },
								doNothingSharedRef,
							),
							DefaultCase(Result(
								doNothingSharedRef,
								OnError(doNothingStep()),
							)),
						),
					),
				),
				doNothingSharedRef,
			),
			expectedOutput: `
dagger:seriesStep[struct {}]
    ├── dagger:ifStep[struct {}]
    │   └── dagger:resultStep[struct {}]
    │       ├── dagger:TestPrintDAG.func1.func1 [main]
    │       ├── dagger:TestPrintDAG.func10 [success]
    │       └── dagger:TestPrintDAG.func11 [failure]
    ├── dagger:continueStep[struct {}]
    │   ├── dagger:TestPrintDAG.func1.func1
    │   └── dagger:TestPrintDAG.func12
    ├── dagger:ifElseStep[struct {}]
    │   ├── dagger:TestPrintDAG.func1.func1
    │   └── dagger:resultStep[struct {}]
    │       ├── dagger:TestPrintDAG.func14 [main]
    │       ├── nil [success]
    │       ├── dagger:TestPrintDAG.func16 [failure]
    │       ├── dagger:TestPrintDAG.func1.func1 [failure]
    │       └── dagger:resultStep[struct {}] [failure]
    │           ├── dagger:TestPrintDAG.func1.func1
    │           ├── nil
    │           └── dagger:TestPrintDAG.func1.func1
    └── dagger:TestPrintDAG.func1.func1`,
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
