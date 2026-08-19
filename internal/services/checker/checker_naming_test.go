package checker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

/**
 * Naming checker tests. Files carry a PackageName (as the scanner now
 * captures it) and the checker flags forbidden package names once per
 * package, not per file.
 */

func namingFile(pkgName, relPath string) models.FileHold {
	component := tcAlpha
	file := models.ProjectFile{
		Path:        "/project/" + relPath,
		PackageName: pkgName,
		Imports:     []models.ResolvedImport{},
	}
	return models.FileHold{File: file, ComponentID: &component}
}

func namingSpec(forbidden ...string) arch.Spec {
	spec := cyclesSpec(map[string][]string{
		tcAlpha: {tcPkgAlpha},
	})
	spec.RootDirectory = domain.NewReferable("/project", domain.NewEmptyReference())

	if len(forbidden) > 0 {
		entries := make([]domain.Referable[string], len(forbidden))
		for i, name := range forbidden {
			entries[i] = domain.NewReferable(name, domain.NewEmptyReference())
		}
		spec.Naming = &arch.Naming{ForbiddenPackages: entries}
	}

	return spec
}

func TestNaming_Check_flags_forbidden_package(t *testing.T) {
	spec := namingSpec("utils", "helpers")

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		namingFile("utils", "internal/alpha/util/a.go"),
	}}

	result, err := NewNaming().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	require.Len(t, result.NamingWarnings, 1)

	w := result.NamingWarnings[0]
	assert.Equal(t, "utils", w.PackageName)
	assert.Contains(t, w.PackagePath, "internal/alpha/util")
	assert.Equal(t, 1, w.FilesCount)
	assert.Equal(t, "/internal/alpha/util/a.go", w.FileRelativePath)
}

func TestNaming_Check_aggregates_files_per_package(t *testing.T) {
	spec := namingSpec("common")

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		namingFile("common", "internal/alpha/common/a.go"),
		namingFile("common", "internal/alpha/common/b.go"),
		namingFile("common", "internal/alpha/common/c.go"),
	}}

	result, err := NewNaming().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	require.Len(t, result.NamingWarnings, 1)
	assert.Equal(t, 3, result.NamingWarnings[0].FilesCount)
}

func TestNaming_Check_clean_packages_pass(t *testing.T) {
	spec := namingSpec("utils", "helpers")

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		namingFile("user", "internal/alpha/user/a.go"),
		namingFile("orders", "internal/alpha/orders/b.go"),
	}}

	result, err := NewNaming().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	assert.Empty(t, result.NamingWarnings)
}

func TestNaming_Check_no_rules_noop(t *testing.T) {
	spec := namingSpec()

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		namingFile("utils", "internal/alpha/util/a.go"),
	}}

	result, err := NewNaming().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	assert.Empty(t, result.NamingWarnings)
}

func TestNaming_Check_multiple_packages_deterministic_order(t *testing.T) {
	spec := namingSpec("utils", "helpers")

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		namingFile("helpers", "internal/alpha/z/helper.go"),
		namingFile("utils", "internal/alpha/a/util.go"),
	}}

	result, err := NewNaming().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	require.Len(t, result.NamingWarnings, 2)

	// sorted by package path
	assert.LessOrEqual(t, result.NamingWarnings[0].PackagePath, result.NamingWarnings[1].PackagePath)
	assert.Equal(t, "utils", result.NamingWarnings[0].PackageName)
	assert.Equal(t, "helpers", result.NamingWarnings[1].PackageName)
}

func TestNaming_Check_files_without_package_name_ignored(t *testing.T) {
	spec := namingSpec("utils")

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		namingFile("", "internal/alpha/odd.go"),
	}}

	result, err := NewNaming().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	assert.Empty(t, result.NamingWarnings)
}
