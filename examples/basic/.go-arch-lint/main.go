package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3"
	"github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = dsl.Spec(func(s *dsl.SpecBuilder) {
	models := s.Path("internal/models")
	repository := s.Path("internal/repository", func() { s.Use(models) })
	service := s.Path("internal/service", func() { s.Use(models, repository) })
	handler := s.Path("internal/handler", func() { s.Use(service) })
	s.Path(".", func() { s.Use(handler, service, repository) }) // main wires everything
})

func main() {
	archlint.MustRun(build, archlint.WithProjectPath("../"))
}
