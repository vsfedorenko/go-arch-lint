package archlint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	archlint "github.com/vsfedorenko/go-arch-lint/v3"
	"github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

// Synthetic baseline-mode contracts (found by probing the freshly merged
// --baseline feature):
//
//  1. --baseline-update without --baseline silently no-op'ed (nothing
//     recorded, exit 1 from ordinary violations) — now a ConfigError.
//  2. A broken/missing baseline file rendered the empty check model:
//     "OK - No warnings found" on stdout with exit 2 — actively
//     misleading in CI.
func TestRun_BaselineUpdateWithoutBaseline_IsConfigError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "app"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module baseline.probe\n\ngo 1.25\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "app", "a.go"), []byte("package app\n"), 0o600))

	spec := dsl.Spec(func(s *dsl.SpecBuilder) {
		s.Path("internal/app")
	})

	err := archlint.Run(spec,
		archlint.WithProjectPath(root),
		archlint.WithBaselineUpdate(),
	)
	require.Error(t, err)
	require.Equal(t, archlint.ExitCodeConfigError, archlint.ExitCode(err), "must map to config error (2), got %d: %v", archlint.ExitCode(err), err)
	assert.Contains(t, err.Error(), "requires --baseline", "error must name the missing flag: ")
}
