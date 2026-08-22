package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
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
		require.True(t, models.IsUserSpaceError(err), "RenderModel: unexpected error: %v", err)
	}

	doc := buf.String()

	// Document shape: one complete HTML document, emitted once.
	assert.True(t, strings.HasPrefix(strings.TrimSpace(doc), "<!DOCTYPE html>"),
		"document must start with <!DOCTYPE html>, got: %.80s", doc)
	assert.Equal(t, 1, strings.Count(doc, "<!DOCTYPE html>"), "exactly one document")

	// Header identity.
	assert.Contains(t, doc, "module <code>github.com/x/proj</code>",
		"header must carry the module name")

	// Violations present: one dependency row, one match row.
	assert.Contains(t, doc, "internal/handler/user.go:10",
		"dependency row must point at file:line")
	assert.Contains(t, doc, "internal/orphan/x.go", "match row must be present")
	assert.Contains(t, doc, gaFixtureImport, "dependency column must carry the import")

	// Type cards: totals per class.
	assert.Contains(t, doc, "<div class=\"n\">2</div>", "total violations card must show 2")

	// Every rule id cell must be tagged with the violation type so CSS
	// color coding works.
	assert.Contains(t, doc, `class="tag dependency"`, "dependency tag missing")
	assert.Contains(t, doc, `class="tag match"`, "match tag missing")
}

// TestRenderModel_FormatHTML_NoViolations verifies the clean path: a
// valid document with the zero card and the "no violations" line — never
// an empty output.
func TestRenderModel_FormatHTML_NoViolations(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatHTML)

	require.NoError(t, r.RenderModel(models.CmdCheckOut{ModuleName: gaFixtureModule}, nil))

	doc := buf.String()
	assert.Contains(t, doc, "<!DOCTYPE html>", "must be a document")
	assert.Contains(t, doc, "No architecture violations found", "clean project notice missing")
	assert.Contains(t, doc, "<div class=\"n\">0</div>", "total card must show 0")
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
		require.True(t, models.IsUserSpaceError(err), "RenderModel: unexpected error: %v", err)
	}

	doc := buf.String()
	assert.NotContains(t, doc, "<script>", "raw <script> must not survive escaping")
	assert.NotContains(t, doc, "util<s", "raw < in file path must be escaped")
	assert.Contains(t, doc, "&lt;script&gt;",
		"the hostile value must still be VISIBLE (escaped), not dropped")
}

// TestRenderModel_FormatHTML_ConfigError verifies a configuration error
// renders the error banner document — an empty green report would wrongly
// read as a passing check.
func TestRenderModel_FormatHTML_ConfigError(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatHTML)

	err := models.NewConfigError("spec broken <&>")
	out := models.CmdCheckOut{ModuleName: gaFixtureModule}

	renderErr := r.RenderModel(out, err)
	require.Error(t, renderErr, "RenderModel must still return the config error for exit-code mapping")

	doc := buf.String()
	assert.Contains(t, doc, "configuration error", "config error banner missing")
	assert.Contains(t, doc, "spec broken &lt;&amp;&gt;", "config error text must be escaped but visible")
}

// TestRenderModel_FormatHTML_NonCheckModelFallsBackToJSON verifies the
// flag stays safe on other commands (e.g. version), mirroring the other
// machine formats.
func TestRenderModel_FormatHTML_NonCheckModelFallsBackToJSON(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatHTML)

	require.NoError(t, r.RenderModel(models.CmdVersionOut{LinterVersion: "1.2.3"}, nil))

	doc := buf.String()
	assert.Contains(t, doc, `"Type": "models.Version"`,
		"non-check model must fall back to wrapped JSON")
}
