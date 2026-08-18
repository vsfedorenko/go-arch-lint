package dsl_test

import (
	"strings"
	"testing"

	"github.com/vsfedorenko/go-arch-lint/dsl"
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
			if msg == "" {
				t.Fatal("expected a panic with a message, got none")
			}
			if !strings.Contains(msg, "inside Spec(func(){...})") {
				t.Fatalf("panic must be actionable, got: %s", msg)
			}
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
	if b == nil || len(b.Components) != 1 || len(b.Deps) != 1 {
		t.Fatalf("normal nesting broke: %+v", b)
	}
	if got := b.Deps["handler"].MayDependOn; len(got) != 1 || got[0].Value != "service" {
		t.Fatalf("MayDependOn lost: %+v", got)
	}
}
