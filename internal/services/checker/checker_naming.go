package checker

import (
	"context"
	"sort"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
)

// Naming asserts packaging-name conventions: no project package may be
// named with a forbidden name (utils, helpers, common, …), no matter which
// component it belongs to. Non-descriptive package names are a widespread
// architectural smell — they become dumping grounds.
type Naming struct {
}

func NewNaming() *Naming {
	return &Naming{}
}

func (c *Naming) Check(ctx context.Context, spec arch.Spec, projectFiles []models.FileHold) (models.CheckResult, error) {
	if spec.Naming == nil || len(spec.Naming.ForbiddenPackages) == 0 {
		return models.CheckResult{}, nil
	}

	forbidden := make(map[string]struct{}, len(spec.Naming.ForbiddenPackages))
	for _, name := range spec.Naming.ForbiddenPackages {
		forbidden[name.Value] = struct{}{}
	}

	// Report each (package, name) violation once — not once per file.
	type violation struct {
		pkgPath string
		pkgName string
		files   []string
	}

	seen := map[string]*violation{}

	for _, projectFile := range projectFiles {
		pkgName := projectFile.File.PackageName
		if pkgName == "" {
			continue
		}

		if _, bad := forbidden[pkgName]; !bad {
			continue
		}

		pkgPath := packagePathOf(projectFile.File.Path)
		key := pkgPath + "|" + pkgName

		if v, exists := seen[key]; exists {
			v.files = append(v.files, projectFile.File.Path)
			continue
		}

		seen[key] = &violation{
			pkgPath: pkgPath,
			pkgName: pkgName,
			files:   []string{projectFile.File.Path},
		}
	}

	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := models.CheckResult{}

	for _, key := range keys {
		v := seen[key]

		sort.Strings(v.files)
		first := v.files[0]

		result.NamingWarnings = append(result.NamingWarnings, models.CheckArchWarningNaming{
			PackageName:      v.pkgName,
			PackagePath:      v.pkgPath,
			FileRelativePath: strings.TrimPrefix(first, spec.RootDirectory.Value),
			FileAbsolutePath: first,
			FilesCount:       len(v.files),
		})
	}

	return result, nil
}

// packagePathOf derives the package directory from a file path.
func packagePathOf(filePath string) string {
	i := strings.LastIndex(filePath, "/")
	if i < 0 {
		return filePath
	}
	return filePath[:i]
}
