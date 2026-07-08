package archlint

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/dsl"
)

func TestRunNoSpecsReturnsError(t *testing.T) {
	err := Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no specs")
}

func TestRunEmptySpecDefReturnsError(t *testing.T) {
	err := Run(dsl.SpecDef{})
	assert.Error(t, err)
}
