package render

import (
	"strings"
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

// TestRenderModel_FormatHTML_CheckOut verifies the --format html fast
// path: a standalone HTML document with the violation table, type cards,
// and the tool/module identity in the header.
func TestRenderModel_FormatHTML_CheckOut(t *testing.T) {
	out := gaViolationFixture()
	out.ArchWarningsMatch = []models.CheckArchWarningMatch{
		{FileRelativePath: "/internal/orphan/x.go"},
	}

	r, buf := newTestRenderer(t, models.FormatHTML)

	// UserSpaceError is expected: it means "violations found". RenderModel
	// renders the model AND returns the error for exit-code mapping.
	if err := r.RenderModel(out, models.NewUserSpaceError("check not successful")); err != nil {
		if !models.IsUserSpaceError(err) {
			t.Fatalf("RenderModel: unexpected error: %v", err)
		}
	}

	doc := buf.String()

	// Document shape: one complete HTML document, emitted once.
	assertTrue(t, strings.HasPrefix(strings.TrimSpace(doc), "<!DOCTYPE html>"),
		"document must start with <!DOCTYPE html>, got: %.80s", doc)
	assertTrue(t, strings.Count(doc, "<!DOCTYPE html>") == 1,
		"exactly one document, got %d", strings.Count(doc, "<!DOCTYPE html>"))

	// Header identity.
	assertTrue(t, strings.Contains(doc, "module <code>github.com/x/proj</code>"),
		"header must carry the module name, got: %s", doc)

	// Violations present: one dependency row, one match row.
	assertTrue(t, strings.Contains(doc, "internal/handler/user.go:10"),
		"dependency row must point at file:line, got: %s", doc)
	assertTrue(t, strings.Contains(doc, "internal/orphan/x.go"),
		"match row must be present, got: %s", doc)
	assertTrue(t, strings.Contains(doc, gaFixtureImport),
		"dependency column must carry the import, got: %s", doc)

	// Type cards: totals per class.
	assertTrue(t, strings.Contains(doc, "<div class=\"n\">2</div>"),
		"total violations card must show 2, got: %s", doc)

	// Every rule id cell must be tagged with the violation type so CSS
	// color coding works.
	assertTrue(t, strings.Contains(doc, `class="tag dependency"`),
		"dependency tag missing, got: %s", doc)
	assertTrue(t, strings.Contains(doc, `class="tag match"`),
		"match tag missing, got: %s", doc)
}

// TestRenderModel_FormatHTML_NoViolations verifies the clean path: a
// valid document with the zero card and the "no violations" line — never
// an empty output.
func TestRenderModel_FormatHTML_NoViolations(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatHTML)

	if err := r.RenderModel(models.CmdCheckOut{ModuleName: gaFixtureModule}, nil); err != nil {
		t.Fatalf("RenderModel: unexpected error: %v", err)
	}

	doc := buf.String()
	assertTrue(t, strings.Contains(doc, "<!DOCTYPE html>"), "must be a document, got: %s", doc)
	assertTrue(t, strings.Contains(doc, "No architecture violations found"),
		"clean project notice missing, got: %s", doc)
	assertTrue(t, strings.Contains(doc, "<div class=\"n\">0</div>"),
		"total card must show 0, got: %s", doc)
}

// TestRenderModel_FormatHTML_Escaping verifies hostile values (angle
// brackets, quotes, script tags inside file names and package paths —
// legal on unix) are escaped so they cannot break out of the HTML.
func TestRenderModel_FormatHTML_Escaping(t *testing.T) {
	out := models.CmdCheckOut{
		ModuleName: gaFixtureModule,
		ArchWarningsNaming: []models.CheckArchWarningNaming{
			{
				PackageName:      `utils"<script>alert(1)</script>`,
				PackagePath:      "/internal/pkg/util<s",
				FileRelativePath: "/internal/pkg/util<s/a.go",
				FilesCount:       1,
			},
		},
	}

	r, buf := newTestRenderer(t, models.FormatHTML)
	if err := r.RenderModel(out, models.NewUserSpaceError("check not successful")); err != nil {
		if !models.IsUserSpaceError(err) {
			t.Fatalf("RenderModel: unexpected error: %v", err)
		}
	}

	doc := buf.String()
	assertTrue(t, !strings.Contains(doc, "<script>"),
		"raw <script> must not survive escaping, got: %s", doc)
	assertTrue(t, !strings.Contains(doc, "util<s"),
		"raw < in file path must be escaped, got: %s", doc)
	assertTrue(t, strings.Contains(doc, "&lt;script&gt;"),
		"the hostile value must still be VISIBLE (escaped), not dropped, got: %s", doc)
}

// TestRenderModel_FormatHTML_ConfigError verifies a configuration error
// renders the error banner document — an empty green report would wrongly
// read as a passing check.
func TestRenderModel_FormatHTML_ConfigError(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatHTML)

	err := models.NewConfigError("spec broken <&>")
	out := models.CmdCheckOut{ModuleName: gaFixtureModule}

	renderErr := r.RenderModel(out, err)
	if renderErr == nil {
		t.Fatal("RenderModel must still return the config error for exit-code mapping")
	}

	doc := buf.String()
	assertTrue(t, strings.Contains(doc, "configuration error"),
		"config error banner missing, got: %s", doc)
	assertTrue(t, strings.Contains(doc, "spec broken &lt;&amp;&gt;"),
		"config error text must be escaped but visible, got: %s", doc)
}

// TestRenderModel_FormatHTML_NonCheckModelFallsBackToJSON verifies the
// flag stays safe on other commands (e.g. version), mirroring the other
// machine formats.
func TestRenderModel_FormatHTML_NonCheckModelFallsBackToJSON(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatHTML)

	if err := r.RenderModel(models.CmdVersionOut{LinterVersion: "1.2.3"}, nil); err != nil {
		t.Fatalf("RenderModel: unexpected error: %v", err)
	}

	doc := buf.String()
	assertTrue(t, strings.Contains(doc, `"Type": "models.Version"`),
		"non-check model must fall back to wrapped JSON, got: %s", doc)
}
