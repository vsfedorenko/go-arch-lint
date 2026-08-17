package checker

import (
	"context"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

type (
	projectFilesResolver interface {
		ProjectFiles(ctx context.Context, spec arch.Spec) ([]models.FileHold, error)
	}

	checker interface {
		Check(ctx context.Context, spec arch.Spec) (models.CheckResult, error)
	}

	sourceCodeRenderer interface {
		SourceCode(ref domain.Reference, highlight bool, showPointer bool) []byte
	}
)
