package render

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

// Fixture literals reused across the renderer JUnit tests (goconst-clean).
const (
	junitRendererModule     = "github.com/x/proj"
	junitRendererComponent  = "handler"
	junitRendererDepImport  = junitRendererModule + "/internal/repository"
	junitRendererDepFile    = "internal/handler/user.go"
	junitRendererOrphanFile = "internal/orphan/x.go"
)

// TestRenderModel_FormatJUnit_CheckOut verifies the --format junit fast
// path: an XML document (not the wrapped {Type, Payload} model) with the
// xml header, one failed testcase per violation.
func TestRenderModel_FormatJUnit_CheckOut(t *testing.T) {
	out := models.CmdCheckOut{
		ModuleName: junitRendererModule,
		ArchWarningsDependency: []models.CheckArchWarningDependency{
			{
				ComponentName:      junitRendererComponent,
				FileRelativePath:   junitRendererDepFile,
				ResolvedImportName: junitRendererDepImport,
				Reference:          domain.NewReferenceSingleLine("internal/handler/user.go", 10, 2),
			},
		},
		ArchWarningsMatch: []models.CheckArchWarningMatch{
			{FileRelativePath: junitRendererOrphanFile},
		},
	}

	r, buf := newTestRenderer(t, models.FormatJUnit)

	// UserSpaceError is expected: it means "violations found". RenderModel
	// renders the model AND returns the error for exit-code mapping.
	if err := r.RenderModel(out, models.NewUserSpaceError("check not successful")); err != nil {
		require.True(t, models.IsUserSpaceError(err), "RenderModel: unexpected error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	assert.True(t, strings.HasPrefix(output, xml.Header), "expected XML header, got: %s", firstRunes(output, 80))

	var report models.JUnitXML
	require.NoError(t, xml.Unmarshal(buf.Bytes(), &report), "failed to unmarshal JUnit report\noutput: %s", output)

	require.Len(t, report.Suites, 1, "suites")
	assert.Equal(t, 2, report.Failures, "failures")
	require.Len(t, report.Suites[0].Cases, 2, "testcases")
	assert.NotNil(t, report.Suites[0].Cases[0].Failure, "first testcase must carry a failure")
}

// TestRenderModel_FormatJUnit_EmptyResults verifies the clean-project
// case: a valid JUnit report with zero failures and one green testcase.
func TestRenderModel_FormatJUnit_EmptyResults(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatJUnit)

	require.NoError(t, r.RenderModel(models.CmdCheckOut{}, nil))

	var report models.JUnitXML
	require.NoError(t, xml.Unmarshal(buf.Bytes(), &report), "failed to unmarshal JUnit report\noutput: %s", buf.String())

	require.Len(t, report.Suites, 1, "suites")
	assert.Equal(t, 0, report.Failures, "failures")
	require.Len(t, report.Suites[0].Cases, 1, "clean project must emit the green arch-check testcase")
	assert.Nil(t, report.Suites[0].Cases[0].Failure, "green testcase must not carry a failure")
}

// TestRenderModel_FormatJUnit_NonCheckModelFallsBackToJSON verifies that
// --format junit on a non-check command degrades safely to the generic
// wrapped JSON instead of crashing (mirrors the --format json fallback).
func TestRenderModel_FormatJUnit_NonCheckModelFallsBackToJSON(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatJUnit)
	r.asciiTemplates = map[string]string{} // irrelevant for the fallback

	model := models.CmdVersionOut{LinterVersion: "1.0.0"}
	require.NoError(t, r.RenderModel(model, nil))

	output := strings.TrimSpace(buf.String())
	assert.True(t, strings.HasPrefix(output, "{"), "expected wrapped JSON object, got: %s", firstRunes(output, 40))
	assert.Contains(t, output, `"Type": "models.Version"`, "expected wrapped model Type")
}
