package dsl_test

import (
	"testing"

	"github.com/vsfedorenko/go-arch-lint/dsl"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression: calling any builder AFTER a nested Spec() crashed with a
// raw segmentation fault (the nested Spec reset the package-level
// context; the builder then dereferenced a nil *SpecBuilder). Found by
// synthetic DSL probing. The guard must turn every such call into an
// actionable panic naming the problem.
func TestBuildersAfterNestedSpec_PanicWithMessage(t *testing.T) {
	callAfterNested := func(f func()) (panicMsg string) {
		defer func() {
			if r := recover(); r != nil {
				panicMsg = r.(error).Error()
			}
		}()
		dsl.Spec(func() {
			dsl.Spec(func() { dsl.Version(1) })
			f()
		})
		return ""
	}

	for name, fn := range map[string]func(){
		"Version":      func() { dsl.Version(1) },
		"Workdir":      func() { dsl.Workdir("x") },
		"Component":    func() { dsl.Component("a", "x") },
		"Vendor":       func() { dsl.Vendor("v", "p") },
		"Deps":         func() { dsl.Deps("a", func() {}) },
		"Common":       func() { dsl.CommonComponents("a") },
		"Exclude":      func() { dsl.Exclude("x") },
		"ExcludeFiles": func() { dsl.ExcludeFiles("x") },
		"Tier":         func() { dsl.Tier("t", "a") },
		"Tiers":        func() { dsl.Tiers("t") },
		"Naming":       func() { dsl.Naming(func() {}) },
		"Interfaces":   func() { dsl.Interfaces(func() {}) },
		"Visibility":   func() { dsl.Visibility(func() {}) },
		"Allow":        func() { dsl.Allow(func() {}) },
	} {
		t.Run(name, func(t *testing.T) {
			msg := callAfterNested(fn)
			require.NotEqual(t, "", msg, "expected a panic with a message, got none")
			assert.Contains(t, msg, "inside Spec(func(){...})", "panic must be actionable, got: %s")
		})
	}
}

// The guard must NOT fire for the normal, nested-callback builders:
// Deps(...) callbacks, Allow(...) callbacks etc. run while the spec
// context is alive.
func TestNestedCallbacks_StillWork(t *testing.T) {
	spec := dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("handler", "handlers/*")
		dsl.Allow(func() { dsl.DepOnAnyVendor(false) })
		dsl.Deps("handler", func() {
			dsl.MayDependOn("service")
			dsl.AnyVendorDeps(true)
		})
	})
	b := spec.Builder()
	require.NotNil(t, b, "normal nesting broke")
	require.Len(t, b.Components, 1, "normal nesting broke: %+v", b)
	require.Len(t, b.Deps, 1, "normal nesting broke: %+v", b)

	got := b.Deps["handler"].MayDependOn
	require.Len(t, got, 1, "MayDependOn lost: %+v", got)
	assert.Equal(t, "service", got[0].Value, "MayDependOn lost: %+v", got)
}
