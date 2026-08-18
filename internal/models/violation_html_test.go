package models

import (
	"strings"
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
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

	if report.Total != 3 {
		t.Fatalf("Total = %d, want 3", report.Total)
	}
	if report.ToolName != "go-arch-lint" || report.ToolVersion != "v2.0.1" {
		t.Fatalf("tool header = %s %s, want go-arch-lint v2.0.1", report.ToolName, report.ToolVersion)
	}

	if len(report.ByType) != 4 {
		t.Fatalf("ByType cards = %d, want 4 (dependency, match, deepscan, naming)", len(report.ByType))
	}
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
		if got.Type != w.typ || got.Label != w.label || got.Count != w.count {
			t.Errorf("ByType[%d] = %+v, want %s/%s/%d", i, got, w.typ, w.label, w.count)
		}
	}

	if report.OmittedCount != 3 || report.SuppressedCount != 2 {
		t.Errorf("cap counters = %d/%d, want 3/2", report.OmittedCount, report.SuppressedCount)
	}

	// File paths must be workspace-relative (leading slash stripped),
	// matching the SARIF/GitHub-Actions convention.
	for _, row := range report.Rows {
		if strings.HasPrefix(row.File, "/") {
			t.Errorf("row file %q must be workspace-relative", row.File)
		}
	}
	if report.Rows[0].File != htmlFixtureFileRel || report.Rows[0].Line != 10 {
		t.Errorf("first row = %+v, want %s:10", report.Rows[0], htmlFixtureFileRel)
	}
}

// TestToHTMLReport_CleanProject verifies the zero-violation report keeps
// the by-type cards (all zero) and carries the tool identity.
func TestToHTMLReport_CleanProject(t *testing.T) {
	report := CmdCheckOut{ModuleName: "m"}.ToHTMLReport("dev")

	if report.Total != 0 || len(report.Rows) != 0 {
		t.Fatalf("clean project must have zero rows, got %+v", report)
	}
	if len(report.ByType) != 4 {
		t.Fatalf("ByType cards = %d, want 4", len(report.ByType))
	}
	for _, c := range report.ByType {
		if c.Count != 0 {
			t.Errorf("clean project count for %s = %d, want 0", c.Type, c.Count)
		}
	}
}

// TestToHTMLReport_UnknownTypeSurfaces verifies that a violation type not
// in the known rule classes still gets a card and a label (no silent
// drops), mirroring the SARIF/JUnit fallback behavior.
func TestToHTMLReport_UnknownTypeSurfaces(t *testing.T) {
	v := Violation{Type: "future-kind", File: "/a.go", Rule: "some rule"}
	if got := htmlTypeLabel(v.Type); got != "future-kind" {
		t.Fatalf("unknown type label = %q, want the raw type", got)
	}
}
