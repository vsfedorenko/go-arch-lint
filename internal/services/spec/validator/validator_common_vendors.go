package validator

import (
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/spec"
)

type validatorCommonVendors struct {
	utils *utils
}

func newValidatorCommonVendors(
	utils *utils,
) *validatorCommonVendors {
	return &validatorCommonVendors{
		utils: utils,
	}
}

func (v *validatorCommonVendors) Validate(doc spec.Document) []arch.Notice {
	notices := make([]arch.Notice, 0)

	for _, vendorName := range doc.CommonVendors() {
		if err := v.utils.assertKnownVendor(vendorName.Value); err != nil {
			notices = append(notices, arch.Notice{
				Notice: err,
				Ref:    vendorName.Reference,
			})
		}
	}

	return notices
}
