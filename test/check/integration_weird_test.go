package check_test

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Weird-path fixture: directories with spaces and "::" (legal on unix,
// hostile to URI encoders, workflow-command parsers, and XML escapers).
// The synthetic format run on such a fixture found all three machine
// formats handle it; this test pins that end-to-end.

const weirdDirName = "wei rd::name"

// writeWeirdProject creates a throwaway project with a violating import
// inside a weird-named directory and returns its root.
func writeWeirdProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module weird.test\n\ngo 1.25\n"), 0o600))
	base := filepath.Join(root, "internal", weirdDirName)
	for _, comp := range []string{"x", "y"} {
		require.NoError(t, os.MkdirAll(filepath.Join(base, comp), 0o755))
	}
	xSrc := "package x\n\nimport _ \"weird.test/internal/" + weirdDirName + "/y\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(base, "x", "a.go"), []byte(xSrc), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(base, "y", "b.go"), []byte("package y\n"), 0o600))
	return root
}

const archWeirdTpl = `package main

import (
	"github.com/vsfedorenko/go-arch-lint"
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

func main() {
	spec := Spec(func() {
		Version(1)
		Workdir("internal")
		Component("x", "wei rd::name/x")
		Component("y", "wei rd::name/y")
		Deps("x", func() { AnyVendorDeps(true) })
		Deps("y", func() { AnyVendorDeps(true) })
	})
	archlint.MustRun(spec, archlint.WithProjectPath(%q), archlint.WithFormat(%q))
}
`

// sarifWeirdLog decodes just the fields the weird-path assertions need.
type sarifWeirdLog struct {
	Runs []struct {
		Results []struct {
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

// runWeirdFormat scaffolds the weird project's arch module and runs it
// with the given --format flag, returning stdout plus the child exit code
// (same env discipline as runArchLint: offline, checksums copied).
func runWeirdFormat(t *testing.T, format string) (string, int) {
	t.Helper()
	root := repoRoot(t)
	dir := scaffoldArch(t, root, fmt.Sprintf(archWeirdTpl, writeWeirdProject(t), format))

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
	code := 0
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			code = parseChildExitCode(errb.String(), exitErr.ExitCode())
		} else {
			require.NoError(t, err, "go run:\nstderr:\n%s", errb.String())
		}
	}
	return out.String(), code
}

func TestWeirdPaths_SARIF(t *testing.T) {
	stdout, code := runWeirdFormat(t, "sarif")
	require.Equal(t, 1, code, "exit code = %d, want 1\nstdout:\n%s")

	var log sarifWeirdLog
	assert.Equal(t, nil, json.Unmarshal([]byte(stdout), &log))
	require.Len(t, log.Runs, 1, "expected results, got %+v", log.Runs)
	require.NotEmpty(t, log.Runs[0].Results, "expected results, got %+v", log.Runs)
	for _, res := range log.Runs[0].Results {
		for _, loc := range res.Locations {
			uri := loc.PhysicalLocation.ArtifactLocation.URI
			// The URI must contain the raw weird path (spaces and ::) —
			// SARIF artifactLocation.uri is a string, not URL-encoded.
			assert.Contains(t, uri, weirdDirName, "uri does not contain the weird dir")
			assert.False(t, strings.HasPrefix(uri, "/"), "uri must be relative: %q", uri)
			assert.GreaterOrEqual(t, loc.PhysicalLocation.Region.StartLine, 1, "startLine must be >= 1")
		}
	}
}

// junitWeird decodes the JUnit XML enough to assert escaping.
type junitWeird struct {
	XMLName    xml.Name `xml:"testsuites"`
	TestSuites []struct {
		TestCases []struct {
			Name    string `xml:"name,attr"`
			Failure struct {
				Message string `xml:"message,attr"`
				Body    string `xml:",chardata"`
			} `xml:"failure"`
		} `xml:"testcase"`
	} `xml:"testsuite"`
}

func TestWeirdPaths_JUnit(t *testing.T) {
	stdout, code := runWeirdFormat(t, "junit")
	require.Equal(t, 1, code, "exit code = %d, want 1\nstdout:\n%s")

	var log junitWeird
	assert.Equal(t, nil, xml.Unmarshal([]byte(stdout), &log))
	for _, ts := range log.TestSuites {
		for _, tc := range ts.TestCases {
			// The :: and spaces must survive XML escaping round-trip.
			assert.Contains(t, tc.Name, weirdDirName, "testcase name lost the weird dir: %q", tc.Name)
			assert.Contains(t, tc.Failure.Message, weirdDirName, "failure message lost the weird dir: %q", tc.Failure.Message)
		}
	}
}

func TestWeirdPaths_GitHubActions(t *testing.T) {
	stdout, code := runWeirdFormat(t, "github-actions")
	require.Equal(t, 1, code, "exit code = %d, want 1\nstdout:\n%s")

	var annotation string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "::error ") || strings.HasPrefix(line, "::notice ") {
			annotation = line
			break
		}
	}
	require.NotEmpty(t, annotation, "no workflow command in output:\n%s", stdout)

	// Parse ::error file=F,line=N,col=C,title=T::msg
	propsStr := annotation[strings.Index(annotation, " ")+1:]
	propsStr = propsStr[:strings.Index(propsStr, "::")]
	for _, part := range strings.Split(propsStr, ",") {
		k, v, ok := strings.Cut(part, "=")
		require.True(t, ok, "malformed property %q in %q")
		if k != "file" {
			continue
		}
		decoded, err := url.QueryUnescape(v)
		require.NoError(t, err, "file property is not percent-encoded properly: %q", v)
		assert.Contains(t, decoded, weirdDirName, "decoded file lost the weird dir (raw %q)", v)
		assert.NotContains(t, v, "::", "raw file property must not contain unescaped '::' (would break workflow-command parsing): %q", v)
	}
}
