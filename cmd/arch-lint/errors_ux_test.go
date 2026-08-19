package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/**
 * Error UX tests for the delegation launcher: the two system failure
 * modes must produce actionable messages instead of silence (go missing)
 * or context-free compiler noise (spec does not compile).
 *
 * These run the real launcher binary against fixture projects.
 */

// buildLauncher compiles the launcher to a temp binary.
func buildLauncher(t *testing.T) string {
	t.Helper()

	bin := t.TempDir() + "/archlint-launcher"
	cmd := exec.Command("go", "build", "-o", bin, ".")
	_, err := cmd.CombinedOutput()
	require.NoError(t, err)

	return bin
}

// writeFixture creates a minimal project with the given spec main.go body.
func writeFixture(t *testing.T, specMain string) string {
	t.Helper()

	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(dir+"/.go-arch-lint", 0o755))
	require.NoError(t, os.WriteFile(dir+"/main.go", []byte("package main\n\nfunc main() {}\n"), 0o600))
	require.NoError(t, os.WriteFile(dir+"/go.mod", []byte("module example.com/fixture\n\ngo 1.25\n"), 0o600))
	require.NoError(t, os.WriteFile(dir+"/.go-arch-lint/go.mod", []byte("module example.com/fixture/.go-arch-lint\n\ngo 1.25\n"), 0o600))
	require.NoError(t, os.WriteFile(dir+"/.go-arch-lint/arch.go", []byte(specMain), 0o600))

	return dir
}

func TestLauncher_GoNotOnPath_PrintsHint(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}

	bin := buildLauncher(t)
	// The go-missing path fails at exec.Command spawn — before any spec
	// compilation — so a minimal spec body suffices (no module deps).
	dir := writeFixture(t, "package main\n\nfunc main() {}\n")

	cmd := exec.Command(bin, "check", "--project-path", dir)

	// Point PATH at a guaranteed-empty directory: CI images often ship a
	// system go in /usr/bin, so stripping to /usr/bin is not enough.
	emptyBin := t.TempDir()
	cmd.Env = []string{"PATH=" + emptyBin, "HOME=" + t.TempDir()}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run()

	out := stderr.String()
	assert.Contains(t, out, "'go' is not on PATH", "expected PATH hint on stderr")
}

func TestLauncher_BrokenSpec_PrintsContext(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}

	bin := buildLauncher(t)
	// A syntax error fails at parse time — before module resolution, so
	// the fixture needs no network access.
	dir := writeFixture(t, "package main\n\nthis does not compile\n")

	cmd := exec.Command(bin, "check", "--project-path", dir)
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	require.Error(t, err)
	var exitErr *exec.ExitError
	require.True(t, errors.As(err, &exitErr), "expected ExitError, got ")

	out := stderr.String()
	assert.Contains(t, out, "did not build", "expected 'did not build' context on stderr")
	assert.Contains(t, out, "go-arch-lint init", "expected init hint on stderr")
}
