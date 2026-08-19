package check_test

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	root := filepath.Join(filepath.Dir(file), "..", "..")
	abs, err := filepath.Abs(root)
	require.NoError(t, err, "abs")
	return abs
}

func testProjectDir(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	dir := filepath.Join(root, "test", "check", "project")
	abs, err := filepath.Abs(dir)
	require.NoError(t, err, "abs")
	return abs
}

func scaffoldArch(t *testing.T, repoRoot, mainGo string) string {
	t.Helper()
	dir := t.TempDir()

	// Build the scaffolded go.mod from the repo's go.mod. We keep the same
	// require block (direct + indirect) so Go has a complete module graph and
	// never needs to look up intermediate versions over the network — which
	// would fail under GOPROXY=off in CI.
	repoGoMod, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	require.NoError(t, err, "read repo go.mod")

	// Extract everything after the module/require directives we want to
	// override: take the `go` line and all require/replace/exclude blocks
	// from the repo, then prepend our own module + replace directive.
	lines := strings.Split(string(repoGoMod), "\n")
	var graphLines []string
	skipping := true
	for _, ln := range lines {
		if skipping {
			// Skip the repo's module line; keep everything from the `go` line on.
			if strings.HasPrefix(ln, "go ") {
				skipping = false
			}
		}
		if !skipping {
			graphLines = append(graphLines, ln)
		}
	}

	goMod := fmt.Sprintf("module arch-lint-local\n\n%s\n\nrequire github.com/vsfedorenko/go-arch-lint/v2 v2.0.0-dev\n\nreplace github.com/vsfedorenko/go-arch-lint/v2 => %s\n",
		strings.Join(graphLines, "\n"), repoRoot)

	files := map[string]string{
		"go.mod":  goMod,
		"main.go": mainGo,
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644), "write %s", path) //nolint:gosec // test fixture: generated source files use 0644
	}

	// Copy the repo's go.sum so checksum verification passes offline.
	srcSum := filepath.Join(repoRoot, "go.sum")
	data, err := os.ReadFile(srcSum)
	require.NoError(t, err, "read go.sum")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.sum"), data, 0o644), "write go.sum") //nolint:gosec // test fixture

	return dir
}

// offlineGoMod returns a go.mod body for a scaffolded .go-arch-lint/
// module that resolves WITHOUT network access: it keeps the repo's full
// require graph (direct + indirect deps) and points the module at the
// local checkout via a replace directive. A bare `require v0.0.0` forces
// Go to look up intermediate module versions, which fails under
// GOPROXY=off in CI (observed: cobra → go-md2man lookup).
func offlineGoMod(t *testing.T, repoRoot string) string {
	t.Helper()

	repoGoMod, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	require.NoError(t, err, "read repo go.mod")

	lines := strings.Split(string(repoGoMod), "\n")
	var graphLines []string
	skipping := true
	for _, ln := range lines {
		if skipping {
			if strings.HasPrefix(ln, "go ") {
				skipping = false
			}
		}
		if !skipping {
			graphLines = append(graphLines, ln)
		}
	}

	return fmt.Sprintf("module arch-lint-local\n\n%s\n\nrequire github.com/vsfedorenko/go-arch-lint/v2 v2.0.0-dev\n\nreplace github.com/vsfedorenko/go-arch-lint/v2 => %s\n",
		strings.Join(graphLines, "\n"), repoRoot)
}

func runArchLint(t *testing.T, dir string) (stdout, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command("go", "run", ".")
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
			// `go run` exits 1 for ANY child failure; the child's real exit
			// code only appears in stderr as "exit status N". Parse it back
			// so the linter-conventional codes (0/1/2) are observable here.
			exitCode = parseChildExitCode(errb.String(), exitErr.ExitCode())
		} else {
			require.NoError(t, err, "failed to run go run:\nstderr: %s", errb.String())
		}
	}
	return out.String(), errb.String(), exitCode
}

// parseChildExitCode extracts "exit status N" from go run's stderr. Falls
// back to the raw go-run exit code when the line is absent (build errors).
func parseChildExitCode(stderr string, fallback int) int {
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "exit status ") {
			if n, err := strconv.Atoi(strings.TrimPrefix(line, "exit status ")); err == nil {
				return n
			}
		}
	}
	return fallback
}

const archOKTpl = `package main

import (
	"github.com/vsfedorenko/go-arch-lint/v2"
	. "github.com/vsfedorenko/go-arch-lint/v2/dsl"
)

func main() {
	spec := Spec(func() {
		Version(1)
		Allow(func() { DepOnAnyVendor(false) })
		Exclude("internal/excluded", "vendor", "variadic")
		ExcludeFiles("^.*_test\\.go$")
		Component("main", "internal")
		Component("a", "internal/a")
		Component("allowb", "internal/a/allowb")
		Component("b", "internal/b")
		Component("c", "internal/c/**")
		Component("d", "internal/d/**")
		Component("e", "internal/e/**")
		Component("nc", "internal/not_covered")
		Component("common", "internal/common/**")
		CommonComponents("common", "a", "c", "d", "e")
		Deps("allowb", func() { MayDependOn("b") })
		Deps("e", func() { AnyVendorDeps(true) })
	})
	archlint.MustRun(spec,
		archlint.WithProjectPath("%s"),
		archlint.WithColors(false),
	)
}
`

const archWarningsTpl = `package main

import (
	"github.com/vsfedorenko/go-arch-lint/v2"
	. "github.com/vsfedorenko/go-arch-lint/v2/dsl"
)

func main() {
	spec := Spec(func() {
		Version(1)
		Allow(func() { DepOnAnyVendor(false) })
		Exclude("internal/excluded", "vendor", "variadic")
		ExcludeFiles("^.*_test\\.go$")
		Component("main", "internal/.")
		Component("a", "internal/a")
		Component("allowb", "internal/a/allowb")
		Component("b", "internal/b")
		Component("c", "internal/c")
		Component("e", "internal/e")
		Component("common", "internal/common/**")
		Component("models", "internal/d/models/*/model")
		CommonComponents("common")
		Deps("e", func() {
			MayDependOn("models")
			AnyVendorDeps(true)
		})
		Deps("allowb", func() { MayDependOn("b") })
	})
	archlint.MustRun(spec,
		archlint.WithProjectPath("%s"),
		archlint.WithColors(false),
	)
}
`

const archInvalidSpecTpl = `package main

import (
	"github.com/vsfedorenko/go-arch-lint/v2"
	. "github.com/vsfedorenko/go-arch-lint/v2/dsl"
)

func main() {
	spec := Spec(func() {
		Version(1)
		Allow(func() { DepOnAnyVendor(false) })
		Component("main", "internal")
		Component("a", "internal/a")
		Component("not_exist", "internal/not_exist")
		CommonComponents("models")
		Deps("main", func() {
			MayDependOn("not_exist_too_rnd_order")
		})
	})
	archlint.MustRun(spec,
		archlint.WithProjectPath("%s"),
		archlint.WithColors(false),
	)
}
`

const archSARIFTpl = `package main

import (
	"github.com/vsfedorenko/go-arch-lint/v2"
	. "github.com/vsfedorenko/go-arch-lint/v2/dsl"
)

func main() {
	spec := Spec(func() {
		Version(1)
		Allow(func() { DepOnAnyVendor(false) })
		Exclude("internal/excluded", "vendor", "variadic")
		ExcludeFiles("^.*_test\\.go$")
		Component("main", "internal/.")
		Component("a", "internal/a")
		Component("allowb", "internal/a/allowb")
		Component("b", "internal/b")
		Component("c", "internal/c")
		Component("e", "internal/e")
		Component("common", "internal/common/**")
		Component("models", "internal/d/models/*/model")
		CommonComponents("common")
		Deps("e", func() {
			MayDependOn("models")
			AnyVendorDeps(true)
		})
		Deps("allowb", func() { MayDependOn("b") })
	})
	archlint.MustRun(spec,
		archlint.WithProjectPath("%s"),
		archlint.WithColors(false),
		archlint.WithFormat("sarif"),
	)
}
`

func TestCheckCommands(t *testing.T) {
	project := testProjectDir(t)
	root := repoRoot(t)

	tests := []struct {
		name       string
		archGo     string
		wantExit   int
		wantOutput string
	}{
		{
			name:       "ok",
			archGo:     fmt.Sprintf(archOKTpl, project),
			wantExit:   0,
			wantOutput: "OK - No warnings found",
		},
		{
			name:       "warnings",
			archGo:     fmt.Sprintf(archWarningsTpl, project),
			wantExit:   1,
			wantOutput: "Component c shouldn't depend on",
		},
		{
			name:       "invalid_spec",
			archGo:     fmt.Sprintf(archInvalidSpecTpl, project),
			wantExit:   2, // config error, distinct from violations (linter convention)
			wantOutput: "not found directories",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := scaffoldArch(t, root, tt.archGo)

			stdout, stderr, exitCode := runArchLint(t, dir)

			combined := stdout + stderr
			require.Equal(t, tt.wantExit, exitCode, "exit code\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
			assert.Contains(t, combined, tt.wantOutput, "output does not contain\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
		})
	}
}

// TestCheckSARIFFormat exercises the --format sarif path end-to-end: the
// scaffolded main() runs with WithFormat("sarif") against the fixture
// project (which has real violations under this spec), and stdout must be
// a parseable SARIF 2.1.0 log whose results carry rule IDs, file URIs and
// positive line numbers. Exit code stays 1 (violations found).
func TestCheckSARIFFormat(t *testing.T) {
	project := testProjectDir(t)
	root := repoRoot(t)
	dir := scaffoldArch(t, root, fmt.Sprintf(archSARIFTpl, project))

	stdout, stderr, exitCode := runArchLint(t, dir)
	require.Equal(t, 1, exitCode, "exit code = %d, want 1 (violations)\nstdout:\n%s\nstderr:\n%s; stdout:\n%s\nstderr:\n%s", stdout, stderr)

	var log sarifLog
	assert.NoError(t, json.Unmarshal([]byte(stdout), &log))
	assert.Equal(t, "2.1.0", log.Version, "SARIF version")
	require.Len(t, log.Runs, 1, "expected 1 run\nstdout:\n%s", stdout)
	require.NotEmpty(t, log.Runs[0].Results, "expected results\nstdout:\n%s", stdout)

	for _, res := range log.Runs[0].Results {
		assert.NotEmpty(t, res.RuleID, "result without ruleId: %+v", res)
		for _, loc := range res.Locations {
			uri := loc.PhysicalLocation.ArtifactLocation.URI
			assert.NotEmpty(t, uri, "artifact URI must be relative")
			assert.False(t, strings.HasPrefix(uri, "/"), "artifact URI must be relative, got %q", uri)
			assert.GreaterOrEqual(t, loc.PhysicalLocation.Region.StartLine, 1, "startLine must be >= 1 (%s)", uri)
		}
	}
}

// sarifLog is the minimal decode target for the integration assertions —
// only the envelope fields the test reasons about.
type sarifLog struct {
	Version string `json:"version"`
	Runs    []struct {
		Results []struct {
			RuleID    string `json:"ruleId"`
			Locations []struct {
				PhysicalLocation struct {
					ArtifactLocation struct {
						URI string `json:"uri"`
					} `json:"artifactLocation"`
					Region struct {
						StartLine int `json:"startLine"`
					} `json:"region"`
				} `json:"physicalLocation"`
			} `json:"locations"`
		} `json:"results"`
	} `json:"runs"`
}

// archJUnitTpl mirrors archSARIFTpl but renders the JUnit XML format.
const archJUnitTpl = `package main

import (
	"github.com/vsfedorenko/go-arch-lint/v2"
	. "github.com/vsfedorenko/go-arch-lint/v2/dsl"
)

func main() {
	spec := Spec(func() {
		Version(1)
		Allow(func() { DepOnAnyVendor(false) })
		Exclude("internal/excluded", "vendor", "variadic")
		ExcludeFiles("^.*_test\\.go$")
		Component("main", "internal/.")
		Component("a", "internal/a")
		Component("allowb", "internal/a/allowb")
		Component("b", "internal/b")
		Component("c", "internal/c")
		Component("e", "internal/e")
		Component("common", "internal/common/**")
		Component("models", "internal/d/models/*/model")
		CommonComponents("common")
		Deps("e", func() {
			MayDependOn("models")
			AnyVendorDeps(true)
		})
		Deps("allowb", func() { MayDependOn("b") })
	})
	archlint.MustRun(spec,
		archlint.WithProjectPath("%s"),
		archlint.WithColors(false),
		archlint.WithFormat("junit"),
	)
}
`

// junitReport is the minimal decode target for the JUnit assertions —
// only the fields the test reasons about.
type junitReport struct {
	XMLName  xml.Name `xml:"testsuites"`
	Tests    int      `xml:"tests,attr"`
	Failures int      `xml:"failures,attr"`
	Suites   []struct {
		XMLName  xml.Name `xml:"testsuite"`
		Tests    int      `xml:"tests,attr"`
		Failures int      `xml:"failures,attr"`
		Cases    []struct {
			XMLName   xml.Name `xml:"testcase"`
			Classname string   `xml:"classname,attr"`
			Name      string   `xml:"name,attr"`
			Failure   *struct {
				Message string `xml:"message,attr"`
				Type    string `xml:"type,attr"`
			} `xml:"failure"`
		} `xml:"testcase"`
	} `xml:"testsuite"`
}

// TestCheckJUnitFormat exercises the --format junit path end-to-end: the
// scaffolded main() runs with WithFormat("junit") against the fixture
// project (which has real violations under this spec), and stdout must be
// a parseable JUnit XML report whose testcases carry classnames, relative
// file:line names and non-empty failure types. Exit code stays 1
// (violations found).
func TestCheckJUnitFormat(t *testing.T) {
	project := testProjectDir(t)
	root := repoRoot(t)
	dir := scaffoldArch(t, root, fmt.Sprintf(archJUnitTpl, project))

	stdout, stderr, exitCode := runArchLint(t, dir)
	require.Equal(t, 1, exitCode, "exit code = %d, want 1 (violations)\nstdout:\n%s\nstderr:\n%s; stdout:\n%s\nstderr:\n%s", stdout, stderr)

	var report junitReport
	assert.NoError(t, xml.Unmarshal([]byte(stdout), &report))
	require.Len(t, report.Suites, 1, "expected 1 testsuite\nstdout:\n%s", stdout)

	suite := report.Suites[0]
	assert.NotEmpty(t, suite.Cases)
	assert.Equal(t, len(suite.Cases), suite.Tests, "suite tests (one per testcase)")
	assert.NotZero(t, suite.Failures, "failures must be consistent")
	assert.Equal(t, suite.Failures, report.Failures, "failures must be consistent suite=%d envelope=%d", suite.Failures, report.Failures)

	for _, tc := range suite.Cases {
		assert.NotEmpty(t, tc.Classname, "testcase without classname: %+v", tc)
		assert.NotEmpty(t, tc.Name, "testcase name must be a relative file[:line]")
		assert.False(t, strings.HasPrefix(tc.Name, "/"), "testcase name must be a relative file[:line], got %q", tc.Name)
		if assert.NotNil(t, tc.Failure, "violation testcase must carry a failure: %+v", tc) {
			assert.NotEmpty(t, tc.Failure.Type, "failure must carry type and message: %+v", tc.Failure)
			assert.NotEmpty(t, tc.Failure.Message, "failure must carry type and message: %+v", tc.Failure)
		}
	}
}
