package main

import (
	. "github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = Spec(func() {
	Exclude("examples/**", "test/**", "internal/services/checker/deepscan/test/**", "internal/services/checker/cycles/test/**", "internal/services/spec/decoder/test/**")

	errgroup := Vendor("go-common", "golang.org/x/sync/errgroup")
	goAST := Vendor("go-ast", "golang.org/x/mod/modfile", "golang.org/x/tools/go/packages")
	cobra := Vendor("3rd-cobra", "github.com/spf13/cobra")
	aurora := Vendor("3rd-color-fmt", "github.com/logrusorgru/aurora/v3")
	chroma := Vendor("3rd-code-highlight", "github.com/alecthomas/chroma", "github.com/alecthomas/chroma/**")
	gojsonschema := Vendor("3rd-json-scheme", "github.com/xeipuuv/gojsonschema")
	d2 := Vendor("3rd-graph", "oss.terrastruct.com/d2/**")
	yaml := Vendor("3rd-yaml",
		"github.com/goccy/go-yaml", "github.com/goccy/go-yaml/**",
		"github.com/fe3dback/go-yaml", "github.com/fe3dback/go-yaml/**",
	)

	models := Path("internal/models/**")
	dslPkg := Path("dsl/**")

	internal := Path("internal/**", func() {
		Use(models, dslPkg, errgroup, goAST, yaml, aurora, chroma, gojsonschema, d2, cobra)
	})

	Path("cmd/arch-lint", func() {
		Use(internal, cobra)
	})

	Path(".", func() {
		Use(internal, models, dslPkg)
	})
})
