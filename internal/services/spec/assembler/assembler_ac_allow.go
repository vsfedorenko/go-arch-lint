package assembler

import (
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/services/spec"
)

type allowAssembler struct{}

func newAllowAssembler() *allowAssembler {
	return &allowAssembler{}
}

func (efa *allowAssembler) assemble(spec *arch.Spec, document spec.Document) error {
	spec.Allow = arch.Allow{
		DepOnAnyVendor:           document.Options().IsDependOnAnyVendor(),
		DeepScan:                 document.Options().DeepScan(),
		IgnoreNotFoundComponents: document.Options().IgnoreNotFoundComponents(),
	}

	return nil
}
