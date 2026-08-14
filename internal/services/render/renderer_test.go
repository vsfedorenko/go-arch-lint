package render

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/common"
)

// fakeReferenceRender is a minimal referenceRender for tests.
type fakeReferenceRender struct{}

func (f fakeReferenceRender) SourceCode(_ common.Reference, _, _ bool) []byte {
	return []byte("// preview\n")
}

func newTestRenderer(format models.Format) *Renderer {
	return NewRenderer(
		fakeColorPrinter{},
		fakeReferenceRender{},
		models.OutputTypeASCII,
		false,
		format,
		map[string]string{},
	)
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

func captureStdout(fn func()) string {
	// Redirect fmt.Println output by swapping os.Stdout.
	// Since the renderer uses fmt.Println directly, we capture via a pipe.
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = orig
	return <-done
}

func TestRenderModel_FormatJSON_CheckOut(t *testing.T) {
	out := models.CmdCheckOut{
		ModuleName: "github.com/x/proj",
		ArchWarningsDependency: []models.CheckArchWarningDependency{
			{
				ComponentName:      "handler",
				FileRelativePath:   "internal/handler/user.go",
				ResolvedImportName: "github.com/x/proj/internal/repository",
				Reference:          common.NewReferenceSingleLine("internal/handler/user.go", 10, 2),
			},
		},
		ArchWarningsMatch: []models.CheckArchWarningMatch{
			{FileRelativePath: "internal/orphan/x.go"},
		},
	}

	r := newTestRenderer(models.FormatJSON)

	output := captureStdout(func() {
		_ = r.RenderModel(out, models.NewUserSpaceError("check not successful"))
	})

	// Output should be a JSON array (not the wrapped {Type, Payload} object)
	output = strings.TrimSpace(output)
	assertTrue(t, strings.HasPrefix(output, "["), "expected JSON array, got: %s", output[:minInt(len(output), 40)])

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
	r := newTestRenderer(models.FormatJSON)

	output := captureStdout(func() {
		_ = r.RenderModel(models.CmdCheckOut{}, nil)
	})

	output = strings.TrimSpace(output)
	assertEquals(t, "[]", output)
}

func TestRenderModel_FormatText_FallsBackToASCII(t *testing.T) {
	// With format=text, the renderer should use the ASCII path. Since our test
	// renderer has no templates, we expect an error about the missing template
	// — proving it did NOT take the format=json fast path.
	r := newTestRenderer(models.FormatText)

	err := r.RenderModel(models.CmdCheckOut{}, nil)
	assertTrue(t, err != nil, "expected error from missing ASCII template")
	assertContains(t, err.Error(), "not exist")
}

func TestNewRenderer_SetsFormat(t *testing.T) {
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
