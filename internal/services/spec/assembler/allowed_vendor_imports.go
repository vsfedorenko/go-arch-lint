package assembler

import (
	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
	"github.com/vsfedorenko/go-arch-lint/internal/services/spec"
)

type allowedVendorImportsAssembler struct{}

func newAllowedVendorImportsAssembler() *allowedVendorImportsAssembler {
	return &allowedVendorImportsAssembler{}
}

//nolint:unparam // error result is part of the composite assembler contract
func (aia *allowedVendorImportsAssembler) assemble(
	yamlDocument spec.Document,
	vendorNames []string,
) ([]domain.Referable[models.Glob], error) {
	list := make([]domain.Referable[models.Glob], 0)

	allowedVendors := make([]string, 0)
	allowedVendors = append(allowedVendors, vendorNames...)
	for _, vendorName := range yamlDocument.CommonVendors() {
		allowedVendors = append(allowedVendors, vendorName.Value)
	}

	for _, name := range allowedVendors {
		yamlVendor, ok := yamlDocument.Vendors()[name]
		if !ok {
			continue
		}

		for _, vendorIn := range yamlVendor.Value.ImportPaths() {
			list = append(list, domain.NewReferable(vendorIn, yamlVendor.Reference))
		}
	}

	return list, nil
}
