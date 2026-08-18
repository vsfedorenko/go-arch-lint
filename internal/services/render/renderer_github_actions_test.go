package render

import (
	"strings"
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

// TestRenderModel_FormatGitHubActions_CheckOut verifies the --format
// github-actions fast path: one workflow command per violation, errors for
// blocking kinds and a notice for the advisory "match" kind, with
// file/line/col properties pointing at the offending source location.
func TestRenderModel_FormatGitHubActions_CheckOut(t *testing.T) {
	out := gaViolationFixture()
	out.ArchWarningsMatch = []models.CheckArchWarningMatch{
		{FileRelativePath: "/internal/orphan/x.go"},
	}

	r, buf := newTestRenderer(t, models.FormatGitHubActions)

	// UserSpaceError is expected: it means "violations found". RenderModel
	// renders the model AND returns the error for exit-code mapping.
	if err := r.RenderModel(out, models.NewUserSpaceError("check not successful")); err != nil {
		if !models.IsUserSpaceError(err) {
			t.Fatalf("RenderModel: unexpected error: %v", err)
		}
	}

	lines := nonEmptyLines(buf.String())
	if len(lines) != 2 {
		t.Fatalf("expected 2 workflow commands, got %d:\n%s", len(lines), buf.String())
	}

	first := lines[0]
	// Leading slash must be stripped: annotations resolve relative to the
	// workspace root, and "/internal/..." is not a workspace-relative path.
	assertTrue(t, strings.HasPrefix(first, "::error file=internal/handler/user.go,line=10,col=2,title=go-arch-lint handler::"),
		"dependency violation must be an ::error with file/line/col/title, got: %s", first)
	assertTrue(t, strings.Contains(first, "may not depend on \""+gaFixtureImport+"\""),
		"message must carry the violated rule verbatim, got: %s", first)

	second := lines[1]
	assertTrue(t, strings.HasPrefix(second, "::notice file=internal/orphan/x.go,title=go-arch-lint::"),
		"unmatched-file violation must be a ::notice without line, got: %s", second)
}

// TestRenderModel_FormatGitHubActions_NoViolations verifies the clean path:
// a single ::notice confirming zero violations (and no ::error lines).
func TestRenderModel_FormatGitHubActions_NoViolations(t *testing.T) {
	out := models.CmdCheckOut{ModuleName: gaFixtureModule}

	r, buf := newTestRenderer(t, models.FormatGitHubActions)

	if err := r.RenderModel(out, nil); err != nil {
		t.Fatalf("RenderModel: unexpected error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	assertTrue(t, output == "::notice ::go-arch-lint: no architecture violations found",
		"expected the clean-project notice, got: %q", output)
}

// TestRenderModel_FormatGitHubActions_Escaping verifies reserved characters
// in properties and messages are percent-encoded per the workflow-command
// grammar so a single command never splits into several. Double quotes are
// NOT reserved (the runner only reserves %, :, , and = in values) and stay
// raw.
func TestRenderModel_FormatGitHubActions_Escaping(t *testing.T) {
	out := models.CmdCheckOut{
		ModuleName: gaFixtureModule,
		ArchWarningsNaming: []models.CheckArchWarningNaming{
			{
				PackageName:      "utils",
				PackagePath:      "/internal/pkg/utils",
				FileRelativePath: "/internal/pkg/utils/a.go",
				FilesCount:       3,
			},
		},
	}

	r, buf := newTestRenderer(t, models.FormatGitHubActions)

	if err := r.RenderModel(out, models.NewUserSpaceError("check not successful")); err != nil {
		if !models.IsUserSpaceError(err) {
			t.Fatalf("RenderModel: unexpected error: %v", err)
		}
	}

	lines := nonEmptyLines(buf.String())
	if len(lines) != 1 {
		t.Fatalf("expected 1 workflow command, got %d:\n%s", len(lines), buf.String())
	}

	line := lines[0]
	assertTrue(t, strings.HasPrefix(line, "::error file=internal/pkg/utils/a.go,title=go-arch-lint::"),
		"naming violation must carry the package file, got: %s", line)
	assertTrue(t, strings.Contains(line, `package name "utils" is forbidden`),
		"non-reserved characters (quotes) must stay raw, got: %s", line)
	assertTrue(t, strings.Contains(line, "%2C 3 file(s))"),
		"the ',' inside the message must be encoded so the command cannot split, got: %s", line)

	// Every literal occurrence of ':' inside values must be encoded, so
	// exactly one "::" separator pair exists (the command's own delimiters).
	assertTrue(t, strings.Count(line, "::") == 2,
		"command must contain exactly the two '::' delimiters, got: %s", line)
}

// TestRenderModel_FormatGitHubActions_ConfigError verifies the failure
// path: when the check could not run (broken spec), the format emits ONE
// ::error naming the config problem — never the "no violations" notice
// that would read as a green check.
func TestRenderModel_FormatGitHubActions_ConfigError(t *testing.T) {
	out := models.CmdCheckOut{ModuleName: gaFixtureModule}

	r, buf := newTestRenderer(t, models.FormatGitHubActions)

	if err := r.RenderModel(out, models.NewConfigError("at least one component must be defined")); err != nil {
		if !models.IsConfigError(err) {
			t.Fatalf("RenderModel: unexpected error: %v", err)
		}
	}

	lines := nonEmptyLines(buf.String())
	if len(lines) != 1 {
		t.Fatalf("expected 1 workflow command, got %d:\n%s", len(lines), buf.String())
	}

	line := lines[0]
	assertTrue(t, strings.HasPrefix(line, "::error title=go-arch-lint::configuration error"),
		"config error must surface as ::error, got: %s", line)
	assertTrue(t, strings.Contains(line, "at least one component must be defined"),
		"the config error message must be carried, got: %s", line)
}

// TestRenderModel_FormatGitHubActions_NonCheckModelFallsBackToJSON verifies
// that other command models keep the wrapped {Type, Payload} JSON output
// under --format github-actions (the flag only reshapes check results).
func TestRenderModel_FormatGitHubActions_NonCheckModelFallsBackToJSON(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatGitHubActions)

	if err := r.RenderModel(models.CmdVersionOut{LinterVersion: "v9.9.9"}, nil); err != nil {
		t.Fatalf("RenderModel: unexpected error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	assertTrue(t, strings.HasPrefix(output, "{"), "expected wrapped JSON fallback, got: %s", firstRunes(output, 40))
	assertTrue(t, strings.Contains(output, `"Payload"`), "expected {Type, Payload} wrapper, got: %s", firstRunes(output, 60))
}

// nonEmptyLines splits rendered output into lines, dropping the trailing
// empty line from the final newline.
func nonEmptyLines(s string) []string {
	var lines []string
	for _, ln := range strings.Split(strings.TrimSpace(s), "\n") {
		if ln != "" {
			lines = append(lines, ln)
		}
	}
	return lines
}
