package dsl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpecCapturesVersion(t *testing.T) {
	b := Spec(func() {
		Version(1)
	}).Builder()

	assert.Equal(t, 1, b.Version.Value)
	assert.True(t, b.Version.Reference.Valid)
}

func TestSpecCapturesWorkdir(t *testing.T) {
	b := Spec(func() {
		Workdir("internal")
	}).Builder()

	assert.Equal(t, "internal", b.Workdir.Value)
}

func TestSpecReturnsSpecDef(t *testing.T) {
	spec := Spec(func() {
		Version(1)
		Workdir("internal")
	})

	b := spec.Builder()
	assert.NotNil(t, b)
	assert.Equal(t, 1, b.Version.Value)
	assert.Equal(t, "internal", b.Workdir.Value)
}

func TestMergeSpecsAccumulatesSlices(t *testing.T) {
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

	b := MergeSpecs(s1, s2).Builder()
	assert.Len(t, b.Exclude, 2)
	assert.Len(t, b.ExcludeFiles, 1)
	assert.Len(t, b.CommonComponents, 2)
	assert.Len(t, b.CommonVendors, 1)
}

func TestMergeSpecsMergesMaps(t *testing.T) {
	s1 := Spec(func() {
		Component("a", "internal/a")
		Component("shared", "v1")
	})
	s2 := Spec(func() {
		Component("b", "internal/b")
		Component("shared", "v2")
	})

	b := MergeSpecs(s1, s2).Builder()
	assert.Contains(t, b.Components, "a")
	assert.Contains(t, b.Components, "b")
	assert.Equal(t, []string{"v2"}, b.Components["shared"].RelativePaths)
}

func TestMergeSpecsScalarFirstSetWins(t *testing.T) {
	s1 := Spec(func() {
		Version(1)
		Workdir("internal")
	})
	s2 := Spec(func() {
		Version(2)
	})

	b := MergeSpecs(s1, s2).Builder()
	assert.Equal(t, 1, b.Version.Value)
	assert.Equal(t, "internal", b.Workdir.Value)
}

func TestMergeSpecsEmptyReturnsNilBuilder(t *testing.T) {
	assert.Nil(t, MergeSpecs().Builder())
}

func TestSpecBuilderPosition(t *testing.T) {
	b := Spec(func() {
		Version(1)
	}).Builder()

	assert.True(t, b.Version.Reference.Valid)
	assert.Equal(t, "types_test.go", b.Version.Reference.File)
}
