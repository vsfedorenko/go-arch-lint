package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/project/scanner"
)

// TestScan_SkipsUnreadableExcludedDir reproduces the case where an excluded
// directory inside the project tree is unreadable (e.g. a root-owned local
// docker volume). Before the fix the walk descended into it, hit a
// permission-denied readdir error and aborted the whole scan. Excluded dirs
// must now be skipped before descending, so the scan succeeds and still
// returns the in-scope source files.
func TestScan_SkipsUnreadableExcludedDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permissions; cannot simulate an unreadable dir")
	}

	projectDir := t.TempDir()

	goFile := filepath.Join(projectDir, "main.go")
	require.NoError(t, os.WriteFile(goFile, []byte("package main\n"), 0o644), "write source file") //nolint:gosec // test fixture: source file needs 0644 to be readable

	excludedDir := filepath.Join(projectDir, ".data")
	require.NoError(t, os.Mkdir(excludedDir, 0o755), "mkdir excluded dir")
	// A subdir the walker would try to readdir into; made unreadable so that
	// descending into it fails, mirroring the root-owned postgres data dir.
	unreadable := filepath.Join(excludedDir, "pgdata")
	require.NoError(t, os.Mkdir(unreadable, 0o000), "mkdir unreadable dir")
	// Restore perms so t.TempDir cleanup can remove it.
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o755) })

	excludePaths := []models.ResolvedPath{{
		LocalPath: ".data",
		AbsPath:   excludedDir,
	}}

	files, err := scanner.NewScanner().Scan(context.Background(), projectDir, "example.com/proj", excludePaths, nil)
	require.NoError(t, err, "scan should skip the excluded unreadable dir")

	require.Len(t, files, 1, "scanned files")
	assert.Equal(t, goFile, files[0].Path, "expected only the source file to be scanned")
}

// TestScan_DoesNotDescendIntoExcludedDir asserts that .go files living inside
// an excluded directory are never scanned.
func TestScan_DoesNotDescendIntoExcludedDir(t *testing.T) {
	projectDir := t.TempDir()

	kept := filepath.Join(projectDir, "keep.go")
	require.NoError(t, os.WriteFile(kept, []byte("package main\n"), 0o644), "write kept file") //nolint:gosec // test fixture: source file needs 0644 to be readable

	excludedDir := filepath.Join(projectDir, "vendor")
	require.NoError(t, os.Mkdir(excludedDir, 0o755), "mkdir excluded dir")
	require.NoError(t, os.WriteFile(filepath.Join(excludedDir, "dep.go"), []byte("package vendor\n"), 0o644), "write excluded file") //nolint:gosec // test fixture: source file needs 0644 to be readable

	excludePaths := []models.ResolvedPath{{
		LocalPath: "vendor",
		AbsPath:   excludedDir,
	}}

	files, err := scanner.NewScanner().Scan(context.Background(), projectDir, "example.com/proj", excludePaths, nil)
	require.NoError(t, err, "unexpected scan error")

	require.Len(t, files, 1, "scanned files")
	assert.Equal(t, kept, files[0].Path, "expected only the kept file to be scanned")
}
