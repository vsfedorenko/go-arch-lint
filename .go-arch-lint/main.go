package main

import (
	"os"

	"github.com/vsfedorenko/go-arch-lint"
)

func main() {
	archlint.MustRun(spec, archlint.OptionsFromFlags(os.Args[1:])...)
}
