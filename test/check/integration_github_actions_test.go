package check_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// archGitHubActionsTpl mirrors archJUnitTpl but renders the GitHub Actions
// workflow-command format.
const archGitHubActionsTpl = `package main

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
		archlint.WithFormat("github-actions"),
	)
}
`

// TestCheckGitHubActionsFormat exercises the --format github-actions path
// end-to-end: the scaffolded main() runs with WithFormat("github-actions")
// against the fixture project (which has real violations under this spec),
// and stdout must consist solely of workflow commands — ::error lines with
// file/line properties for blocking violations, relative file paths (no
// leading slash), and reserved characters percent-encoded. Exit code stays
// 1 (violations found).
func TestCheckGitHubActionsFormat(t *testing.T) {
	project := testProjectDir(t)
	root := repoRoot(t)
	dir := scaffoldArch(t, root, fmt.Sprintf(archGitHubActionsTpl, project))

	stdout, stderr, exitCode := runArchLint(t, dir)
	require.Equal(t, 1, exitCode,
		"want exit 1 (violations found); stdout:\n%s\nstderr:\n%s", stdout, stderr)

	var errors, notices int
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		require.True(t, strings.HasPrefix(line, "::"), "non workflow-command line in output: %q\nstdout:\n%s")
		switch {
		case strings.HasPrefix(line, "::error "):
			errors++
		case strings.HasPrefix(line, "::notice "):
			notices++
		default:
			require.FailNow(t, "unexpected workflow command (want ::error or ::notice)", "%q", line)
		}

		// Annotation files must be workspace-relative: GitHub resolves them
		// against the checkout root, so a leading '/' breaks the annotation.
		if prop, ok := workflowProperty(line, "file"); ok {
			assert.NotEmpty(t, prop, "annotation file must be non-empty, got %q in %q", prop, line)
			assert.False(t, strings.HasPrefix(prop, "/"), "annotation file must be relative, got %q in %q", prop, line)
		}

		// Exactly one "::" separator pair per command — a raw ':' in a value
		// would split the command.
		assert.Equal(t, 2, strings.Count(line, "::"), "command must contain exactly two '::' delimiters, got %q", line)
	}

	require.NotZero(t, errors, "expected ::error annotations for fixture violations, got none\nstdout:\n%s", stdout)
	assert.NotZero(t, notices, "expected at least one ::notice for the unmatched-file advisory, got none\nstdout:\n%s", stdout)
}

// workflowProperty extracts a property value from a workflow-command line
// (::error file=x,line=1::msg → "x" for "file"). Values are already
// percent-encoded; the test only reasons about structure, so no decoding
// is needed.
func workflowProperty(line, key string) (string, bool) {
	header := strings.TrimPrefix(line, "::")
	end := strings.Index(header, "::")
	if end < 0 {
		return "", false
	}
	for _, kv := range strings.Split(header[:end], ",") {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 && parts[0] == key {
			return parts[1], true
		}
	}
	return "", false
}
