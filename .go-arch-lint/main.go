package main

import (
	"os"

	"github.com/vsfedorenko/go-arch-lint/v2"
)

func main() {
	archlint.MustRunCLI(spec, os.Args[1:])
}
