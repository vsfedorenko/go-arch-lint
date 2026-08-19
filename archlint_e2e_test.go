package archlint_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	archlint "github.com/vsfedorenko/go-arch-lint"
	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

/**
 * Black-box tests for the public library API (archlint.Run / ExitCode):
 * a consumer writes a spec in code and lints a real project on disk.
 * No internal helpers — only the documented entry points.
 */

// writeProject creates a two-component project on disk:
// internal/alpha imports internal/beta (allowed), and optionally
// internal/beta imports internal/alpha (a cycle when enabled).
func writeProject(t *testing.T, withCycle bool) string {
	t.Helper()

	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}

	write("go.mod", "module example.com/e2e\n\ngo 1.25\n")
	write("internal/alpha/a.go", "package alpha\n\nimport _ \"example.com/e2e/internal/beta\"\n")
	write("internal/beta/b.go", "package beta\n")

	if withCycle {
		write("internal/beta/back.go", "package beta\n\nimport _ \"example.com/e2e/internal/alpha\"\n")
	}

	return root
}

func twoComponentSpec() dsl.SpecDef {
	return dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("alpha", "alpha/**")
		dsl.Component("beta", "beta/**")
		dsl.Deps("alpha", func() { dsl.MayDependOn("beta") })
		dsl.Deps("beta", func() { dsl.MayDependOn("beta") })
	})
}

func TestRun_clean_project_passes(t *testing.T) {
	root := writeProject(t, false)

	err := archlint.Run(twoComponentSpec(), archlint.WithProjectPath(root))
	require.NoError(t, err, "clean project must pass, got")
	assert.Equal(t, archlint.ExitCodeOK, archlint.ExitCode(err))
}

func TestRun_violations_fail_with_userspace_error(t *testing.T) {
	root := writeProject(t, false)

	// beta imports alpha — not allowed by the spec.
	spec := dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("alpha", "alpha/**")
		dsl.Component("beta", "beta/**")
		// alpha may depend on beta, but beta has NO allowance -> the
		// actual alpha->beta import violates.
		dsl.Deps("alpha", func() { dsl.MayDependOn("alpha") })
		dsl.Deps("beta", func() { dsl.MayDependOn("beta") })
	})

	err := archlint.Run(spec, archlint.WithProjectPath(root))
	require.Error(t, err)
	require.True(t, models.IsUserSpaceError(err), "violations must be a UserSpaceError, got %T: ")
	assert.Equal(t, archlint.ExitCodeViolations, archlint.ExitCode(err))
}

func TestRun_cycle_detected(t *testing.T) {
	root := writeProject(t, true)

	err := archlint.Run(twoComponentSpec(), archlint.WithProjectPath(root))
	require.Error(t, err)
	require.True(t, models.IsUserSpaceError(err), "cycle must surface as UserSpaceError, got ")
}

func TestRun_missing_project_is_config_error(t *testing.T) {
	err := archlint.Run(twoComponentSpec(), archlint.WithProjectPath(t.TempDir()+"/nonexistent"))
	require.Error(t, err)
	assert.Equal(t, archlint.ExitCodeConfigError, archlint.ExitCode(err))
	require.True(t, models.IsConfigError(err), "missing project must be a config error, got %T: %v")
}

func TestRun_empty_spec_rejected(t *testing.T) {
	err := archlint.Run(dsl.SpecDef{})
	require.Error(t, err)
	assert.Equal(t, archlint.ExitCodeConfigError, archlint.ExitCode(err))
}

func TestRun_naming_rule_flows_through_public_api(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}

	write("go.mod", "module example.com/naming\n\ngo 1.25\n")
	write("internal/core/a.go", "package core\n")
	write("internal/util/u.go", "package util\n")

	spec := dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("core", "core/**")
		dsl.Component("util", "util/**")
		dsl.Naming(func() { dsl.ForbiddenPackages("util") })
		dsl.CommonComponents("core", "util")
	})

	err := archlint.Run(spec, archlint.WithProjectPath(root))
	require.Error(t, err)
	require.True(t, models.IsUserSpaceError(err), "naming violation must be UserSpaceError, got ")
}

func TestRun_tier_rule_flows_through_public_api(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}

	// domain -> infra (downward, allowed); infra -> domain (upward, violation).
	write("go.mod", "module example.com/tiers\n\ngo 1.25\n")
	write("internal/domain/d.go", "package domain\n\nimport _ \"example.com/tiers/internal/infra\"\n")
	write("internal/infra/i.go", "package infra\n\nimport _ \"example.com/tiers/internal/domain\"\n")

	spec := dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("domain", "domain/**")
		dsl.Component("infra", "infra/**")
		dsl.Tiers("domain", "infra")
		dsl.Tier("domain", "domain")
		dsl.Tier("infra", "infra")
		// allow everything at the dependency level: tier rules are
		// independent of mayDependOn permissions.
		dsl.Deps("domain", func() { dsl.AnyProjectDeps(true) })
		dsl.Deps("infra", func() { dsl.AnyProjectDeps(true) })
	})

	err := archlint.Run(spec, archlint.WithProjectPath(root))
	require.Error(t, err)
	require.True(t, models.IsUserSpaceError(err), "tier violation must be UserSpaceError, got ")
}

func TestRun_json_format_emits_violations(t *testing.T) {
	root := writeProject(t, false)

	spec := dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("alpha", "alpha/**")
		dsl.Component("beta", "beta/**")
		dsl.Deps("alpha", func() { dsl.MayDependOn("alpha") }) // alpha->beta violates
		dsl.Deps("beta", func() { dsl.MayDependOn("beta") })
	})

	// JSON format must not change the error classification.
	err := archlint.Run(spec, archlint.WithProjectPath(root), archlint.WithFormat(models.FormatJSON))
	require.Error(t, err, "json run: expected UserSpaceError")
	require.True(t, models.IsUserSpaceError(err), "json run: expected UserSpaceError, got %v", err)
}

// errors classification through the public surface.
func TestExitCode_plain_errors_are_config(t *testing.T) {
	assert.Equal(t, archlint.ExitCodeConfigError, archlint.ExitCode(errors.New("boom")))
	assert.Equal(t, archlint.ExitCodeOK, archlint.ExitCode(nil))
	assert.Equal(t, archlint.ExitCodeViolations, archlint.ExitCode(models.NewUserSpaceError("v")))
}
