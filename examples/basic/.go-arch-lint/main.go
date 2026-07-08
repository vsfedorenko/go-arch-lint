package main

import (
	"github.com/fe3dback/go-arch-lint"
	. "github.com/fe3dback/go-arch-lint/dsl"
)

var _ = Spec(func() {
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
	archlint.MustRunCLI()
}
