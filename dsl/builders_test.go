package dsl

import (
	"testing"

	"github.com/fe3dback/go-arch-lint/internal/models/common"
	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Version(1)
	})
	b, _ := FlushSpec()
	assert.Equal(t, 1, b.Version.Value)
	assert.True(t, b.Version.Reference.Valid)
}

func TestWorkdir(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Workdir("internal")
	})
	b, _ := FlushSpec()
	assert.Equal(t, "internal", b.Workdir.Value)
}

func TestAllowDepOnAnyVendor(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Allow(func() {
			DepOnAnyVendor(false)
		})
	})
	b, _ := FlushSpec()
	assert.Equal(t, false, b.Allow.DepOnAnyVendor.Value)
}

func TestAllowDeepScan(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Allow(func() {
			DeepScan(true)
		})
	})
	b, _ := FlushSpec()
	assert.Equal(t, true, b.Allow.DeepScan.Value)
}

func TestAllowIgnoreNotFoundComponents(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Allow(func() {
			IgnoreNotFoundComponents(true)
		})
	})
	b, _ := FlushSpec()
	assert.Equal(t, true, b.Allow.IgnoreNotFoundComponents.Value)
}

func TestExclude(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Exclude("vendor", "test")
	})
	b, _ := FlushSpec()
	assert.Len(t, b.Exclude, 2)
	assert.Equal(t, "vendor", b.Exclude[0].Value)
	assert.Equal(t, "test", b.Exclude[1].Value)
}

func TestExcludeFiles(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		ExcludeFiles(`^.*_test\.go$`)
	})
	b, _ := FlushSpec()
	assert.Len(t, b.ExcludeFiles, 1)
	assert.Equal(t, `^.*_test\.go$`, b.ExcludeFiles[0].Value)
}

func TestComponent(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Component("main", "app")
	})
	b, _ := FlushSpec()
	assert.Contains(t, b.Components, "main")
	assert.Equal(t, []string{"app"}, b.Components["main"].RelativePaths)
}

func TestComponentMultiplePaths(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Component("services", "services/**", "lib/svc")
	})
	b, _ := FlushSpec()
	assert.Equal(t, []string{"services/**", "lib/svc"}, b.Components["services"].RelativePaths)
}

func TestVendor(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Vendor("cobra", "github.com/spf13/cobra")
	})
	b, _ := FlushSpec()
	assert.Contains(t, b.Vendors, "cobra")
	assert.Equal(t, []string{"github.com/spf13/cobra"}, b.Vendors["cobra"].ImportPaths)
}

func TestVendorMultiplePaths(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Vendor("yaml", "github.com/goccy/go-yaml", "github.com/goccy/go-yaml/**")
	})
	b, _ := FlushSpec()
	assert.Len(t, b.Vendors["yaml"].ImportPaths, 2)
}

func TestCommonComponents(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		CommonComponents("models", "utils")
	})
	b, _ := FlushSpec()
	assert.Len(t, b.CommonComponents, 2)
	assert.Equal(t, "models", b.CommonComponents[0].Value)
}

func TestCommonVendors(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		CommonVendors("go-common")
	})
	b, _ := FlushSpec()
	assert.Len(t, b.CommonVendors, 1)
}

func TestDepsMayDependOn(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Deps("main", func() {
			MayDependOn("container")
		})
	})
	b, _ := FlushSpec()
	assert.Contains(t, b.Deps, "main")
	assert.Len(t, b.Deps["main"].MayDependOn, 1)
	assert.Equal(t, "container", b.Deps["main"].MayDependOn[0].Value)
}

func TestDepsCanUse(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Deps("services", func() {
			CanUse("cobra", "yaml")
		})
	})
	b, _ := FlushSpec()
	assert.Len(t, b.Deps["services"].CanUse, 2)
}

func TestDepsAnyVendorDeps(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Deps("container", func() {
			AnyVendorDeps(true)
		})
	})
	b, _ := FlushSpec()
	assert.Equal(t, true, b.Deps["container"].AnyVendorDeps.Value)
}

func TestDepsAnyProjectDeps(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Deps("main", func() {
			AnyProjectDeps(true)
		})
	})
	b, _ := FlushSpec()
	assert.Equal(t, true, b.Deps["main"].AnyProjectDeps.Value)
}

func TestDepsDeepScanOverride(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Deps("operations", func() {
			DeepScan(false)
		})
	})
	b, _ := FlushSpec()
	assert.Equal(t, false, b.Deps["operations"].DeepScan.Value)
}

func TestBuilderPositionsAreFromTestFile(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Component("main", "app") // this line should be the reference
	})
	b, _ := FlushSpec()
	ref := b.Components["main"].Reference
	assert.True(t, ref.Valid)
	assert.Equal(t, "builders_test.go", ref.File)
	assert.Greater(t, ref.Line, 0)
}

func TestMultipleDeps(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Deps("main", func() { MayDependOn("a") })
		Deps("container", func() { MayDependOn("b") })
	})
	b, _ := FlushSpec()
	assert.Len(t, b.Deps, 2)
	assert.Contains(t, b.Deps, "main")
	assert.Contains(t, b.Deps, "container")
}

// Ensure unused import doesn't cause issues
var _ = common.NewEmptyReferable[int]
