package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

// TestRenderModel_FormatSARIF_CheckOut verifies the --format sarif fast
// path: a full SARIF 2.1.0 log (not the wrapped {Type, Payload} model),
// with one result per violation and the driver version injected via
// WithDriverVersion.
func TestRenderModel_FormatSARIF_CheckOut(t *testing.T) {
	out := models.CmdCheckOut{
		ModuleName: gaFixtureModule,
		ArchWarningsDependency: []models.CheckArchWarningDependency{
			{
				ComponentName:      gaFixtureComponent,
				FileRelativePath:   "internal/handler/user.go",
				ResolvedImportName: gaFixtureImport,
				Reference:          domain.NewReferenceSingleLine("internal/handler/user.go", 10, 2),
			},
		},
		ArchWarningsMatch: []models.CheckArchWarningMatch{
			{FileRelativePath: "internal/orphan/x.go"},
		},
	}

	r, buf := newTestRenderer(t, models.FormatSARIF)
	r.WithDriverVersion("v9.9.9")

	// UserSpaceError is expected: it means "violations found". RenderModel
	// renders the model AND returns the error for exit-code mapping.
	if err := r.RenderModel(out, models.NewUserSpaceError("check not successful")); err != nil {
		if !models.IsUserSpaceError(err) {
			t.Fatalf("RenderModel: unexpected error: %v", err)
		}
	}

	output := strings.TrimSpace(buf.String())
	assertTrue(t, strings.HasPrefix(output, "{"), "expected SARIF JSON object, got: %s", firstRunes(output, 40))

	var log models.SARIFLog
	if err := json.Unmarshal([]byte(output), &log); err != nil {
		t.Fatalf("failed to unmarshal SARIF log: %v\noutput: %s", err, output)
	}

	assertTrue(t, log.Version == "2.1.0", "expected SARIF version 2.1.0, got %s", log.Version)
	assertTrue(t, len(log.Runs) == 1, "expected 1 run, got %d", len(log.Runs))

	run := log.Runs[0]
	assertTrue(t, run.Tool.Driver.Name == "go-arch-lint", "unexpected driver name: %s", run.Tool.Driver.Name)
	assertTrue(t, run.Tool.Driver.Version == "v9.9.9", "expected injected driver version v9.9.9, got %s", run.Tool.Driver.Version)
	assertTrue(t, len(run.Results) == 2, "expected 2 results, got %d", len(run.Results))
	assertTrue(t, run.Results[0].RuleID == models.SARIFRuleDependency, "first result must be the dependency rule, got %s", run.Results[0].RuleID)
}

// TestRenderModel_FormatSARIF_EmptyResults verifies the clean-project
// case: a valid SARIF log with zero results (results: [], not null).
func TestRenderModel_FormatSARIF_EmptyResults(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatSARIF)

	if err := r.RenderModel(models.CmdCheckOut{}, nil); err != nil {
		t.Fatalf("RenderModel: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	assertTrue(t, strings.HasPrefix(output, "{"), "expected SARIF JSON object, got: %s", firstRunes(output, 40))

	var log models.SARIFLog
	if err := json.Unmarshal([]byte(output), &log); err != nil {
		t.Fatalf("failed to unmarshal SARIF log: %v\noutput: %s", err, output)
	}

	assertTrue(t, len(log.Runs) == 1, "expected 1 run, got %d", len(log.Runs))
	assertTrue(t, len(log.Runs[0].Results) == 0, "expected 0 results")
	assertTrue(t, strings.Contains(output, "\"results\": []"), "results must serialize as [] not null, got: %s", output)
}

// TestRenderModel_FormatSARIF_DefaultDriverVersion verifies the fallback
// when no build version was injected: "dev", not an empty string.
func TestRenderModel_FormatSARIF_DefaultDriverVersion(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatSARIF)

	if err := r.RenderModel(models.CmdCheckOut{}, nil); err != nil {
		t.Fatalf("RenderModel: %v", err)
	}

	var log models.SARIFLog
	if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
		t.Fatalf("failed to unmarshal SARIF log: %v", err)
	}

	assertTrue(t, log.Runs[0].Tool.Driver.Version == "dev", "expected default driver version 'dev', got %q", log.Runs[0].Tool.Driver.Version)
}

// TestRenderModel_FormatSARIF_NonCheckModelFallsBackToJSON verifies that
// --format sarif on a non-check command degrades safely to the generic
// wrapped JSON instead of crashing (mirrors the --format json fallback).
func TestRenderModel_FormatSARIF_NonCheckModelFallsBackToJSON(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatSARIF)
	r.asciiTemplates = map[string]string{} // irrelevant for the fallback

	model := models.CmdVersionOut{LinterVersion: "1.0.0"}
	if err := r.RenderModel(model, nil); err != nil {
		t.Fatalf("RenderModel: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	assertTrue(t, strings.HasPrefix(output, "{"), "expected wrapped JSON object, got: %s", firstRunes(output, 40))
	assertTrue(t, strings.Contains(output, "\"Type\": \"models.Version\""), "expected wrapped model Type, got: %s", output)
}
