package archlint_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	archlint "github.com/vsfedorenko/go-arch-lint/v2"
	"github.com/vsfedorenko/go-arch-lint/v2/dsl"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
)

/**
 * Black-box tests for the delegated CLI surface (archlint.RunCLI):
 * the scaffolded .go-arch-lint/main.go entry point that must keep EVERY
 * delegated command's own behavior (check, mapping, graph, self-inspect)
 * instead of silently degrading them to a check run.
 */

func cliSpec() dsl.SpecDef {
	return dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("alpha", "alpha/**")
		dsl.Component("beta", "beta/**")
		dsl.Deps("alpha", func() { dsl.AnyProjectDeps(true) })
		dsl.Deps("beta", func() { dsl.AnyProjectDeps(true) })
	})
}

func TestRunCLI_check_passes_on_clean_project(t *testing.T) {
	root := writeProject(t, false)

	err := archlint.RunCLI(cliSpec(), []string{tcCmdCheck, flProjectPath, root, flNoColors})
	assert.NoError(t, err, "clean project check must pass")
}

func TestRunCLI_no_command_defaults_to_check(t *testing.T) {
	root := writeProject(t, false)

	// A bare `go run .go-arch-lint/` (no command token) used to lint via
	// MustRun; it must keep doing so, not print help and exit 0.
	err := archlint.RunCLI(cliSpec(), []string{flProjectPath, root, flNoColors})
	assert.NoError(t, err, "bare invocation must still run a check")
}

func TestRunCLI_violations_exit_contract_preserved(t *testing.T) {
	root := writeProject(t, false)

	spec := dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("alpha", "alpha/**")
		dsl.Component("beta", "beta/**")
		// alpha imports beta, but only alpha itself is allowed.
		dsl.Deps("alpha", func() { dsl.MayDependOn("alpha") })
		dsl.Deps("beta", func() { dsl.AnyProjectDeps(true) })
	})

	err := archlint.RunCLI(spec, []string{tcCmdCheck, flProjectPath, root, flNoColors})
	require.Error(t, err)
	assert.True(t, models.IsUserSpaceError(err), "violations must stay UserSpaceError, got %T: %v", err, err)
	assert.Equal(t, archlint.ExitCodeViolations, archlint.ExitCode(err))
}

func TestRunCLI_mapping_runs_mapping_not_check(t *testing.T) {
	root := writeProject(t, false)

	// The regression this command guards: `mapping` used to print the
	// CHECK report ("linters:" block, "OK - No warnings found") because
	// the scaffold unconditionally ran a check. Capture stdout and assert
	// the mapping-specific markers appear and the check ones do not.
	stdout := captureStdout(t, func() {
		_ = archlint.RunCLI(cliSpec(), []string{"mapping", flProjectPath, root, flNoColors})
	})

	assert.Contains(t, stdout, "Project Packages:", "mapping output expected")
	assert.NotContains(t, stdout, "No warnings found", "mapping must not run a check")
	assert.NotContains(t, stdout, "linters:", "mapping must not run a check")
}

func TestRunCLI_self_inspect_reports_module(t *testing.T) {
	root := writeProject(t, false)

	stdout := captureStdout(t, func() {
		err := archlint.RunCLI(cliSpec(), []string{tcCmdSelfInspect, flProjectPath, root, flJSONFlag})
		assert.NoError(t, err)
	})

	assert.Contains(t, stdout, "models.SelfInspect", "self-inspect JSON wrapper expected")
	assert.Contains(t, stdout, "example.com/e2e", "module name expected in self-inspect output")
}

func TestRunCLI_unknown_command_is_config_error(t *testing.T) {
	root := writeProject(t, false)

	err := archlint.RunCLI(cliSpec(), []string{"frobnicate", flProjectPath, root})
	require.Error(t, err)
	assert.Equal(t, archlint.ExitCodeConfigError, archlint.ExitCode(err))
}

func TestRunCLI_empty_spec_rejected(t *testing.T) {
	err := archlint.RunCLI(dsl.SpecDef{}, []string{tcCmdCheck})
	require.Error(t, err)
	assert.Equal(t, archlint.ExitCodeConfigError, archlint.ExitCode(err))
}
