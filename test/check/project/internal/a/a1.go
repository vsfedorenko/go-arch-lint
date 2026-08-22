package a

import "github.com/vsfedorenko/go-arch-lint/v3/test/check/project/internal/common"

func A1() {
	common.C1() // common - allowed
}
