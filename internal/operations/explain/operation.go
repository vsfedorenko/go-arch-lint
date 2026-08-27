package explain

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/services/checker"
)

// maxUsagesShown caps the usage-site list in the output; sites beyond
// the cap are counted in OmittedUsages (mirrors the check command's
// display-cap contract: the cap never changes semantics, only display).
const maxUsagesShown = 20

// Operation implements the `explain` command: dissect how the spec
// treats ONE import path.
type Operation struct {
	specAssembler        specAssembler
	projectInfoAssembler projectInfoAssembler
	projectFilesResolver projectFilesResolver
	stdPackages          map[string]struct{}
}

func NewOperation(
	specAssembler specAssembler,
	projectInfoAssembler projectInfoAssembler,
	projectFilesResolver projectFilesResolver,
	stdPackages map[string]struct{},
) *Operation {
	return &Operation{
		specAssembler:        specAssembler,
		projectInfoAssembler: projectInfoAssembler,
		projectFilesResolver: projectFilesResolver,
		stdPackages:          stdPackages,
	}
}

func (o *Operation) Behave(ctx context.Context, in models.CmdExplainIn) (models.CmdExplainOut, error) {
	projectInfo, err := o.projectInfoAssembler.ProjectInfo(in.ProjectPath, in.ArchFile)
	if err != nil {
		return models.CmdExplainOut{}, models.NewConfigError(
			fmt.Sprintf("failed to assemble project info: %s", err),
		)
	}

	spec, err := o.specAssembler.Assemble(projectInfo)
	if err != nil {
		return models.CmdExplainOut{}, fmt.Errorf("failed to assemble spec: %w", err)
	}

	if len(spec.Integrity.DocumentNotices) > 0 {
		return models.CmdExplainOut{}, models.NewConfigError(
			"arch spec is invalid — run 'go-arch-lint check' to see the notices",
		)
	}

	// Classify exactly like the scanner does: std packages first, then
	// the root module and workspace members (project), everything else
	// is vendor.
	importType := models.GetImportTypeInWorkspace(
		in.ImportPath,
		spec.ModuleName.Value,
		workspaceModulePaths(spec),
		o.stdPackages,
	)

	resolvedImport := models.ResolvedImport{
		Name:       in.ImportPath,
		ImportType: importType,
	}

	out := models.CmdExplainOut{
		ModuleName: spec.ModuleName.Value,
		ImportPath: in.ImportPath,
		ImportType: importTypeName(importType),
		Verdicts:   make([]models.CmdExplainVerdict, 0, len(spec.Components)),
	}

	// The owning component: whose declared paths contain the import
	// (project imports only — a vendor path has no owner by design).
	if importType == models.ImportTypeProject {
		out.OwnerComponent = ownerComponent(spec, in.ImportPath)
	}

	for _, component := range spec.Components {
		verdict, err := checker.VerdictForImport(component, resolvedImport, spec.Allow.DepOnAnyVendor.Value)
		if err != nil {
			return models.CmdExplainOut{}, fmt.Errorf(
				"failed to explain import for component %q: %w", component.Name.Value, err,
			)
		}

		out.Verdicts = append(out.Verdicts, models.CmdExplainVerdict{
			Component: component.Name.Value,
			Allowed:   verdict.Allowed,
			Rule:      verdict.Rule,
			Fix:       verdict.Fix,
		})
	}

	usages, err := o.collectUsages(ctx, spec, in.ImportPath)
	if err != nil {
		return models.CmdExplainOut{}, err
	}

	out.Usages = usages[:min(len(usages), maxUsagesShown)]
	out.OmittedUsages = max(len(usages)-maxUsagesShown, 0)

	return out, nil
}

// collectUsages scans the project for actual import sites of the path
// and reports file, line, and the importing component.
func (o *Operation) collectUsages(ctx context.Context, spec arch.Spec, importPath string) ([]models.CmdExplainUsage, error) {
	projectFiles, err := o.projectFilesResolver.ProjectFiles(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project files: %w", err)
	}

	usages := make([]models.CmdExplainUsage, 0)
	for _, hold := range projectFiles {
		for _, resolvedImport := range hold.File.Imports {
			if resolvedImport.Name != importPath {
				continue
			}

			component := ""
			if hold.ComponentID != nil {
				component = *hold.ComponentID
			}

			usages = append(usages, models.CmdExplainUsage{
				File:      strings.TrimPrefix(hold.File.Path, spec.RootDirectory.Value),
				Line:      resolvedImport.Reference.Line,
				Component: component,
			})
		}
	}

	sort.Slice(usages, func(i, j int) bool {
		if usages[i].File != usages[j].File {
			return usages[i].File < usages[j].File
		}

		return usages[i].Line < usages[j].Line
	})

	return usages, nil
}

// ownerComponent finds the component whose own declared paths contain
// the import path. The assembler puts a component's own paths into its
// AllowedProjectImports alongside the paths of components it Uses; the
// owner is the one whose ResolvedPaths cover the import.
func ownerComponent(spec arch.Spec, importPath string) string {
	for _, component := range spec.Components {
		for _, resolvedPath := range component.ResolvedPaths {
			if resolvedPath.Value.ImportPath == importPath {
				return component.Name.Value
			}
		}
	}

	return ""
}

// workspaceModulePaths extracts workspace member module paths from the
// spec (the scanner consumes domain.WorkspaceModule values the same
// way).
func workspaceModulePaths(spec arch.Spec) []string {
	paths := make([]string, 0, len(spec.WorkspaceModules))
	for _, module := range spec.WorkspaceModules {
		paths = append(paths, module.Path)
	}

	return paths
}

// importTypeName maps the numeric classification to its display name.
func importTypeName(importType models.ImportType) string {
	switch importType {
	case models.ImportTypeStdLib:
		return "std"
	case models.ImportTypeProject:
		return "project"
	case models.ImportTypeVendor:
		return "vendor"
	default:
		return "unknown"
	}
}
