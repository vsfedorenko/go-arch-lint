package code

import (
	"os"
	"regexp"
	"strings"
	"testing"

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
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSourceCode_InvalidReferenceIsEmpty(t *testing.T) {
	r := NewRender(stubPrinter{})
	out := r.SourceCode(domain.Reference{}, false, false)
	if len(out) != 0 {
		t.Fatalf("invalid reference must render nothing, got %q", out)
	}
}

func TestSourceCode_MissingFileIsEmpty(t *testing.T) {
	r := NewRender(stubPrinter{})
	ref := domain.NewReferenceRange("/definitely/not/a/file.go", 1, 1, 1)
	out := r.SourceCode(ref, false, false)
	if len(out) != 0 {
		t.Fatalf("missing file must render nothing, got %q", out)
	}
}

func TestSourceCode_RendersRangeWithLineNumbers(t *testing.T) {
	path := writeTempFile(t, "alpha", "beta", "gamma", "delta")
	r := NewRender(stubPrinter{})

	// lines 2..3, pointer on line 2
	ref := domain.NewReferenceRange(path, 2, 2, 3)
	out := string(r.SourceCode(ref, false, false))

	if !strings.Contains(out, "beta") || !strings.Contains(out, "gamma") {
		t.Fatalf("range lines missing in output: %q", out)
	}
	if !strings.Contains(out, "delta") == false {
		// "delta" must NOT be present (line 4 is outside the range)
		if strings.Contains(out, "delta") {
			t.Fatalf("line outside range leaked into output: %q", out)
		}
	}
	if !strings.Contains(out, "2 |") || !strings.Contains(out, "3 |") {
		t.Fatalf("line numbers missing: %q", out)
	}
	if !strings.Contains(out, "> ") {
		t.Fatalf("pointer marker missing for the violation line: %q", out)
	}
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
	if markers != 1 {
		t.Fatalf("exactly one pointer marker expected, got %d in %q", markers, out)
	}
}

func TestSourceCode_ColumnPointer(t *testing.T) {
	path := writeTempFile(t, "package main")
	r := NewRender(stubPrinter{})
	ref := domain.NewReferenceRange(path, 1, 1, 1)

	out := string(r.SourceCode(ref, false, true))
	if !strings.Contains(out, "^") {
		t.Fatalf("column caret missing: %q", out)
	}
}

func TestSourceCode_TabsReplacedWithSpaces(t *testing.T) {
	path := writeTempFile(t, "\tindented()")
	r := NewRender(stubPrinter{})
	ref := domain.NewReferenceRange(path, 1, 1, 1)

	out := string(r.SourceCode(ref, false, false))
	if strings.Contains(out, "\t|") || strings.Contains(out, "|\t") {
		t.Fatalf("raw tab leaked into rendered output: %q", out)
	}
	if !strings.Contains(out, "indented()") {
		t.Fatalf("content missing: %q", out)
	}
}

func TestSourceCode_ReferenceClampedToRealLines(t *testing.T) {
	// The file has 2 lines; the reference asks for lines 5..9. Clamping
	// must not panic and must render the available tail.
	path := writeTempFile(t, "first", "second")
	r := NewRender(stubPrinter{})
	ref := domain.NewReferenceRange(path, 5, 5, 9)

	out := string(r.SourceCode(ref, false, false))
	if strings.Contains(out, "first") {
		t.Fatalf("clamped render must not include earlier lines: %q", out)
	}
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
	if !strings.Contains(plain, "func main() {}") {
		t.Fatalf("highlighted content missing: %q", plain)
	}
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")
