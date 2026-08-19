package c

import "github.com/vsfedorenko/go-arch-lint/v2/test/check/project/internal/a"

func C1() {
	a.A1() // not allowed
}
