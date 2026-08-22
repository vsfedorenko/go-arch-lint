package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3"
	"github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = dsl.Spec(func(s *dsl.SpecBuilder) {
	user := s.Path("internal/domain/user")
	order := s.Path("internal/domain/order", func() { s.Use(user) })
	application := s.Path("internal/application", func() { s.Use(user, order) })
	infrastructure := s.Path("internal/infrastructure", func() { s.Use(user, order) })
	interfaces := s.Path("internal/interfaces", func() { s.Use(application) })
	s.Path(".", func() { s.Use(application, infrastructure, interfaces) })
})

func main() {
	archlint.MustRun(build, archlint.WithProjectPath("../"))
}
