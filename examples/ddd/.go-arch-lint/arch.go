// Example: Domain-Driven Design (DDD) with bounded contexts
package main

import . "github.com/fe3dback/go-arch-lint/dsl"

var _ = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
	})

	ExcludeFiles(`^.*_test\.go$`)

	// Domain layer: entities and value objects per bounded context
	Component("user-domain", "domain/user")
	Component("order-domain", "domain/order")

	// Application layer: use cases / services
	Component("application", "application")

	// Infrastructure layer: repository implementations, external adapters
	Component("infrastructure", "infrastructure")

	// Interface layer: HTTP handlers, CLI, etc.
	Component("interfaces", "interfaces")

	// User domain entities are shared across contexts
	CommonComponents("user-domain")

	// Order domain can reference user domain
	Deps("order-domain", func() {
		MayDependOn("user-domain")
	})

	// Application services depend on domain entities
	Deps("application", func() {
		MayDependOn("order-domain")
	})

	// Infrastructure implements repository interfaces from application layer
	// and directly references domain entities
	Deps("infrastructure", func() {
		MayDependOn("application", "order-domain")
	})

	// Interface layer calls application services
	Deps("interfaces", func() {
		MayDependOn("application")
	})
})
