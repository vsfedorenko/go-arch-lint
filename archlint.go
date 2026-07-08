package archlint

import (
	"fmt"
	"os"

	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/app"
)

func Run(specs ...dsl.SpecDef) error {
	if len(specs) == 0 {
		return fmt.Errorf("no specs provided — ensure Spec() was called")
	}
	merged := dsl.MergeSpecs(specs...)
	if merged == nil {
		return fmt.Errorf("all specs are empty — ensure Spec() populated the builder")
	}
	dsl.SetSpec(merged)
	if code := app.Execute(); code != 0 {
		return fmt.Errorf("arch-lint exited with code %d", code)
	}
	return nil
}

func MustRun(specs ...dsl.SpecDef) {
	if err := Run(specs...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
