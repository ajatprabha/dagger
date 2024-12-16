package dagger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrCycle_Error(t *testing.T) {
	e := &ErrCycle{stepName: fmtStr("test")}
	assert.Equalf(t, "dagger: cycle detected at step 'test'", e.Error(), "Error()")
}

func TestErrInvalid_Error(t *testing.T) {
	e := &ErrInvalid{err: assert.AnError}
	assert.Equalf(t, assert.AnError.Error(), e.Error(), "Error()")
}
