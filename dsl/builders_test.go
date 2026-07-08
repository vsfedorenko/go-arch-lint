package dsl

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/internal/models/common"
)

func TestVersion(t *testing.T) {
	b := Spec(func() {
		Version(1)
	}).Builder()
	assert.Equal(t, 1, b.Version.Value)
	assert.True(t, b.Version.Reference.Valid)
}

func TestWorkdir(t *testing.T) {
	b := Spec(func() {
		Workdir("internal")
	}).Builder()
	assert.Equal(t, "internal", b.Workdir.Value)
}

func TestAllowDepOnAnyVendor(t *testing.T) {
	b := Spec(func() {
		Allow(func() {
			DepOnAnyVendor(false)
		})
	}).Builder()
	assert.Equal(t, false, b.Allow.DepOnAnyVendor.Value)
}

func TestAllowDeepScan(t *testing.T) {
	b := Spec(func() {
		Allow(func() {
			DeepScan(true)
		})
	}).Builder()
	assert.Equal(t, true, b.Allow.DeepScan.Value)
}

func TestAllowIgnoreNotFoundComponents(t *testing.T) {
	b := Spec(func() {
		Allow(func() {
			IgnoreNotFoundComponents(true)
		})
	}).Builder()
	assert.Equal(t, true, b.Allow.IgnoreNotFoundComponents.Value)
}

func TestExclude(t *testing.T) {
	b := Spec(func() {
		Exclude("vendor", "test")
	}).Builder()
	assert.Len(t, b.Exclude, 2)
	assert.Equal(t, "vendor", b.Exclude[0].Value)
	assert.Equal(t, "test", b.Exclude[1].Value)
}

func TestExcludeFiles(t *testing.T) {
	b := Spec(func() {
		ExcludeFiles(`^.*_test\.go$`)
	}).Builder()
	assert.Len(t, b.ExcludeFiles, 1)
	assert.Equal(t, `^.*_test\.go$`, b.ExcludeFiles[0].Value)
}

func TestComponent(t *testing.T) {
	b := Spec(func() {
		Component("main", "app")
	}).Builder()
	assert.Contains(t, b.Components, "main")
	assert.Equal(t, []string{"app"}, b.Components["main"].RelativePaths)
}

func TestComponentMultiplePaths(t *testing.T) {
	b := Spec(func() {
		Component("services", "services/**", "lib/svc")
	}).Builder()
	assert.Equal(t, []string{"services/**", "lib/svc"}, b.Components["services"].RelativePaths)
}

func TestVendor(t *testing.T) {
	b := Spec(func() {
		Vendor("cobra", "github.com/spf13/cobra")
	}).Builder()
	assert.Contains(t, b.Vendors, "cobra")
	assert.Equal(t, []string{"github.com/spf13/cobra"}, b.Vendors["cobra"].ImportPaths)
}

func TestVendorMultiplePaths(t *testing.T) {
	b := Spec(func() {
		Vendor("yaml", "github.com/goccy/go-yaml", "github.com/goccy/go-yaml/**")
	}).Builder()
	assert.Len(t, b.Vendors["yaml"].ImportPaths, 2)
}

func TestCommonComponents(t *testing.T) {
	b := Spec(func() {
		CommonComponents("models", "utils")
	}).Builder()
	assert.Len(t, b.CommonComponents, 2)
	assert.Equal(t, "models", b.CommonComponents[0].Value)
}

func TestCommonVendors(t *testing.T) {
	b := Spec(func() {
		CommonVendors("go-common")
	}).Builder()
	assert.Len(t, b.CommonVendors, 1)
}

func TestDepsMayDependOn(t *testing.T) {
	b := Spec(func() {
		Deps("main", func() {
			MayDependOn("container")
		})
	}).Builder()
	assert.Contains(t, b.Deps, "main")
	assert.Len(t, b.Deps["main"].MayDependOn, 1)
	assert.Equal(t, "container", b.Deps["main"].MayDependOn[0].Value)
}

func TestDepsCanUse(t *testing.T) {
	b := Spec(func() {
		Deps("services", func() {
			CanUse("cobra", "yaml")
		})
	}).Builder()
	assert.Len(t, b.Deps["services"].CanUse, 2)
}

func TestDepsAnyVendorDeps(t *testing.T) {
	b := Spec(func() {
		Deps("container", func() {
			AnyVendorDeps(true)
		})
	}).Builder()
	assert.Equal(t, true, b.Deps["container"].AnyVendorDeps.Value)
}

func TestDepsAnyProjectDeps(t *testing.T) {
	b := Spec(func() {
		Deps("main", func() {
			AnyProjectDeps(true)
		})
	}).Builder()
	assert.Equal(t, true, b.Deps["main"].AnyProjectDeps.Value)
}

func TestDepsDeepScanOverride(t *testing.T) {
	b := Spec(func() {
		Deps("operations", func() {
			DeepScan(false)
		})
	}).Builder()
	assert.Equal(t, false, b.Deps["operations"].DeepScan.Value)
}

func TestBuilderPositionsAreFromTestFile(t *testing.T) {
	b := Spec(func() {
		Component("main", "app") // this line should be the reference
	}).Builder()
	ref := b.Components["main"].Reference
	assert.True(t, ref.Valid)
	assert.Equal(t, "builders_test.go", ref.File)
	assert.Greater(t, ref.Line, 0)
}

func TestMultipleDeps(t *testing.T) {
	b := Spec(func() {
		Deps("main", func() { MayDependOn("a") })
		Deps("container", func() { MayDependOn("b") })
	}).Builder()
	assert.Len(t, b.Deps, 2)
	assert.Contains(t, b.Deps, "main")
	assert.Contains(t, b.Deps, "container")
}

// Ensure unused import doesn't cause issues
var _ = common.NewEmptyReferable[int]
