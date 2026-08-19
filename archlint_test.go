package archlint

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/v2/dsl"
)

func TestRunEmptySpecDefReturnsError(t *testing.T) {
	err := Run(dsl.SpecDef{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}
