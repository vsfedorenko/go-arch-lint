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

func TestSpecReturnsSpecDef(t *testing.T) {
	resetSpecBuilder()
	spec := Spec(func() {
		Version(1)
		Workdir("internal")
	})

	b, err := FlushSpec()
	assert.NoError(t, err)
	assert.Equal(t, 1, b.Version.Value)
	assert.Equal(t, "internal", b.Workdir.Value)

	merged := MergeSpecs(spec)
	assert.NotNil(t, merged)
	assert.Equal(t, 1, merged.Version.Value)
	assert.Equal(t, "internal", merged.Workdir.Value)
}

func TestMergeSpecsAccumulatesSlices(t *testing.T) {
	resetSpecBuilder()
	s1 := Spec(func() {
		Exclude("dir1")
		ExcludeFiles("^.*_test\\.go$")
		CommonComponents("models")
		CommonVendors("go-common")
	})
	s2 := Spec(func() {
		Exclude("dir2")
		CommonComponents("utils")
	})

	merged := MergeSpecs(s1, s2)
	assert.Len(t, merged.Exclude, 2)
	assert.Len(t, merged.ExcludeFiles, 1)
	assert.Len(t, merged.CommonComponents, 2)
	assert.Len(t, merged.CommonVendors, 1)
}

func TestMergeSpecsMergesMaps(t *testing.T) {
	resetSpecBuilder()
	s1 := Spec(func() {
		Component("a", "internal/a")
		Component("shared", "v1")
	})
	s2 := Spec(func() {
		Component("b", "internal/b")
		Component("shared", "v2")
	})

	merged := MergeSpecs(s1, s2)
	assert.Contains(t, merged.Components, "a")
	assert.Contains(t, merged.Components, "b")
	assert.Equal(t, []string{"v2"}, merged.Components["shared"].RelativePaths)
}

func TestMergeSpecsScalarFirstSetWins(t *testing.T) {
	resetSpecBuilder()
	s1 := Spec(func() {
		Version(1)
		Workdir("internal")
	})
	s2 := Spec(func() {
		Version(2)
	})

	merged := MergeSpecs(s1, s2)
	assert.Equal(t, 1, merged.Version.Value)
	assert.Equal(t, "internal", merged.Workdir.Value)
}

func TestMergeSpecsEmptyReturnsNil(t *testing.T) {
	merged := MergeSpecs()
	assert.Nil(t, merged)
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
