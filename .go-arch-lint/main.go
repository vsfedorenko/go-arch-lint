package main

import (
	"os"

	"github.com/vsfedorenko/go-arch-lint/v3"
)

func main() {
	archlint.MustRunCLI(build, os.Args[1:])
}
