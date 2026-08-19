package assembler

import (
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/spec"
)

type workdirAssembler struct{}

func newWorkdirAssembler() *workdirAssembler {
	return &workdirAssembler{}
}

func (efa *workdirAssembler) assemble(spec *arch.Spec, document spec.Document) error {
	spec.WorkingDirectory = document.WorkingDirectory()

	return nil
}
