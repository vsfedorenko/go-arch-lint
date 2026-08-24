package assembler

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

type resolver struct {
	pathResolver  pathResolver
	rootDirectory string
	moduleName    string

	// workspaceModules are the go.work sibling modules: when a resolved
	// path lives inside one of them, the import path is built from THAT
	// module's path, not from the root module.
	workspaceModules []domain.WorkspaceModule
}

func newResolver(
	pathResolver pathResolver,
	rootDirectory string,
	moduleName string,
	workspaceModules []domain.WorkspaceModule,
) *resolver {
	return &resolver{
		pathResolver:     pathResolver,
		rootDirectory:    rootDirectory,
		moduleName:       moduleName,
		workspaceModules: workspaceModules,
	}
}

// owningModule returns the module the absDir belongs to: the go.work member
// with the longest matching directory prefix, or nil for the root module.
func (r *resolver) owningModule(absDir string) *domain.WorkspaceModule {
	var best *domain.WorkspaceModule
	for i := range r.workspaceModules {
		module := &r.workspaceModules[i]
		moduleDir := filepath.Clean(module.Dir)
		if absDir != moduleDir && !strings.HasPrefix(absDir, moduleDir+string(filepath.Separator)) {
			continue
		}
		if best == nil || len(moduleDir) > len(filepath.Clean(best.Dir)) {
			best = module
		}
	}
	return best
}

func (r *resolver) resolveLocalGlobPath(localGlobPath string) ([]models.ResolvedPath, error) {
	list := make([]models.ResolvedPath, 0)

	absPath := fmt.Sprintf("%s/%s", r.rootDirectory, localGlobPath)
	resolved, err := r.pathResolver.Resolve(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path '%s'", absPath)
	}

	for _, absResolvedPath := range resolved {
		localPath := strings.TrimPrefix(absResolvedPath, fmt.Sprintf("%s/", r.rootDirectory))
		localPath = strings.TrimRight(localPath, "/")

		importPath := fmt.Sprintf("%s/%s", r.moduleName, localPath)
		if module := r.owningModule(absResolvedPath); module != nil {
			// A go.work member: the import path is declared by the member's
			// own go.mod, so rebuild it relative to the module directory.
			moduleLocal := strings.TrimPrefix(absResolvedPath, fmt.Sprintf("%s/", filepath.Clean(module.Dir)))
			importPath = fmt.Sprintf("%s/%s", module.Path, moduleLocal)
		}

		list = append(list, models.ResolvedPath{
			ImportPath: strings.TrimRight(importPath, "/"),
			LocalPath:  strings.TrimRight(localPath, "/"),
			AbsPath:    filepath.Clean(strings.TrimRight(absResolvedPath, "/")),
		})
	}

	return list, nil
}
