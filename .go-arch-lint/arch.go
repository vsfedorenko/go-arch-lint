package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = dsl.Spec(func(s *dsl.SpecBuilder) {
	s.Exclude("examples/**", "test/**", "internal/services/checker/deepscan/test/**", "internal/services/checker/cycles/test/**", "internal/services/spec/decoder/test/**")

	errgroup := s.Vendor("go-common", "golang.org/x/sync/errgroup")
	goAST := s.Vendor("go-ast", "golang.org/x/mod/modfile", "golang.org/x/tools/go/packages")
	cobra := s.Vendor("3rd-cobra", "github.com/spf13/cobra")
	aurora := s.Vendor("3rd-color-fmt", "github.com/logrusorgru/aurora/v3")
	chroma := s.Vendor("3rd-code-highlight", "github.com/alecthomas/chroma", "github.com/alecthomas/chroma/**")
	gojsonschema := s.Vendor("3rd-json-scheme", "github.com/xeipuuv/gojsonschema")
	d2 := s.Vendor("3rd-graph", "oss.terrastruct.com/d2/**")
	yaml := s.Vendor("3rd-yaml",
		"github.com/goccy/go-yaml", "github.com/goccy/go-yaml/**",
		"github.com/fe3dback/go-yaml", "github.com/fe3dback/go-yaml/**",
	)

	models := s.Path("internal/models/**")
	dslPkg := s.Path("dsl/**")

	internal := s.Path("internal/**", func() {
		s.Use(models, dslPkg, errgroup, goAST, yaml, aurora, chroma, gojsonschema, d2, cobra)
	})

	s.Path("cmd/arch-lint", func() {
		s.Use(internal, cobra)
	})

	s.Path(".", func() {
		s.Use(internal, models, dslPkg)
	})
})
