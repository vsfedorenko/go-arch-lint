package main

import (
	"os"

	"github.com/vsfedorenko/go-arch-lint/v2"
)

func main() {
	archlint.MustRun(spec, archlint.OptionsFromFlags(os.Args[1:])...)
}
