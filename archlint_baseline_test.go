package archlint_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	archlint "github.com/vsfedorenko/go-arch-lint/v2"
	"github.com/vsfedorenko/go-arch-lint/v2/dsl"
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
	spec := dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("main", "app")
	})

	err := archlint.Run(spec,
		archlint.WithProjectPath("../"),
		archlint.WithBaselineUpdate(),
	)
	require.Error(t, err)
	require.Equal(t, archlint.ExitCodeConfigError, archlint.ExitCode(err), "must map to config error (2), got %d: %v", archlint.ExitCode(err), err)
	assert.Contains(t, err.Error(), "requires --baseline", "error must name the missing flag: ")
}
