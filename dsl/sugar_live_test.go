package dsl

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tcDomainPath = "internal/domain"

// Package-level sugar: dot-import-style Path/Use/Vendor/Exclude routed to
// the Spec running on the same goroutine. These tests pin the sugar
// contracts — same build result as the method form, same panics, and
// parallel Specs isolated per goroutine.

// The sugar form must produce the identical Build the method form does.
func TestSugar_Canonical(t *testing.T) {
	b := Spec(func() {
		domain := Path(tcDomainPath)
		pgx := Vendor("pgx", "github.com/jackc/pgx/v5")

		Path("internal/core", func() { Use(domain, pgx) })
	})

	assert.Equal(t, []string{tcDomainPath, "internal/core"}, b.Order, "Order must match the method form")

	core, ok := b.Uses["internal/core"]
	require.True(t, ok, "core use rule missing")
	assert.Equal(t, []string{tcDomainPath}, core.Paths, "core path targets")
	assert.Equal(t, []string{"pgx"}, core.Vends, "core vendor targets")
}

// Nested package-level Paths join the parent prefix exactly like the
// method form.
func TestSugar_Nesting(t *testing.T) {
	b := Spec(func() {
		domain := Path(tcDomainPath)

		Path("cmd", func() {
			Path("app", func() { Use(domain) })
		})
	})

	assert.Equal(t, []string{tcDomainPath, "cmd", "cmd/app"}, b.Order, "nested paths must join prefixes (intermediate dirs declared too)")
	_, ok := b.Uses["cmd/app"]
	assert.True(t, ok, "use inside the nested path fn must attach to the leaf")
}

// Mixing both styles in one Spec is legal: the sugar routes to the same
// builder the methods use.
func TestSugar_MixedForms(t *testing.T) {
	b := Spec(func(s *SpecBuilder) {
		domain := s.Path(tcDomainPath)

		Path("internal/core", func() { Use(domain) })
	})

	assert.Equal(t, []string{tcDomainPath, "internal/core"}, b.Order, "mixed styles must share one builder")
}

// Any package-level call outside Spec must die with file:line, not
// silently no-op or corrupt a global.
func TestSugar_OutsideSpecPanics(t *testing.T) {
	msg := "package-level DSL calls work only inside Spec(func(){...})"
	assert.Panics(t, func() { _ = Path("x") }, "Path outside Spec must panic")
	// (panic text carries the caller file:line; asserted via a captured run below)
	func() {
		defer func() {
			r := recover()
			require.NotNil(t, r, "panic expected")
			assert.Contains(t, fmt.Sprint(r), msg, "panic must explain the Spec requirement")
			assert.Contains(t, fmt.Sprint(r), "sugar_live_test.go:", "panic must carry file:line")
		}()
		_ = Path("x")
	}()
}

// A spec fn of any other shape is a build-time panic, not a silent skip.
func TestSpec_RejectedFnShape(t *testing.T) {
	assert.Panics(t, func() { _ = Spec(func(n int) {}) },
		"Spec must reject fn shapes other than func() and func(s *SpecBuilder)")
}

// Parallel Specs on separate goroutines must never see each other's
// builders — the goroutine-keyed routing is what makes the package-level
// form sound under concurrency.
func TestSugar_ParallelSpecs(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			b := Spec(func() {
				dep := Path(fmt.Sprintf("dep%d", n))
				for j := range 16 {
					Path(fmt.Sprintf("pkg%d/leaf%d", n, j), func() { Use(dep) })
				}
			})
			assert.Len(t, b.Order, 17, "spec %d: foreign paths leaked in", n)
		}(i)
	}
	wg.Wait()
}
