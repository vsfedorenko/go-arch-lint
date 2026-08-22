package selfInspect

import (
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

type (
	specAssembler interface {
		Assemble(prj domain.Project) (arch.Spec, error)
	}

	projectInfoAssembler interface {
		ProjectInfo(rootDirectory string, archFilePath string) (domain.Project, error)
	}
)
