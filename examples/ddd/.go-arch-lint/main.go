package main

import (
	"github.com/vsfedorenko/go-arch-lint"
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

var _ = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
	})

	ExcludeFiles(`^.*_test\.go$`)

	Component("user-domain", "domain/user")
	Component("order-domain", "domain/order")
	Component("application", "application")
	Component("infrastructure", "infrastructure")
	Component("interfaces", "interfaces")

	CommonComponents("user-domain")

	Deps("order-domain", func() {
		MayDependOn("user-domain")
	})

	Deps("application", func() {
		MayDependOn("order-domain")
	})

	Deps("infrastructure", func() {
		MayDependOn("application", "order-domain")
	})

	Deps("interfaces", func() {
		MayDependOn("application")
	})
})

func main() {
	archlint.MustRunCLI()
}
