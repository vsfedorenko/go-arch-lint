package render

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	require.Error(t, err, "RenderModel must return the error")
	assert.Equal(t, "boom at line 5", err.Error(), "RenderModel must return the original error")

	out := sb.String()
	for _, want := range []string{"ERR: boom at line 5", "------------", "// preview"} {
		assert.Contains(t, out, want, "output")
	}
	assert.True(t, strings.HasSuffix(out, "\n"), "preview must be newline-terminated, got ")
}
