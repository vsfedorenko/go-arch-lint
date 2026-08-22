package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
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
		require.True(t, models.IsUserSpaceError(err), "RenderModel: unexpected error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	assert.True(t, strings.HasPrefix(output, "{"), "expected SARIF JSON object, got: %s", firstRunes(output, 40))

	var log models.SARIFLog
	require.NoError(t, json.Unmarshal([]byte(output), &log), "failed to unmarshal SARIF log\noutput: %s", output)

	assert.Equal(t, "2.1.0", log.Version, "SARIF version")
	require.Len(t, log.Runs, 1, "runs")

	run := log.Runs[0]
	assert.Equal(t, "go-arch-lint", run.Tool.Driver.Name, "driver name")
	assert.Equal(t, "v9.9.9", run.Tool.Driver.Version, "injected driver version")
	require.Len(t, run.Results, 2, "results")
	assert.Equal(t, models.SARIFRuleDependency, run.Results[0].RuleID, "first result must be the dependency rule")
}

// TestRenderModel_FormatSARIF_EmptyResults verifies the clean-project
// case: a valid SARIF log with zero results (results: [], not null).
func TestRenderModel_FormatSARIF_EmptyResults(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatSARIF)

	require.NoError(t, r.RenderModel(models.CmdCheckOut{}, nil))

	output := strings.TrimSpace(buf.String())
	assert.True(t, strings.HasPrefix(output, "{"), "expected SARIF JSON object, got: %s", firstRunes(output, 40))

	var log models.SARIFLog
	require.NoError(t, json.Unmarshal([]byte(output), &log), "failed to unmarshal SARIF log\noutput: %s", output)

	require.Len(t, log.Runs, 1, "runs")
	assert.Empty(t, log.Runs[0].Results, "results")
	assert.Contains(t, output, `"results": []`, "results must serialize as [] not null")
}

// TestRenderModel_FormatSARIF_DefaultDriverVersion verifies the fallback
// when no build version was injected: "dev", not an empty string.
func TestRenderModel_FormatSARIF_DefaultDriverVersion(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatSARIF)

	require.NoError(t, r.RenderModel(models.CmdCheckOut{}, nil))

	var log models.SARIFLog
	require.NoError(t, json.Unmarshal(buf.Bytes(), &log), "failed to unmarshal SARIF log")

	assert.Equal(t, "dev", log.Runs[0].Tool.Driver.Version, "default driver version")
}

// TestRenderModel_FormatSARIF_NonCheckModelFallsBackToJSON verifies that
// --format sarif on a non-check command degrades safely to the generic
// wrapped JSON instead of crashing (mirrors the --format json fallback).
func TestRenderModel_FormatSARIF_NonCheckModelFallsBackToJSON(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatSARIF)
	r.asciiTemplates = map[string]string{} // irrelevant for the fallback

	model := models.CmdVersionOut{LinterVersion: "1.0.0"}
	require.NoError(t, r.RenderModel(model, nil))

	output := strings.TrimSpace(buf.String())
	assert.True(t, strings.HasPrefix(output, "{"), "expected wrapped JSON object, got: %s", firstRunes(output, 40))
	assert.Contains(t, output, `"Type": "models.Version"`, "expected wrapped model Type")
}
