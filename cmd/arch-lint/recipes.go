package main

// Recipe starter specs for `go-arch-lint init --recipe <name>`.
//
// Each recipe writes the standard scaffold (go.mod, arch.go, main.go)
// with arch.go pre-filled for a well-known architecture pattern. Every
// recipe sets IgnoreNotFoundComponents(true) inside Allow: layer
// directories can be created gradually without the linter failing on a
// not-yet-existing layer.
//
// The hexagonal recipe is pinned byte-for-byte by the black-box drift
// guard in test/check/integration_recipe_test.go
// (TestRecipeSpecMatchesLauncher) — update both together.

// recipe is a named starter spec.
type recipe struct {
	desc   string // one-line description for help output
	archGo string // the arch.go body (package clause + spec)
}

// knownRecipes lists every recipe the launcher accepts. Keep the names
// short and lowercase; they double as CLI values.
var knownRecipes = map[string]recipe{
	"clean": {
		desc: "clean architecture: delivery -> usecase -> domain",
		archGo: `package main

import (
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

// Clean architecture: the domain layer depends on nothing; use cases
// depend on the domain; delivery mechanisms (http, cli, grpc) depend on
// use cases and the domain.
var spec = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
		// Directories that do not exist yet are fine while you build the
		// project out; remove this once every component has code.
		IgnoreNotFoundComponents(true)
	})

	ExcludeFiles(` + "`^.*_test\\.go$`" + `)

	Component("domain", "domain")
	Component("usecase", "usecase")
	Component("delivery", "delivery")

	CommonComponents("domain")

	Deps("usecase", func() {
		MayDependOn("domain")
	})

	Deps("delivery", func() {
		MayDependOn("usecase", "domain")
	})
})
`,
	},
	"hexagonal": {
		desc: "ports and adapters: domain+core at the center, http/db depend inward",
		archGo: `package main

import (
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

// Ports & adapters: the domain and core logic sit at the center; HTTP and DB
// adapters depend inward on core, never on each other.
var spec = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
		// Directories that do not exist yet are fine while you build the
		// project out; remove this once every component has code.
		IgnoreNotFoundComponents(true)
	})

	ExcludeFiles(` + "`^.*_test\\.go$`" + `)

	Component("domain", "domain")
	Component("core", "core")
	Component("http", "adapter/http")
	Component("db", "adapter/db")

	CommonComponents("domain")

	Deps("core", func() {
		MayDependOn("domain")
	})

	Deps("http", func() {
		MayDependOn("core")
	})

	Deps("db", func() {
		MayDependOn("core")
	})
})
`,
	},
	"ddd": {
		desc: "DDD: bounded contexts with application/domain/infrastructure/interfaces",
		archGo: `package main

import (
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

// Domain-Driven Design: bounded contexts under contexts/, each split
// into the classic layers. Interfaces may reach application and domain;
// application and infrastructure may reach domain; nothing reaches
// infrastructure from the inside.
var spec = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
		// Directories that do not exist yet are fine while you build the
		// project out; remove this once every component has code.
		IgnoreNotFoundComponents(true)
	})

	ExcludeFiles(` + "`^.*_test\\.go$`" + `)

	Component("interfaces", "contexts/*/interfaces")
	Component("application", "contexts/*/application")
	Component("domain", "contexts/*/domain")
	Component("infrastructure", "contexts/*/infrastructure")

	CommonComponents("domain")

	Deps("interfaces", func() {
		MayDependOn("application", "domain")
	})

	Deps("application", func() {
		MayDependOn("domain")
	})

	Deps("infrastructure", func() {
		MayDependOn("domain")
	})
})
`,
	},
}

// recipeArchGo returns the arch.go body for a recipe name.
func recipeArchGo(name string) (string, bool) {
	r, ok := knownRecipes[name]
	if !ok {
		return "", false
	}
	return r.archGo, true
}
