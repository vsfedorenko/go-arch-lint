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

	Component("handler", "handler")
	Component("service", "service")
	Component("repository", "repository")
	Component("models", "models")

	CommonComponents("models")

	Deps("handler", func() {
		MayDependOn("service")
	})

	Deps("service", func() {
		MayDependOn("repository")
	})
})

func main() {
	archlint.MustRun(spec)
}
