package render

import (
	"errors"
	"strings"
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

// TestRenderModel_ReferableErrorPreview verifies the code-preview path
// writes through the injected writer (r.out) — not os.Stdout — so the
// Renderer stays fully pipeable (PR #13 left this path on fmt.Printf).
func TestRenderModel_ReferableErrorPreview(t *testing.T) {
	var sb strings.Builder

	r := NewRendererTo(
		&sb,
		fakeColorPrinter{},
		fakeReferenceRender{},
		models.OutputTypeASCII,
		false,
		models.FormatText,
		map[string]string{},
	)

	original := errors.New("boom at line 5")
	referable := models.NewReferableErr(original, domain.Reference{})
	err := r.RenderModel(nil, referable)

	if err == nil {
		t.Fatal("RenderModel must return the error")
	}
	if err.Error() != "boom at line 5" {
		t.Fatalf("RenderModel must return the original error, got %q", err.Error())
	}

	out := sb.String()
	for _, want := range []string{"ERR: boom at line 5", "------------", "// preview"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("preview must be newline-terminated, got %q", out)
	}
}
