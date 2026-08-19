package check_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	archlint "github.com/vsfedorenko/go-arch-lint/v2"
)

// End-to-end contracts for the output flags on the scaffold path — the
// exact surface a user drives through `go-arch-lint check --output-type=…
// --json --output-json-one-line`. The scaffolded main() forwards
// os.Args[1:] through OptionsFromFlags; before the fix these flags were
// silently dropped (check --output-type=json still printed ASCII) — the
// flag-drop class reported against the CLI upstream (issue #62).

const archOutputFlagsTpl = `package main

import (
	"os"

	"github.com/vsfedorenko/go-arch-lint/v2"
	. "github.com/vsfedorenko/go-arch-lint/v2/dsl"
)

func main() {
	spec := Spec(func() {
		Version(1)
		Workdir("internal")
		Component("x", "wei rd::name/x")
		Component("y", "wei rd::name/y")
		Deps("x", func() {
			MayDependOn("y")
			AnyVendorDeps(true)
		})
		Deps("y", func() { AnyVendorDeps(true) })
	})
	archlint.MustRun(spec, archlint.OptionsFromFlags(os.Args[1:])...)
}
`

// runArchLintArgs scaffolds-then-runs variant of runArchLint with explicit
// CLI args (the shared helper always runs bare `go run .`).
func runArchLintArgs(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	// `go run . --flag value` — flags after the package path.
	cmdArgs := append([]string{"run", ".", "--"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GOFLAGS=-mod=mod",
		"GOSUMDB=off",
		"GOPROXY=off",
	)

	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb

	err := cmd.Run()
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			exitCode = parseChildExitCode(errb.String(), exitErr.ExitCode())
		} else {
			require.NoError(t, err, "go run:\nstderr:\n%s", errb.String())
		}
	}
	return out.String(), errb.String(), exitCode
}

// runOutputFlagsFixture scaffolds the weird project's arch module with the
// OptionsFromFlags-forwarding main and runs it with the given CLI flags.
func runOutputFlagsFixture(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	root := repoRoot(t)
	dir := scaffoldArch(t, root, archOutputFlagsTpl)

	// The scaffolded main defaults to projectPath "../" (its own module
	// dir); the weird fixture lives elsewhere — point the run at it.
	args = append([]string{"--project-path", writeWeirdProject(t)}, args...)

	return runArchLintArgs(t, dir, args...)
}

func TestOutputFlags_output_type_json_renders_wrapper(t *testing.T) {
	stdout, _, code := runOutputFlagsFixture(t, "--output-type=json", "--no-colors")

	var wrapper struct {
		Type string `json:"Type"`
	}
	assert.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stdout)), &wrapper))
	assert.Contains(t, wrapper.Type, "Check", "wrapper Type must name the check model, got ")
	require.Equal(t, 0, code, "clean fixture must exit 0, got ")
}

func TestOutputFlags_json_alias_matches_output_type(t *testing.T) {
	stdoutAlias, _, _ := runOutputFlagsFixture(t, "--json", "--no-colors")
	stdoutExplicit, _, _ := runOutputFlagsFixture(t, "--output-type=json", "--no-colors")

	require.Equal(t, strings.TrimSpace(stdoutExplicit), strings.TrimSpace(stdoutAlias),
		"--json must alias --output-type=json:\nalias:    %q\nexplicit: %q", stdoutAlias, stdoutExplicit)
}

func TestOutputFlags_one_line_without_json_is_config_error(t *testing.T) {
	_, stderr, code := runOutputFlagsFixture(t, "--output-json-one-line")

	require.Equal(t, archlint.ExitCodeConfigError, code, "one-line without json output must exit 2 (config error)")
	assert.Contains(t, stderr, "--output-json-one-line only affects json output", "stderr must carry the actionable error, got: %s")
}

func TestOutputFlags_one_line_compacts_wrapper(t *testing.T) {
	stdout, _, _ := runOutputFlagsFixture(t, "--json", "--output-json-one-line", "--no-colors")

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	require.Len(t, lines, 1, "one-line JSON must be a single line, got %d: %q", len(lines), stdout)
	assert.NotContains(t, lines[0], "  ", "one-line JSON must not be indented: ")
}

func TestOutputFlags_unknown_output_type_is_config_error(t *testing.T) {
	_, stderr, code := runOutputFlagsFixture(t, "--output-type=yaml")

	require.Equal(t, archlint.ExitCodeConfigError, code, "unknown output-type must exit 2 (config error)")
	assert.Contains(t, stderr, "unknown output-type", "stderr must name the problem, got: %s")
}
