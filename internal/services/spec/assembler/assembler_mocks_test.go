package assembler

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

// The assembler drives the whole spec pipeline through three ports
// (archDecoder, archValidator, pathResolver). Mockery-generated mocks pin
// the orchestration contracts that were previously untested (0% coverage).

func newTestProject() domain.Project {
	return domain.Project{
		Directory:      "/proj",
		ModuleName:     "example.com/proj",
		GoArchFilePath: "/proj/.go-arch-lint/arch.go",
	}
}

func TestAssemble_DecodeErrorPropagates(t *testing.T) {
	decoder := newMockarchDecoder(t)
	decoder.EXPECT().Decode("/proj/.go-arch-lint/arch.go").
		Return(nil, nil, errors.New("boom"))

	sa := NewAssembler(decoder, nil, nil)
	_, err := sa.Assemble(newTestProject())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode document")
	assert.Contains(t, err.Error(), "boom")
}

func TestAssemble_SchemeNoticesShortCircuitValidator(t *testing.T) {
	// When the decoder reports scheme notices, the validator must NOT run:
	// the document shape is already known-bad.
	decoder := newMockarchDecoder(t)
	decoder.EXPECT().Decode("/proj/.go-arch-lint/arch.go").
		Return(nil, []arch.Notice{{Notice: errors.New("bad scheme")}}, nil)

	validator := newMockarchValidator(t)
	// no EXPECT() on validator: any call fails the test

	sa := NewAssembler(decoder, validator, nil)
	spec, err := sa.Assemble(newTestProject())
	require.NoError(t, err)
	assert.Len(t, spec.Integrity.DocumentNotices, 1, "scheme notice must surface")
}

func TestAssemble_NilDocumentStopsAfterValidation(t *testing.T) {
	decoder := newMockarchDecoder(t)
	decoder.EXPECT().Decode("/proj/.go-arch-lint/arch.go").
		Return(nil, []arch.Notice{}, nil)

	validator := newMockarchValidator(t)
	validator.EXPECT().Validate(nil, "/proj").Return(nil)

	sa := NewAssembler(decoder, validator, nil)
	spec, err := sa.Assemble(newTestProject())
	require.NoError(t, err)
	assert.NotNil(t, spec, "a spec is always returned, even with no document")
}
