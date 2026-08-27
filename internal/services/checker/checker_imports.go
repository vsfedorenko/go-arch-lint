package checker

import (
	"context"
	"fmt"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

type Imports struct {
	spec   arch.Spec
	result results
}

func NewImport() *Imports {
	return &Imports{
		result: newResults(),
	}
}

func (c *Imports) Check(ctx context.Context, spec arch.Spec, projectFiles []models.FileHold) (models.CheckResult, error) {
	c.spec = spec

	components := c.assembleComponentsMap(spec)

	for _, projectFile := range projectFiles {
		if projectFile.ComponentID == nil {
			c.result.addNotMatchedWarning(models.CheckArchWarningMatch{
				Reference:        domain.NewEmptyReference(),
				FileRelativePath: strings.TrimPrefix(projectFile.File.Path, spec.RootDirectory.Value),
				FileAbsolutePath: projectFile.File.Path,
			})

			continue
		}

		componentID := *projectFile.ComponentID
		if component, ok := components[componentID]; ok {
			err := c.checkFile(component, projectFile.File)
			if err != nil {
				return models.CheckResult{}, fmt.Errorf("failed check file '%s': %w", projectFile.File.Path, err)
			}

			continue
		}

		return models.CheckResult{}, fmt.Errorf("not found component '%s' in map", componentID)
	}

	return c.result.assembleSortedResults(), nil
}

func (c *Imports) assembleComponentsMap(spec arch.Spec) map[string]arch.Component {
	results := make(map[string]arch.Component)

	for _, component := range spec.Components {
		results[component.Name.Value] = component
	}

	return results
}

func (c *Imports) checkFile(component arch.Component, file models.ProjectFile) error {
	for _, resolvedImport := range file.Imports {
		verdict, err := VerdictForImport(component, resolvedImport, c.spec.Allow.DepOnAnyVendor.Value)
		if err != nil {
			return fmt.Errorf("failed check import '%s': %w",
				resolvedImport.Name,
				err,
			)
		}

		if verdict.Allowed {
			continue
		}

		c.result.addDependencyWarning(models.CheckArchWarningDependency{
			Reference:          resolvedImport.Reference,
			ComponentName:      component.Name.Value,
			FileRelativePath:   strings.TrimPrefix(file.Path, c.spec.RootDirectory.Value),
			FileAbsolutePath:   file.Path,
			ResolvedImportName: resolvedImport.Name,
		})
	}

	return nil
}
