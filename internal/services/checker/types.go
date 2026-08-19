package checker

import (
	"context"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

type (
	projectFilesResolver interface {
		ProjectFiles(ctx context.Context, spec arch.Spec) ([]models.FileHold, error)
	}

	// checker inspects already-resolved project files. The composite
	// resolves the file list ONCE and hands it to every checker —
	// checkers own their analysis, not project scanning.
	checker interface {
		Check(ctx context.Context, spec arch.Spec, projectFiles []models.FileHold) (models.CheckResult, error)
	}

	sourceCodeRenderer interface {
		SourceCode(ref domain.Reference, highlight bool, showPointer bool) []byte
	}
)
