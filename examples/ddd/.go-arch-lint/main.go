package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3"
	. "github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = Spec(func() {
	user := Path("internal/domain/user")
	order := Path("internal/domain/order", func() { Use(user) })
	application := Path("internal/application", func() { Use(user, order) })
	infrastructure := Path("internal/infrastructure", func() { Use(user, order) })
	interfaces := Path("internal/interfaces", func() { Use(application) })
	Path(".", func() { Use(application, infrastructure, interfaces) })
})

func main() {
	archlint.MustRun(build, archlint.WithProjectPath("../"))
}
