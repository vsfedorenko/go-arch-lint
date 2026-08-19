package validator

import (
	"fmt"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/spec"
)

type validatorDepsVendors struct {
	utils *utils
}

func newValidatorDepsVendors(
	utils *utils,
) *validatorDepsVendors {
	return &validatorDepsVendors{
		utils: utils,
	}
}

func (v *validatorDepsVendors) Validate(doc spec.Document) []arch.Notice {
	notices := make([]arch.Notice, 0)

	for name, rule := range doc.Dependencies() {
		existVendors := make(map[string]bool)

		for _, vendorName := range rule.Value.CanUse() {
			if _, ok := existVendors[vendorName.Value]; ok {
				notices = append(notices, arch.Notice{
					Notice: fmt.Errorf("vendor '%s' duplicated in '%s' deps", vendorName.Value, name),
					Ref:    vendorName.Reference,
				})
			}

			if err := v.utils.assertKnownVendor(vendorName.Value); err != nil {
				notices = append(notices, arch.Notice{
					Notice: err,
					Ref:    vendorName.Reference,
				})
			}

			existVendors[vendorName.Value] = true
		}
	}

	return notices
}
