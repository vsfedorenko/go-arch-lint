package info

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

// workspace_test.go pins the go.work awareness of ProjectInfo: `use`
// entries are collected with their declared module paths, the root module
// is excluded, and broken members degrade to being skipped — the lint must
// keep working with root-module-only knowledge.

const testSiblingModulePath = "example.com/y"

func writeWorkspace(t *testing.T, useDirectives string, memberModules map[string]string) string {
	t.Helper()
	root := writeProject(t, "module example.com/root\n\ngo 1.25\n")

	body := "go 1.25\n\n" + useDirectives
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.work"), []byte(body), 0o600)) //nolint:gosec // test fixture in t.TempDir()

	for rel, modulePath := range memberModules {
		dir := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(dir, 0o755), "mkdir %s", rel) //nolint:gosec // test fixture dirs
		require.NoError(t, os.WriteFile(                             //nolint:gosec // test fixture in t.TempDir()
			filepath.Join(dir, "go.mod"),
			[]byte("module "+modulePath+"\n\ngo 1.25\n"),
			0o600,
		), "write %s/go.mod", rel)
	}

	return root
}

func TestProjectInfo_WorkspaceModulesCollected(t *testing.T) {
	root := writeWorkspace(t, "use (\n\t.\n\t./two/y\n)\n", map[string]string{
		"two/y": "example.com/y",
	})

	info, err := NewAssembler().ProjectInfo(root, ".go-arch-lint/arch.go")
	require.NoError(t, err, "ProjectInfo")

	want := []domain.WorkspaceModule{{Dir: filepath.Join(root, "two", "y"), Path: testSiblingModulePath}}
	assert.Equal(t, want, info.WorkspaceModules, "sibling module collected, root excluded")
}

func TestProjectInfo_NoGoWork_NoWorkspaceModules(t *testing.T) {
	root := writeProject(t, "module example.com/root\n\ngo 1.25\n")

	info, err := NewAssembler().ProjectInfo(root, ".go-arch-lint/arch.go")
	require.NoError(t, err, "ProjectInfo")
	assert.Empty(t, info.WorkspaceModules, "no go.work → no workspace modules")
}

func TestProjectInfo_WorkspaceMemberWithoutGoModSkipped(t *testing.T) {
	// A use entry pointing at a directory without go.mod (module outside
	// the linted tree) is skipped, not an error.
	root := writeWorkspace(t, "use (\n\t.\n\t./missing\n)\n", nil)

	info, err := NewAssembler().ProjectInfo(root, ".go-arch-lint/arch.go")
	require.NoError(t, err, "ProjectInfo")
	assert.Empty(t, info.WorkspaceModules, "broken member skipped")
}

func TestProjectInfo_BrokenGoWorkIgnored(t *testing.T) {
	root := writeProject(t, "module example.com/root\n\ngo 1.25\n")
	require.NoError(t, os.WriteFile( //nolint:gosec // test fixture in t.TempDir()
		filepath.Join(root, "go.work"),
		[]byte("this is not a go.work file !!!\n"),
		0o600,
	), "write go.work")

	info, err := NewAssembler().ProjectInfo(root, ".go-arch-lint/arch.go")
	require.NoError(t, err, "malformed go.work must not fail the lint")
	assert.Empty(t, info.WorkspaceModules, "no workspace knowledge from a broken go.work")
}
