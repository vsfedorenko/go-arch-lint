// Example: Basic layered architecture (handler → service → repository)
package main

import . "github.com/fe3dback/go-arch-lint/dsl"

var _ = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
	})

	ExcludeFiles(`^.*_test\.go$`)

	// Components map Go packages to architectural layers
	Component("handler", "handler")
	Component("service", "service")
	Component("repository", "repository")
	Component("models", "models")

	// Models can be imported by any layer
	CommonComponents("models")

	// Dependency rules: each layer may only depend on the layer below
	Deps("handler", func() {
		MayDependOn("service")
	})

	Deps("service", func() {
		MayDependOn("repository")
	})

	// repository has no deps on other project components (only models, which is common)
})
