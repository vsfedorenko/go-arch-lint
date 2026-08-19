package info

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ProjectInfo assembles the project descriptor: absolute root, arch-file
// path, go.mod presence, and module name extraction. These tests drive it
// against a real temp-dir project layout.

func writeProject(t *testing.T, goModBody string) string {
	t.Helper()
	root := t.TempDir()
	if goModBody != "" {
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte(goModBody), 0o600))
	}
	return root
}

func TestProjectInfo_HappyPath(t *testing.T) {
	root := writeProject(t, "module example.com/myapp\n\ngo 1.25\n")

	info, err := NewAssembler().ProjectInfo(root, ".go-arch-lint/arch.go")
	require.NoError(t, err, "ProjectInfo")

	abs, _ := filepath.Abs(root)
	assert.Equal(t, abs, info.Directory, "directory")
	assert.Equal(t, "example.com/myapp", info.ModuleName, "module")
	assert.Equal(t, filepath.Join(abs, "go.mod"), info.GoModFilePath, "go.mod path")
	assert.True(t, strings.HasSuffix(info.GoArchFilePath, ".go-arch-lint/arch.go"),
		"arch path = %q", info.GoArchFilePath)
}

func TestProjectInfo_RelativeRoot(t *testing.T) {
	// Relative roots (the CLI default "./") resolve against CWD.
	root := writeProject(t, "module m\n")
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	info, err := NewAssembler().ProjectInfo(".", ".go-arch-lint/arch.go")
	require.NoError(t, err, "ProjectInfo")

	assert.True(t, filepath.IsAbs(info.Directory), "directory must be absolute, got %q", info.Directory)
}

func TestProjectInfo_MissingGoMod(t *testing.T) {
	root := writeProject(t, "")

	_, err := NewAssembler().ProjectInfo(root, ".go-arch-lint/arch.go")
	require.Error(t, err, "missing go.mod must fail")
	require.ErrorContains(t, err, "not found project 'go.mod'")
}

func TestProjectInfo_BrokenGoMod(t *testing.T) {
	root := writeProject(t, "this is not a go.mod file !!!\n")

	// modfile.ParseLax is tolerant of unknown lines, but broken syntax
	// must surface as a module-name error, not a crash.
	_, err := NewAssembler().ProjectInfo(root, ".go-arch-lint/arch.go")
	require.False(t, err == nil && !goModIsParseable(root),
		"expected an error or a parseable go.mod, got err=%v", err)
}

func goModIsParseable(root string) bool {
	body, err := os.ReadFile(filepath.Join(root, "go.mod"))
	return err == nil && strings.Contains(string(body), "module")
}

func TestProjectInfo_GoModWithoutModule(t *testing.T) {
	root := writeProject(t, "go 1.25\n")

	_, err := NewAssembler().ProjectInfo(root, ".go-arch-lint/arch.go")
	require.Error(t, err, "go.mod without a module directive must fail module-name extraction")
	require.ErrorContains(t, err, "failed get module name")
}

func TestProjectInfo_ArchFileURLRejected(t *testing.T) {
	root := writeProject(t, "module m\n")

	_, err := NewAssembler().ProjectInfo(root, "https://example.com/arch.go")
	require.Error(t, err, "URL arch files must be rejected in v2 Go DSL mode")
	require.ErrorContains(t, err, "not supported")
}

func TestProjectInfo_AbsoluteArchFilePath(t *testing.T) {
	root := writeProject(t, "module m\n")
	absArch := filepath.Join(root, "elsewhere.go")

	info, err := NewAssembler().ProjectInfo(root, absArch)
	require.NoError(t, err, "ProjectInfo")

	assert.Equal(t, absArch, info.GoArchFilePath, "absolute arch path must pass through")
}
