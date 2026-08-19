package b

import "github.com/vsfedorenko/go-arch-lint/v2/test/check/project/internal/common/sub/foo/bar"

func B1() {
	bar.BR1() // common - allowed
}
