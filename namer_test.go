package dagger

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func namedStep(_ context.Context, _ testState) error { return nil }

type testStep struct{}
type unknownStep struct{}

func (s *testStep) StepName() string                                      { return "testStep" }
func (s *testStep) Exec(_ context.Context, _ testState) error             { return nil }
func (s *unknownStep) Exec(_ context.Context, _ testState) error          { return nil }
func (s *unknownStep) internalStep1(_ context.Context, _ testState) error { return nil }
func (s unknownStep) internalStep2(_ context.Context, _ testState) error  { return nil }

func TestStepName(t *testing.T) {
	testcases := []struct {
		name string
		step func() Step[testState]
		want string
	}{
		{
			name: "UnknownStep",
			step: func() Step[testState] { return &unknownStep{} },
			want: "dagger:unknownStep",
		},
		{
			name: "UnknownInternalStepPointerReceiver",
			step: func() Step[testState] {
				return NewStep((&unknownStep{}).internalStep1)
			},
			want: "dagger:*unknownStep.internalStep1",
		},
		{
			name: "UnknownInternalStepValueReceiver",
			step: func() Step[testState] {
				return NewStep(unknownStep{}.internalStep2)
			},
			want: "dagger:unknownStep.internalStep2",
		},
		{
			name: "AnonymousFunction",
			step: func() Step[testState] {
				return NewStep(func(_ context.Context, _ testState) error { return nil })
			},
			want: "dagger:TestStepName.func4.1",
		},
		{
			name: "NamedFunction",
			step: func() Step[testState] { return NewStep(namedStep) },
			want: "dagger:namedStep",
		},
		{
			name: "StepNameMethod",
			step: func() Step[testState] { return &testStep{} },
			want: "testStep",
		},
	}

	for _, tc := range testcases {
		stepName := StepName(tc.step())
		assert.Equal(t, tc.want, stepName.String())
	}
}

type typedStep[S any] struct{}

func (s *typedStep[S]) Exec(_ context.Context, _ S) error { return nil }

func Test_stepTypeName(t *testing.T) {
	thisModule := "github.com/ajatprabha"

	t.Run("StdLibTypedStep", func(t *testing.T) {
		s := StepName(&typedStep[int]{})
		gsn, ok := s.(GenericScopedName)

		assert.True(t, ok)
		assert.Equal(t, thisModule, gsn.StepScopedName().Module())
		assert.Empty(t, gsn.TypeScopedName().Module())
		assert.Equal(t, "int", gsn.TypeScopedName().String())
		assert.Equal(t, "dagger:typedStep[int]", s.String())
	})

	t.Run("SamePackageTypedStep", func(t *testing.T) {
		s := StepName(&typedStep[testState]{})
		gsn, ok := s.(GenericScopedName)

		assert.True(t, ok)
		assert.Equal(t, thisModule, gsn.StepScopedName().Module())
		assert.Equal(t, thisModule, gsn.TypeScopedName().Module())
		assert.Equal(t, "dagger", gsn.TypeScopedName().Package())
		assert.Equal(t, "dagger:typedStep[testState]", s.String())
	})

	t.Run("SamePackageTypedPointerStep", func(t *testing.T) {
		s := StepName(&typedStep[*testState]{})
		gsn, ok := s.(GenericScopedName)

		assert.True(t, ok)
		assert.Equal(t, thisModule, gsn.StepScopedName().Module())
		assert.Equal(t, thisModule, gsn.TypeScopedName().Module())
		assert.Equal(t, "dagger", gsn.TypeScopedName().Package())
		assert.Equal(t, "dagger:typedStep[*testState]", s.String())
	})

	t.Run("DifferentPackageTypedStep", func(t *testing.T) {
		s := StepName(&typedStep[strings.Builder]{})
		gsn, ok := s.(GenericScopedName)

		assert.True(t, ok)
		assert.Empty(t, gsn.TypeScopedName().Module())
		assert.Equal(t, "strings", gsn.TypeScopedName().Package())
		assert.Equal(t, thisModule, gsn.StepScopedName().Module())
		assert.Equal(t, "dagger:typedStep[Builder]", s.String())
	})

	t.Run("DifferentPackageTypedPointerStep", func(t *testing.T) {
		s := StepName(&typedStep[*bytes.Buffer]{})
		gsn, ok := s.(GenericScopedName)

		assert.True(t, ok)
		assert.Empty(t, gsn.TypeScopedName().Module())
		assert.Equal(t, "bytes", gsn.TypeScopedName().Package())
		assert.Equal(t, thisModule, gsn.StepScopedName().Module())
		assert.Equal(t, "dagger:typedStep[*Buffer]", s.String())
	})
}

type namedTypedStep[S any] struct{}

func (s *namedTypedStep[S]) StepName() fmt.Stringer {
	return fmtStr(fmt.Sprintf("namedTypedStep[%T]", *new(S)))
}

func (s *namedTypedStep[S]) Exec(_ context.Context, _ S) error { return nil }

func TestStepNamer(t *testing.T) {
	step := &namedTypedStep[int]{}
	assert.Equal(t, "namedTypedStep[int]", StepName(step).String())
}
