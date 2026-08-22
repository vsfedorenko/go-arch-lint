package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3"
	"github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = dsl.Spec(func(s *dsl.SpecBuilder) {
	domain := s.Path("internal/domain")
	core := s.Path("internal/core", func() { s.Use(domain) })
	http := s.Path("internal/adapter/http", func() { s.Use(core) })
	db := s.Path("internal/adapter/db", func() { s.Use(core, domain) })
	s.Path(".", func() { s.Use(core, db, http) })
})

func main() {
	archlint.MustRun(build, archlint.WithProjectPath("../"))
}
