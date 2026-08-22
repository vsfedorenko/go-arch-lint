package b

import (
	"testing"

	"github.com/vsfedorenko/go-arch-lint/v3/test/check/project/internal/a"
)

func TestB1(t *testing.T) {
	a.A1() // not allowed, but not checked (excluded by regexp)
}
