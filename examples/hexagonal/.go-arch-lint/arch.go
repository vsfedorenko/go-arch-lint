// Example: Hexagonal architecture (ports & adapters)
package main

import . "github.com/fe3dback/go-arch-lint/dsl"

var _ = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
	})

	ExcludeFiles(`^.*_test\.go$`)

	// Core: business logic with port interfaces
	Component("core", "core")
	// Adapters: infrastructure implementations
	Component("http", "adapter/http")
	Component("db", "adapter/db")
	// Domain: shared entities and value objects
	Component("domain", "domain")

	// Domain entities are shared across all layers
	CommonComponents("domain")

	// Core depends on domain entities
	Deps("core", func() {
		MayDependOn("domain")
	})

	// Adapters depend on core (implement port interfaces)
	Deps("http", func() {
		MayDependOn("core")
	})

	Deps("db", func() {
		MayDependOn("core")
	})
})
