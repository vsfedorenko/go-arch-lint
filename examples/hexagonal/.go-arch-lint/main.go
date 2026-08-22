package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3"
	. "github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = Spec(func() {
	domain := Path("internal/domain")
	core := Path("internal/core", func() { Use(domain) })
	http := Path("internal/adapter/http", func() { Use(core) })
	db := Path("internal/adapter/db", func() { Use(core, domain) })
	Path("cmd", func() {
		Path("app", func() { Use(core, db, http) }) // main собирает всё
	})
})

func main() {
	archlint.MustRun(build, archlint.WithProjectPath("../"))
}
