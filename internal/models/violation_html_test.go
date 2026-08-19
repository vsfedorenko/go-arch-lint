package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

// htmlFixtureFileRel is the workspace-relative form of junitFixtureDepFile
// (leading slash stripped) — what the HTML rows must carry.
const htmlFixtureFileRel = "internal/handler/user.go"

// TestToHTMLReport verifies the HTMLReport conversion: counts per type in
// the stable rule order, project-relative file paths, and the display-cap
// counters carried over from the check output.
func TestToHTMLReport(t *testing.T) {
	out := CmdCheckOut{
		ModuleName: junitFixtureModule,
		ArchWarningsDependency: []CheckArchWarningDependency{
			{
				ComponentName:      junitFixtureComponent,
				FileRelativePath:   junitFixtureDepFile,
				ResolvedImportName: junitFixtureDepImport,
				Reference:          domain.NewReferenceSingleLine(junitFixtureDepFile, 10, 2),
			},
		},
		ArchWarningsMatch: []CheckArchWarningMatch{
			{FileRelativePath: "/internal/orphan/x.go"},
			{FileRelativePath: "/internal/orphan/y.go"},
		},
		OmittedCount:    3,
		SuppressedCount: 2,
	}

	report := out.ToHTMLReport("v2.0.1")

	assert.Equal(t, 3, report.Total, "Total")
	assert.Equal(t, "go-arch-lint", report.ToolName, "tool name")
	assert.Equal(t, "v2.0.1", report.ToolVersion, "tool version")

	require.Len(t, report.ByType, 4, "ByType cards (dependency, match, deepscan, naming)")
	want := []struct {
		typ   string
		label string
		count int
	}{
		{"dependency", "Dependency", 1},
		{"match", "Not matched", 2},
		{"deepscan", "DeepScan", 0},
		{"naming", "Naming", 0},
	}
	for i, w := range want {
		got := report.ByType[i]
		assert.Equal(t, w.typ, got.Type, "ByType[%d].Type", i)
		assert.Equal(t, w.label, got.Label, "ByType[%d].Label", i)
		assert.Equal(t, w.count, got.Count, "ByType[%d].Count", i)
	}

	assert.Equal(t, 3, report.OmittedCount, "OmittedCount")
	assert.Equal(t, 2, report.SuppressedCount, "SuppressedCount")

	// File paths must be workspace-relative (leading slash stripped),
	// matching the SARIF/GitHub-Actions convention.
	for _, row := range report.Rows {
		assert.False(t, strings.HasPrefix(row.File, "/"), "row file %q must be workspace-relative", row.File)
	}
	assert.Equal(t, htmlFixtureFileRel, report.Rows[0].File, "first row file")
	assert.Equal(t, 10, report.Rows[0].Line, "first row line")
}

// TestToHTMLReport_CleanProject verifies the zero-violation report keeps
// the by-type cards (all zero) and carries the tool identity.
func TestToHTMLReport_CleanProject(t *testing.T) {
	report := CmdCheckOut{ModuleName: "m"}.ToHTMLReport("dev")

	assert.Equal(t, 0, report.Total, "clean project total")
	assert.Empty(t, report.Rows, "clean project must have zero rows")
	require.Len(t, report.ByType, 4)
	for _, c := range report.ByType {
		assert.Equal(t, 0, c.Count, "clean project count for %s", c.Type)
	}
}

// TestToHTMLReport_UnknownTypeSurfaces verifies that a violation type not
// in the known rule classes still gets a card and a label (no silent
// drops), mirroring the SARIF/JUnit fallback behavior.
func TestToHTMLReport_UnknownTypeSurfaces(t *testing.T) {
	v := Violation{Type: "future-kind", File: "/a.go", Rule: "some rule"}
	assert.Equal(t, "future-kind", htmlTypeLabel(v.Type), "unknown type label must be the raw type")
}
