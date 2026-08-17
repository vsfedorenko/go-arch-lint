package selfInspect

import (
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

type (
	specAssembler interface {
		Assemble(prj domain.Project) (arch.Spec, error)
	}

	projectInfoAssembler interface {
		ProjectInfo(rootDirectory string, archFilePath string) (domain.Project, error)
	}
)
