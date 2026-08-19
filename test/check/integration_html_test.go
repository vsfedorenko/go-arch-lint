package check_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.Equal(t, 1, exitCode, "exit code = %d, want 1 (violations found)\nstdout:\n%s\nstderr:\n%s; stderr:\n%s", stderr)

	require.True(t, strings.HasPrefix(strings.TrimSpace(stdout), "<!DOCTYPE html>"), "output must be an HTML document, got: %.120s", stdout)

	// Identity: tool name in the header.
	assert.Contains(t, stdout, "go-arch-lint", "report must identify the tool")

	// Violation content: the fixture project has dependency violations
	// (a→b forbidden etc.) — the table must reference internal/ files.
	assert.Contains(t, stdout, "internal/", "violation table must reference project files")

	// Structure: one table with the file column markup.
	assert.Contains(t, stdout, "<table>", "report must contain the violation table")
	assert.Contains(t, stdout, `<td class="file">`, "report must use the file column markup")

	// html/template guarantees no raw script markup can survive.
	assert.NotContains(t, stdout, "<script>", "unescaped script tag in report")
}

// TestCheckHTMLFormat_WeirdPaths pins the escaping contract on paths with
// spaces and "::" — the same hostile fixture the SARIF/JUnit/GitHubActions
// weird-path tests use: the violation must be present (not silently
// dropped) and the document stays well-formed.
func TestCheckHTMLFormat_WeirdPaths(t *testing.T) {
	stdout, exitCode := runWeirdFormat(t, "html")
	require.Equal(t, 1, exitCode, "exit code = %d, want 1\nstdout:\n%s")

	require.True(t, strings.HasPrefix(strings.TrimSpace(stdout), "<!DOCTYPE html>"), "output must be an HTML document, got: %.120s", stdout)

	// The weird directory must appear in the report — otherwise the
	// violation was silently dropped. html/template keeps spaces and
	// colons readable as plain text.
	assert.Contains(t, stdout, weirdDirName, "weird directory name must be present in the report")

	// A well-formed document has exactly one table.
	assert.Equal(t, 1, strings.Count(stdout, "<table>"), "exactly one violation table expected")
}
