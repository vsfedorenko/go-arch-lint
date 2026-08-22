package main

import (
	. "github.com/vsfedorenko/go-arch-lint/v2/dsl"
)

var spec = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
	})

	ExcludeFiles(`^.*_test\.go$`)
	ExcludeFiles(`^.*\/test\/.*$`)

	Vendor("go-common", "golang.org/x/sync/errgroup")
	Vendor("go-ast", "golang.org/x/mod/modfile", "golang.org/x/tools/go/packages")
	Vendor("3rd-cobra", "github.com/spf13/cobra")
	Vendor("3rd-color-fmt", "github.com/logrusorgru/aurora/v3")
	Vendor("3rd-code-highlight", "github.com/alecthomas/chroma", "github.com/alecthomas/chroma/**")
	Vendor("3rd-json-scheme", "github.com/xeipuuv/gojsonschema")
	Vendor("3rd-graph", "oss.terrastruct.com/d2/**")
	Vendor("3rd-yaml",
		"github.com/goccy/go-yaml",
		"github.com/goccy/go-yaml/**",
		"github.com/fe3dback/go-yaml",
		"github.com/fe3dback/go-yaml/**",
	)

	Component("main", "app")
	Component("container", "app/container/**")
	Component("operations", "operations/*")
	Component("services", "services/**")
	Component("view", "view")
	Component("models", "models/**")
	Component("dsl", "../dsl", "../dsl/v2")

	CommonVendors("go-common")
	CommonComponents("models")

	Naming(func() {
		ForbiddenPackages("utils", "helpers", "common", "misc", "stuff")
	})

	Interfaces(func() {
		MustLiveWithConsumer()
	})

	Visibility(func() {
		// view and operations are container-internal plumbing: only the
		// glue layer may consume them.
		VisibleTo("view", "container")
		VisibleTo("operations", "container")
	})

	Tiers("cmd", "glue", "operations", "services", "view")
	Tier("cmd", "main")
	Tier("glue", "container")
	Tier("operations", "operations")
	Tier("services", "services")
	Tier("view", "view")

	Deps("main", func() {
		MayDependOn("container")
	})

	Deps("container", func() {
		AnyVendorDeps(true)
		MayDependOn("operations", "services", "view")
	})

	Deps("operations", func() {
		MayDependOn("services")
		CanUse("3rd-graph")
	})

	Deps("services", func() {
		MayDependOn("services", "dsl")
		CanUse("go-ast", "3rd-yaml", "3rd-color-fmt", "3rd-code-highlight", "3rd-json-scheme")
	})
})
