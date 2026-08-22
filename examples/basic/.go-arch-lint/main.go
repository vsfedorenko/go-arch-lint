package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3"
	. "github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = Spec(func() {
	models := Path("internal/models")
	repository := Path("internal/repository", func() { Use(models) })
	service := Path("internal/service", func() { Use(models, repository) })
	handler := Path("internal/handler", func() { Use(service) })
	Path(".", func() { Use(handler, service, repository) }) // main wires everything
})

func main() {
	archlint.MustRun(build, archlint.WithProjectPath("../"))
}
