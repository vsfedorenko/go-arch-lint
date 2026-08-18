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
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module weird.test\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, "internal", weirdDirName)
	for _, comp := range []string{"x", "y"} {
		if err := os.MkdirAll(filepath.Join(base, comp), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	xSrc := "package x\n\nimport _ \"weird.test/internal/" + weirdDirName + "/y\"\n"
	if err := os.WriteFile(filepath.Join(base, "x", "a.go"), []byte(xSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "y", "b.go"), []byte("package y\n"), 0o600); err != nil {
		t.Fatal(err)
	}
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
			t.Fatalf("go run: %v\nstderr:\n%s", err, errb.String())
		}
	}
	return out.String(), code
}

func TestWeirdPaths_SARIF(t *testing.T) {
	stdout, code := runWeirdFormat(t, "sarif")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1\nstdout:\n%s", code, stdout)
	}

	var log sarifWeirdLog
	if err := json.Unmarshal([]byte(stdout), &log); err != nil {
		t.Fatalf("stdout is not SARIF JSON: %v\n%s", err, stdout)
	}
	if len(log.Runs) != 1 || len(log.Runs[0].Results) == 0 {
		t.Fatalf("expected results, got %+v", log.Runs)
	}
	for _, res := range log.Runs[0].Results {
		for _, loc := range res.Locations {
			uri := loc.PhysicalLocation.ArtifactLocation.URI
			// The URI must contain the raw weird path (spaces and ::) —
			// SARIF artifactLocation.uri is a string, not URL-encoded.
			if !strings.Contains(uri, weirdDirName) {
				t.Errorf("uri %q does not contain the weird dir %q", uri, weirdDirName)
			}
			if strings.HasPrefix(uri, "/") {
				t.Errorf("uri must be relative: %q", uri)
			}
			if loc.PhysicalLocation.Region.StartLine < 1 {
				t.Errorf("startLine must be >= 1: %d", loc.PhysicalLocation.Region.StartLine)
			}
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
	if code != 1 {
		t.Fatalf("exit code = %d, want 1\nstdout:\n%s", code, stdout)
	}

	var log junitWeird
	if err := xml.Unmarshal([]byte(stdout), &log); err != nil {
		t.Fatalf("stdout is not well-formed JUnit XML: %v\n%s", err, stdout)
	}
	for _, ts := range log.TestSuites {
		for _, tc := range ts.TestCases {
			// The :: and spaces must survive XML escaping round-trip.
			if !strings.Contains(tc.Name, weirdDirName) {
				t.Errorf("testcase name %q lost the weird dir", tc.Name)
			}
			if !strings.Contains(tc.Failure.Message, weirdDirName) {
				t.Errorf("failure message %q lost the weird dir", tc.Failure.Message)
			}
		}
	}
}

func TestWeirdPaths_GitHubActions(t *testing.T) {
	stdout, code := runWeirdFormat(t, "github-actions")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1\nstdout:\n%s", code, stdout)
	}

	var annotation string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "::error ") || strings.HasPrefix(line, "::notice ") {
			annotation = line
			break
		}
	}
	if annotation == "" {
		t.Fatalf("no workflow command in output:\n%s", stdout)
	}

	// Parse ::error file=F,line=N,col=C,title=T::msg
	propsStr := annotation[strings.Index(annotation, " ")+1:]
	propsStr = propsStr[:strings.Index(propsStr, "::")]
	for _, part := range strings.Split(propsStr, ",") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			t.Fatalf("malformed property %q in %q", part, annotation)
		}
		if k != "file" {
			continue
		}
		decoded, err := url.QueryUnescape(v)
		if err != nil {
			t.Fatalf("file property is not percent-encoded properly: %q: %v", v, err)
		}
		if !strings.Contains(decoded, weirdDirName) {
			t.Errorf("decoded file %q lost the weird dir (raw %q)", decoded, v)
		}
		if strings.Contains(v, "::") {
			t.Errorf("raw file property %q must not contain unescaped '::' (would break workflow-command parsing)", v)
		}
	}
}
