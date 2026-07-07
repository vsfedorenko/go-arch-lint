package dsl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpecCapturesVersion(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Version(1)
	})

	builder, err := FlushSpec()
	assert.NoError(t, err)
	assert.Equal(t, 1, builder.Version.Value)
}

func TestSpecCapturesWorkdir(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Workdir("internal")
	})

	builder, err := FlushSpec()
	assert.NoError(t, err)
	assert.Equal(t, "internal", builder.Workdir.Value)
}

func TestFlushSpecReturnsErrorWhenNotInitialized(t *testing.T) {
	resetSpecBuilder()
	_, err := FlushSpec()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Spec() was not called")
}

func TestSpecBuilderPosition(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Version(1) // line 28 in this test file
	})

	builder, _ := FlushSpec()
	// The Reference should point to the test file
	assert.True(t, builder.Version.Reference.Valid)
	assert.Equal(t, "types_test.go", builder.Version.Reference.File)
}
