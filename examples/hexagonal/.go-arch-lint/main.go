package main

import (
	"github.com/vsfedorenko/go-arch-lint/v2"
	. "github.com/vsfedorenko/go-arch-lint/v2/dsl"
)

var spec = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
	})

	ExcludeFiles(`^.*_test\.go$`)

	Component("core", "core")
	Component("http", "adapter/http")
	Component("db", "adapter/db")
	Component("domain", "domain")

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

func main() {
	archlint.MustRun(spec)
}
