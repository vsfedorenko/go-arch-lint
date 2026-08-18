package render

import (
	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

// Shared fixture values for the format tests (json/sarif/junit/
// github-actions): one module, one component, one forbidden import.
// goconst keeps the suite honest about repeated literals.
const (
	gaFixtureModule    = "github.com/x/proj"
	gaFixtureComponent = "handler"
	gaFixtureImport    = gaFixtureModule + "/internal/repository"
)

// gaViolationFixture builds the canonical single-dependency-violation
// CmdCheckOut every github-actions test starts from.
func gaViolationFixture() models.CmdCheckOut {
	return models.CmdCheckOut{
		ModuleName: gaFixtureModule,
		ArchWarningsDependency: []models.CheckArchWarningDependency{
			{
				ComponentName:      gaFixtureComponent,
				FileRelativePath:   "/internal/handler/user.go",
				ResolvedImportName: gaFixtureImport,
				Reference:          domain.NewReferenceSingleLine("/internal/handler/user.go", 10, 2),
			},
		},
	}
}
