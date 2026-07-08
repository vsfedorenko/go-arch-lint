package allowb

import (
	"github.com/vsfedorenko/go-arch-lint/test/check/project/internal/b"
	"github.com/vsfedorenko/go-arch-lint/test/check/project/internal/common/sub/foo/bar"
)

func AA1() {
	bar.BR1() // allowed common
	b.B1()    // allowed by deps
}
