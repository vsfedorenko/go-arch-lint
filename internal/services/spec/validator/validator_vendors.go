package validator

import (
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/services/spec"
)

type validatorVendors struct{}

func newValidatorVendors() *validatorVendors {
	return &validatorVendors{}
}

func (v *validatorVendors) Validate(_ spec.Document) []arch.Notice {
	return make([]arch.Notice, 0)
}
