package resolver

import (
	"context"
	"regexp"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

type (
	projectFilesResolver interface {
		Scan(
			ctx context.Context,
			projectDirectory string,
			moduleName string,
			excludePaths []models.ResolvedPath,
			excludeFileMatchers []*regexp.Regexp,
		) ([]models.ProjectFile, error)

		ScanInWorkspace(
			ctx context.Context,
			workspaceModules []domain.WorkspaceModule,
			projectDirectory string,
			moduleName string,
			excludePaths []models.ResolvedPath,
			excludeFileMatchers []*regexp.Regexp,
		) ([]models.ProjectFile, error)
	}

	projectFilesHolder interface {
		HoldProjectFiles(files []models.ProjectFile, components []arch.Component) []models.FileHold
	}
)
