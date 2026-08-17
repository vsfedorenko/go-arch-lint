package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
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
		ModuleName: "github.com/x/proj",
		ArchWarningsDependency: []models.CheckArchWarningDependency{
			{
				ComponentName:      "handler",
				FileRelativePath:   "internal/handler/user.go",
				ResolvedImportName: "github.com/x/proj/internal/repository",
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
		if !models.IsUserSpaceError(err) {
			t.Fatalf("RenderModel: unexpected error: %v", err)
		}
	}

	// Output should be a JSON array (not the wrapped {Type, Payload} object)
	output := strings.TrimSpace(buf.String())
	assertTrue(t, strings.HasPrefix(output, "["), "expected JSON array, got: %s", firstRunes(output, 40))

	var violations []models.Violation
	if err := json.Unmarshal([]byte(output), &violations); err != nil {
		t.Fatalf("failed to unmarshal JSON array: %v\noutput: %s", err, output)
	}

	assertTrue(t, len(violations) == 2, "expected 2 violations, got %d", len(violations))
	assertEquals(t, "dependency", violations[0].Type)
	assertEquals(t, "match", violations[1].Type)
}

func TestRenderModel_FormatJSON_EmptyViolations(t *testing.T) {
	// When there are no violations, the JSON array should be "[]" (not null).
	r, buf := newTestRenderer(t, models.FormatJSON)

	if err := r.RenderModel(models.CmdCheckOut{}, nil); err != nil {
		t.Fatalf("RenderModel: %v", err)
	}

	assertEquals(t, "[]", strings.TrimSpace(buf.String()))
}

func TestRenderModel_FormatText_FallsBackToASCII(t *testing.T) {
	// With format=text, the renderer should use the ASCII path. Since our test
	// renderer has no templates, we expect an error about the missing template
	// — proving it did NOT take the format=json fast path.
	r, _ := newTestRenderer(t, models.FormatText)

	err := r.RenderModel(models.CmdCheckOut{}, nil)
	assertTrue(t, err != nil, "expected error from missing ASCII template")
	assertContains(t, err.Error(), "not exist")
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
	assertEquals(t, models.FormatJSON, r.format)
	assertEquals(t, models.OutputTypeJSON, r.outputType)
	assertTrue(t, r.outputJSONOneLine, "expected one-line JSON to be true")
	if r.out == nil {
		t.Error("expected default renderer to write to os.Stdout (non-nil out)")
	}
}

// --- minimal assertion helpers (avoid extra deps churn) ---

func assertTrue(t *testing.T, cond bool, format string, args ...any) {
	t.Helper()
	if !cond {
		t.Errorf(format, args...)
	}
}

func assertEquals[T comparable](t *testing.T, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("want %v, got %v", want, got)
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected %q to contain %q", s, substr)
	}
}

func firstRunes(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}
