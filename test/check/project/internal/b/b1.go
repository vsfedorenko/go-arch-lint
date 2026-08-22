package b

import "github.com/vsfedorenko/go-arch-lint/v3/test/check/project/internal/common/sub/foo/bar"

func B1() {
	bar.BR1() // common - allowed
}
