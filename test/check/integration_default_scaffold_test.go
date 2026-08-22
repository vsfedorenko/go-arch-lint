package check_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// integration_default_scaffold_test.go pins the DEFAULT `init` scaffold
// contract: a freshly scaffolded project must run `check` green WITHOUT the
// user editing arch.go first. The scaffold used to ship with every component
// commented out (spec invalid: "at least one component must be defined") and
// Workdir("internal") (invalid on projects without internal/) — the "3. Run
// 'go-arch-lint check'" next-step printed by init was guaranteed to fail.

// scaffoldDefaultArchDir builds the arch module exactly the way `init` +
// `go mod tidy` would, but offline: the default spec written by the launcher
// (read from a real init run, so template drift breaks this test), local
// replace, repo go.sum.
func scaffoldDefaultArchDir(t *testing.T, projectDir, repoRoot string) {
	t.Helper()

	// Run the real launcher so the test pins the SHIPPED template, not a
	// copy that could drift.
	launcher := filepath.Join(t.TempDir(), "arch-lint")
	build := exec.Command("go", "build", "-o", launcher, "./cmd/arch-lint")
	build.Dir = repoRoot
	out, err := build.CombinedOutput()
	require.NoError(t, err, "build launcher: %s", out)

	init := exec.Command(launcher, "init", "-p", projectDir)
	out, err = init.CombinedOutput()
	require.NoError(t, err, "launcher init: %s", out)

	// Offline wiring: replace + full require graph + repo go.sum.
	goMod := offlineGoMod(t, repoRoot)
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ".go-arch-lint", "go.mod"), []byte(goMod), 0o600), "rewrite go.mod for offline replace") //nolint:gosec // test fixture in t.TempDir()

	sum, err := os.ReadFile(filepath.Join(repoRoot, "go.sum"))
	require.NoError(t, err, "read repo go.sum")
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ".go-arch-lint", "go.sum"), sum, 0o600), "write go.sum") //nolint:gosec // test fixture in t.TempDir()
}

// TestDefaultScaffoldChecksGreen scaffolds a minimal-but-real module and
// asserts the fresh default spec checks green end-to-end.
func TestDefaultScaffoldChecksGreen(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the launcher binary")
	}
	root := repoRoot(t)

	proj := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(proj, "pkg"), 0o755), "mkdir pkg") //nolint:gosec // test fixture dirs
	write := func(rel, body string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(proj, rel), []byte(body), 0o600), "write %s", rel) //nolint:gosec // test fixture in t.TempDir()
	}
	write("go.mod", "module fixt\n\ngo 1.25\n")
	write("main.go", "package main\n\nfunc main() {}\n")
	write("pkg/a.go", "package pkg\n\nfunc A() {}\n")

	scaffoldDefaultArchDir(t, proj, root)
	out, code := runArchCheck(t, proj)
	assert.Equal(t, 0, code, "fresh default scaffold must check green; exit %d.\noutput:\n%s", code, out)
	assert.Contains(t, out, "No warnings found", "expected OK banner")
}

// TestDefaultScaffoldEmptyModuleChecksGreen covers the empty-project corner:
// a module with no packages at all must still check green (the scaffold used
// to fail on a missing internal/ workdir before any code existed).
func TestDefaultScaffoldEmptyModuleChecksGreen(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the launcher binary")
	}
	root := repoRoot(t)

	proj := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(proj, "go.mod"), []byte("module fixt\n\ngo 1.25\n"), 0o600), "write go.mod") //nolint:gosec // test fixture in t.TempDir()

	scaffoldDefaultArchDir(t, proj, root)
	out, code := runArchCheck(t, proj)
	assert.Equal(t, 0, code, "empty module must check green; exit %d.\noutput:\n%s", code, out)
}
