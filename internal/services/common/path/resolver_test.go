package path

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The glob resolver underlies component path resolution ("internal/**",
// "services/*"). These tests pin its doublestar passthrough, directory
// filtering, and the dot-suffix normalization quirk.

func TestResolve_PlainGlobMatchesDirectories(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "alpha", "beta")

	r := NewResolver()
	got, err := r.Resolve(filepath.Join(root, "*"))
	require.NoError(t, err, "Resolve")
	assertDirs(t, got, []string{filepath.Join(root, "alpha"), filepath.Join(root, "beta")})
}

func TestResolve_FilesAreFilteredOut(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "alpha")
	writeFile(t, filepath.Join(root, "readme.md"), "x")

	r := NewResolver()
	got, err := r.Resolve(filepath.Join(root, "*"))
	require.NoError(t, err, "Resolve")
	// Only the directory matches; the file is dropped.
	assertDirs(t, got, []string{filepath.Join(root, "alpha")})
}

func TestResolve_DoubleStarRecurses(t *testing.T) {
	// Contract: "a/**" includes the anchor directory itself plus every
	// descendant directory (the expand() walk starts at the anchor).
	root := t.TempDir()
	mkdirs(t, root, "a/b/c", "a/d")

	r := NewResolver()
	got, err := r.Resolve(filepath.Join(root, "a/**")) //nolint:gocritic // glob patterns, not literal paths
	require.NoError(t, err, "Resolve")
	assertDirs(t, got, []string{
		filepath.Join(root, "a"),
		filepath.Join(root, "a", "b"),
		filepath.Join(root, "a", "b", "c"),
		filepath.Join(root, "a", "d"),
	})
}

func TestResolve_NoMatchIsEmpty(t *testing.T) {
	root := t.TempDir()
	r := NewResolver()
	got, err := r.Resolve(filepath.Join(root, "nothing-*"))
	require.NoError(t, err, "Resolve")
	assert.Empty(t, got, "expected no matches")
}

func TestResolve_DotSuffixTrimmed(t *testing.T) {
	// Contract: the "/." suffix is trimmed BEFORE globbing, so
	// "<root>/alpha/." resolves to alpha itself. Note the result keeps a
	// trailing slash — downstream consumers (spec assembler resolver)
	// TrimRight("/") and filepath.Clean it; pinned here so a refactor of
	// either side cannot silently change ownership of the cleanup.
	root := t.TempDir()
	mkdirs(t, root, "alpha")

	r := NewResolver()
	got, err := r.Resolve(filepath.Join(root, "alpha") + "/.")
	require.NoError(t, err, "Resolve")
	assertDirs(t, got, []string{filepath.Join(root, "alpha") + "/"})
}

func TestResolve_NestedDoubleStarSegments(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "x/p1/inner", "x/p2/inner")

	r := NewResolver()
	got, err := r.Resolve(filepath.Join(root, "x/**/inner")) //nolint:gocritic // glob patterns, not literal paths
	require.NoError(t, err, "Resolve")
	assertDirs(t, got, []string{
		filepath.Join(root, "x", "p1", "inner"),
		filepath.Join(root, "x", "p2", "inner"),
	})
}

// --- helpers ---

func mkdirs(t *testing.T, root string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755))
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

func assertDirs(t *testing.T, got []string, want []string) {
	t.Helper()
	require.Len(t, got, len(want), "resolve got %v, want ")
	set := map[string]bool{}
	for _, w := range want {
		set[w] = true
	}
	for _, g := range got {
		assert.Contains(t, set, g, "resolve got unexpected %v, want ")
	}
}
