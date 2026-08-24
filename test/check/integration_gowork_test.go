package check_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// integration_gowork_test.go pins the go.work delegation contract: the
// launcher's `go run .go-arch-lint/` must succeed in a project whose root
// carries a go.work. The scaffold module is not a workspace member, so
// workspace mode rejects it ("current directory is contained in a module
// that is not one of the workspace modules listed in go.work", or — with
// `use .` covering the root module — "main module does not contain package
// <dir>/.go-arch-lint"). The launcher disables workspace mode for the
// delegated build only; the user's go.work stays untouched.
//
// Unlike runArchCheck (which invokes `go run` directly from the arch
// module), this test drives the REAL launcher binary so the delegation
// environment is the one users get.

// buildLauncherFor compiles the launcher binary from the repo root.
func buildLauncherFor(t *testing.T, repoRoot string) string {
	t.Helper()

	launcher := filepath.Join(t.TempDir(), "arch-lint")
	build := exec.Command("go", "build", "-o", launcher, "./cmd/arch-lint") //nolint:gosec // intentional: builds the project's own launcher into a test temp dir
	build.Dir = repoRoot
	out, err := build.CombinedOutput()
	require.NoError(t, err, "build launcher: %s", out)

	return launcher
}

// runLauncherCheck runs the launcher against projectDir and returns the
// combined output plus the exit code.
func runLauncherCheck(t *testing.T, launcher, projectDir string) (string, int) {
	t.Helper()

	cmd := exec.Command(launcher, "check", "--project-path", projectDir) //nolint:gosec // intentional: runs the launcher built by this test against a temp fixture
	cmd.Env = append(os.Environ(),
		"GOFLAGS=-mod=mod",
		"GOSUMDB=off",
		"GOPROXY=off",
	)

	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr, "launcher must exit with an ExitError, got %v", err)
		code = exitErr.ExitCode()
	}
	return out.String(), code
}

// TestGoWorkProjectChecksGreen scaffolds a single-module project whose
// root carries a go.work with `use .` and asserts the delegated check is
// green through the real launcher.
func TestGoWorkProjectChecksGreen(t *testing.T) {
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
	// `use .` makes the ROOT module a workspace member — the exact shape
	// that produced "main module does not contain package fixt/.go-arch-lint".
	write("go.work", "go 1.25\n\nuse .\n")

	launcher := buildLauncherFor(t, root)
	scaffoldDefaultArchDir(t, proj, root)

	out, code := runLauncherCheck(t, launcher, proj)
	assert.Equal(t, 0, code, "check under go.work must be green; exit %d.\noutput:\n%s", code, out)
	assert.Contains(t, out, "No warnings found", "expected OK banner under go.work")
}
