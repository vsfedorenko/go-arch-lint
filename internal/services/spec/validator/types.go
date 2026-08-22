package validator

import (
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/services/spec"
)

type (
	validator interface {
		Validate(doc spec.Document) []arch.Notice
	}

	pathResolver interface {
		Resolve(absPath string) (resolvePaths []string, err error)
	}
)
