package c

import "github.com/vsfedorenko/go-arch-lint/v3/test/check/project/internal/a"

func C1() {
	a.A1() // not allowed
}
