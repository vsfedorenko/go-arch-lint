package check_test

import (
	"fmt"
	"strings"
	"testing"
)

// The HTML report format: a standalone, self-contained document emitted
// by the delegated check run. These tests exercise the real binary path
// end-to-end (scaffold + go run), not the renderer in isolation, so they
// pin what a CI artifact consumer actually receives.

// archHTMLTpl mirrors archGitHubActionsTpl but renders the HTML format.
const archHTMLTpl = `package main

import (
	"github.com/vsfedorenko/go-arch-lint"
	. "github.com/vsfedorenko/go-arch-lint/dsl"
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
		archlint.WithFormat("html"),
	)
}
`

// TestCheckHTMLFormat exercises the --format html path end-to-end: the
// scaffolded main() runs with WithFormat("html") against the fixture
// project (which has real violations under this spec), and stdout must be
// a complete HTML document: tool identity, violation table rows with
// file:line, per-type tags. Exit code stays 1 (violations found).
func TestCheckHTMLFormat(t *testing.T) {
	project := testProjectDir(t)
	root := repoRoot(t)
	dir := scaffoldArch(t, root, fmt.Sprintf(archHTMLTpl, project))

	stdout, stderr, exitCode := runArchLint(t, dir)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1 (violations found)\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}

	if !strings.HasPrefix(strings.TrimSpace(stdout), "<!DOCTYPE html>") {
		t.Fatalf("output must be an HTML document, got: %.120s", stdout)
	}

	// Identity: tool name in the header.
	if !strings.Contains(stdout, "go-arch-lint") {
		t.Errorf("report must identify the tool, got:\n%s", stdout)
	}

	// Violation content: the fixture project has dependency violations
	// (a→b forbidden etc.) — the table must reference internal/ files.
	if !strings.Contains(stdout, "internal/") {
		t.Errorf("violation table must reference project files, got:\n%s", stdout)
	}

	// Structure: one table with the file column markup.
	if !strings.Contains(stdout, "<table>") {
		t.Errorf("report must contain the violation table, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, `<td class="file">`) {
		t.Errorf("report must use the file column markup, got:\n%s", stdout)
	}

	// html/template guarantees no raw script markup can survive.
	if strings.Contains(stdout, "<script>") {
		t.Errorf("unescaped script tag in report:\n%s", stdout)
	}
}

// TestCheckHTMLFormat_WeirdPaths pins the escaping contract on paths with
// spaces and "::" — the same hostile fixture the SARIF/JUnit/GitHubActions
// weird-path tests use: the violation must be present (not silently
// dropped) and the document stays well-formed.
func TestCheckHTMLFormat_WeirdPaths(t *testing.T) {
	stdout, exitCode := runWeirdFormat(t, "html")
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1\nstdout:\n%s", exitCode, stdout)
	}

	if !strings.HasPrefix(strings.TrimSpace(stdout), "<!DOCTYPE html>") {
		t.Fatalf("output must be an HTML document, got: %.120s", stdout)
	}

	// The weird directory must appear in the report — otherwise the
	// violation was silently dropped. html/template keeps spaces and
	// colons readable as plain text.
	if !strings.Contains(stdout, weirdDirName) {
		t.Errorf("weird directory name must be present in the report, got:\n%s", stdout)
	}

	// A well-formed document has exactly one table.
	if strings.Count(stdout, "<table>") != 1 {
		t.Errorf("exactly one violation table expected, got %d\n%s", strings.Count(stdout, "<table>"), stdout)
	}
}
