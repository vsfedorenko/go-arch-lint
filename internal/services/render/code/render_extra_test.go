package code

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

type stubPrinter struct{}

func (stubPrinter) Gray(in string) string { return in }

// writeFile creates a temp file with the given content (lines joined with \n
// plus a trailing newline) and returns its path.
func writeTempFile(t *testing.T, lines ...string) string {
	t.Helper()
	path := t.TempDir() + "/snippet.go"
	content := strings.Join(lines, "\n") + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestSourceCode_InvalidReferenceIsEmpty(t *testing.T) {
	r := NewRender(stubPrinter{})
	out := r.SourceCode(domain.Reference{}, false, false)
	assert.Empty(t, out, "invalid reference must render nothing")
}

func TestSourceCode_MissingFileIsEmpty(t *testing.T) {
	r := NewRender(stubPrinter{})
	ref := domain.NewReferenceRange("/definitely/not/a/file.go", 1, 1, 1)
	out := r.SourceCode(ref, false, false)
	assert.Empty(t, out, "missing file must render nothing")
}

func TestSourceCode_RendersRangeWithLineNumbers(t *testing.T) {
	path := writeTempFile(t, "alpha", "beta", "gamma", "delta")
	r := NewRender(stubPrinter{})

	// lines 2..3, pointer on line 2
	ref := domain.NewReferenceRange(path, 2, 2, 3)
	out := string(r.SourceCode(ref, false, false))

	assert.Contains(t, out, "beta", "range lines missing in output")
	assert.Contains(t, out, "gamma", "range lines missing in output")
	// "delta" must NOT be present (line 4 is outside the range)
	assert.NotContains(t, out, "delta", "line outside range leaked into output")
	assert.Contains(t, out, "2 |", "line numbers missing")
	assert.Contains(t, out, "3 |", "line numbers missing")
	assert.Contains(t, out, "> ", "pointer marker missing for the violation line")
}

func TestSourceCode_PointerOnlyOnViolationLine(t *testing.T) {
	path := writeTempFile(t, "one", "two", "three")
	r := NewRender(stubPrinter{})
	ref := domain.NewReferenceRange(path, 3, 2, 3)

	out := string(r.SourceCode(ref, false, false))
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	markers := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "> ") {
			markers++
		}
	}
	assert.Equal(t, 1, markers, "exactly one pointer marker expected in ")
}

func TestSourceCode_ColumnPointer(t *testing.T) {
	path := writeTempFile(t, "package main")
	r := NewRender(stubPrinter{})
	ref := domain.NewReferenceRange(path, 1, 1, 1)

	out := string(r.SourceCode(ref, false, true))
	assert.Contains(t, out, "^", "column caret missing")
}

func TestSourceCode_TabsReplacedWithSpaces(t *testing.T) {
	path := writeTempFile(t, "\tindented()")
	r := NewRender(stubPrinter{})
	ref := domain.NewReferenceRange(path, 1, 1, 1)

	out := string(r.SourceCode(ref, false, false))
	assert.NotContains(t, out, "\t|", "raw tab leaked into rendered output")
	assert.NotContains(t, out, "|\t", "raw tab leaked into rendered output")
	assert.Contains(t, out, "indented()", "content missing")
}

func TestSourceCode_ReferenceClampedToRealLines(t *testing.T) {
	// The file has 2 lines; the reference asks for lines 5..9. Clamping
	// must not panic and must render the available tail.
	path := writeTempFile(t, "first", "second")
	r := NewRender(stubPrinter{})
	ref := domain.NewReferenceRange(path, 5, 5, 9)

	out := string(r.SourceCode(ref, false, false))
	assert.NotContains(t, out, "first", "clamped render must not include earlier lines")
}

func TestSourceCode_HighlightPath(t *testing.T) {
	// The chroma highlighter runs on real Go files; smoke-test that a .go
	// path with highlight=true still renders the content.
	path := writeTempFile(t, "package main", "func main() {}")
	r := NewRender(stubPrinter{})
	ref := domain.NewReferenceRange(path, 1, 1, 2)

	out := string(r.SourceCode(ref, true, false))
	// chroma interleaves ANSI escapes inside the tokens themselves — strip
	// them before asserting on the visible text.
	plain := ansiRe.ReplaceAllString(out, "")
	assert.Contains(t, plain, "func main() {}", "highlighted content missing")
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")
