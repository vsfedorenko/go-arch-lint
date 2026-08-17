package check

import (
	"context"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

type (
	projectInfoAssembler interface {
		ProjectInfo(rootDirectory string, archFilePath string) (domain.Project, error)
	}

	specAssembler interface {
		Assemble(prj domain.Project) (arch.Spec, error)
	}

	referenceRender interface {
		SourceCode(ref domain.Reference, highlight bool, showPointer bool) []byte
	}

	specChecker interface {
		Check(ctx context.Context, spec arch.Spec) (models.CheckResult, error)
	}
)
