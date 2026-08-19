package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

// fakeReferenceRender is a minimal referenceRender for tests.
type fakeReferenceRender struct{}

func (f fakeReferenceRender) SourceCode(_ domain.Reference, _, _ bool) []byte {
	return []byte("// preview\n")
}

// newTestRenderer builds a Renderer writing into a caller-owned buffer —
// no process-wide os.Stdout swapping needed.
func newTestRenderer(t *testing.T, format models.Format) (*Renderer, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	r := NewRendererTo(
		buf,
		fakeColorPrinter{},
		fakeReferenceRender{},
		models.OutputTypeASCII,
		false,
		format,
		map[string]string{},
	)
	return r, buf
}

// fakeColorPrinter satisfies colorPrinter without real ANSI codes.
type fakeColorPrinter struct{}

func (fakeColorPrinter) Red(s string) string     { return s }
func (fakeColorPrinter) Green(s string) string   { return s }
func (fakeColorPrinter) Yellow(s string) string  { return s }
func (fakeColorPrinter) Blue(s string) string    { return s }
func (fakeColorPrinter) Magenta(s string) string { return s }
func (fakeColorPrinter) Cyan(s string) string    { return s }
func (fakeColorPrinter) Gray(s string) string    { return s }

func TestRenderModel_FormatJSON_CheckOut(t *testing.T) {
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

	r, buf := newTestRenderer(t, models.FormatJSON)

	// UserSpaceError is expected: it means "violations found". RenderModel
	// renders the model AND returns the error for exit-code mapping.
	if err := r.RenderModel(out, models.NewUserSpaceError("check not successful")); err != nil {
		require.True(t, models.IsUserSpaceError(err), "RenderModel: unexpected error: %v", err)
	}

	// Output should be a JSON array (not the wrapped {Type, Payload} object)
	output := strings.TrimSpace(buf.String())
	assert.True(t, strings.HasPrefix(output, "["), "expected JSON array, got: %s", firstRunes(output, 40))

	var violations []models.Violation
	require.NoError(t, json.Unmarshal([]byte(output), &violations), "failed to unmarshal JSON array\noutput: %s", output)

	assert.Len(t, violations, 2, "violations")
	assert.Equal(t, "dependency", violations[0].Type)
	assert.Equal(t, "match", violations[1].Type)
}

func TestRenderModel_FormatJSON_EmptyViolations(t *testing.T) {
	// When there are no violations, the JSON array should be "[]" (not null).
	r, buf := newTestRenderer(t, models.FormatJSON)

	require.NoError(t, r.RenderModel(models.CmdCheckOut{}, nil))

	assert.Equal(t, "[]", strings.TrimSpace(buf.String()))
}

func TestRenderModel_FormatText_FallsBackToASCII(t *testing.T) {
	// With format=text, the renderer should use the ASCII path. Since our test
	// renderer has no templates, we expect an error about the missing template
	// — proving it did NOT take the format=json fast path.
	r, _ := newTestRenderer(t, models.FormatText)

	err := r.RenderModel(models.CmdCheckOut{}, nil)
	require.Error(t, err, "expected error from missing ASCII template")
	assert.Contains(t, err.Error(), "not exist")
}

func TestNewRenderer_DefaultsToStdout(t *testing.T) {
	r := NewRenderer(
		fakeColorPrinter{},
		fakeReferenceRender{},
		models.OutputTypeJSON,
		true,
		models.FormatJSON,
		map[string]string{},
	)
	assert.Equal(t, models.FormatJSON, r.format)
	assert.Equal(t, models.OutputTypeJSON, r.outputType)
	assert.True(t, r.outputJSONOneLine, "expected one-line JSON to be true")
	assert.NotNil(t, r.out, "expected default renderer to write to os.Stdout (non-nil out)")
}

// --- minimal assertion helpers (avoid extra deps churn) ---

func firstRunes(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}
