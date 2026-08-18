package suppress

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The suite drives the exported surface of the directive index: file
// scanning, line/file matching, argument filters, and the parser edge
// cases the e2e suppress tests cannot reach individually.

func writeSource(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "src.go")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIndexNoDirectives(t *testing.T) {
	path := writeSource(t,
		"package demo",
		"",
		"import (",
		"\t\"fmt\"",
		")",
		"",
		"func F() { fmt.Println(\"go-arch-lint:ignore\") }",
	)

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if index.HasDirectives() {
		t.Fatal("plain source must yield no directives")
	}
	if index.IsLineSuppressed(path, 7, "anything") {
		t.Fatal("nothing is suppressed without directives")
	}
	if index.IsFileSuppressed(path) {
		t.Fatal("file is not suppressed without directives")
	}
}

func TestIndexStandaloneDirectiveAboveLine(t *testing.T) {
	// Directive on its own comment line applies to the NEXT code line.
	path := writeSource(t,
		"package demo",
		"",
		"//go-arch-lint:ignore",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if !index.IsLineSuppressed(path, 4, "example.com/app/internal/beta") {
		t.Fatal("standalone directive must suppress the next code line")
	}
	if index.IsLineSuppressed(path, 3, "example.com/app/internal/beta") {
		t.Fatal("the comment line itself is not the target")
	}
	if index.IsLineSuppressed(path, 5, "example.com/app/internal/beta") {
		t.Fatal("lines after the target are not suppressed")
	}
}

func TestIndexTrailingDirectiveSameLine(t *testing.T) {
	// Directive after code applies to its own line.
	path := writeSource(t,
		"package demo",
		"",
		"import \"example.com/app/internal/beta\" //go-arch-lint:ignore",
	)

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if !index.IsLineSuppressed(path, 3, "example.com/app/internal/beta") {
		t.Fatal("trailing directive must suppress its own line")
	}
}

func TestIndexDirectiveWithTargetArgument(t *testing.T) {
	path := writeSource(t,
		"package demo",
		"",
		"//go-arch-lint:ignore beta",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}

	// Full import path matches by last segment.
	if !index.IsLineSuppressed(path, 4, "example.com/app/internal/beta") {
		t.Fatal("argument must match the dependency target by last path segment")
	}
	// Exact argument match.
	if !index.IsLineSuppressed(path, 4, "beta") {
		t.Fatal("argument must match the exact target")
	}
	// Different target is NOT suppressed.
	if index.IsLineSuppressed(path, 4, "example.com/app/internal/gamma") {
		t.Fatal("argument filter must not suppress other targets")
	}
}

func TestIndexDirectiveWithMultipleArguments(t *testing.T) {
	path := writeSource(t,
		"package demo",
		"",
		"//go-arch-lint:ignore beta gamma",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if !index.IsLineSuppressed(path, 4, "gamma") {
		t.Fatal("second argument must suppress its target")
	}
	if index.IsLineSuppressed(path, 4, "delta") {
		t.Fatal("unlisted target must not be suppressed")
	}
}

func TestIndexAnyTargetResetsArgumentFilter(t *testing.T) {
	// A later argument-less directive on the same line resets the filter
	// to "any target" (addLine semantics).
	path := writeSource(t,
		"package demo",
		"",
		"//go-arch-lint:ignore beta",
		"//go-arch-lint:ignore",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if !index.IsLineSuppressed(path, 5, "anything-at-all") {
		t.Fatal("argument-less directive must reset the filter to any-target")
	}
}

func TestIndexIgnoreFile(t *testing.T) {
	path := writeSource(t,
		"package demo",
		"//go-arch-lint:ignore-file",
		"",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if !index.IsFileSuppressed(path) {
		t.Fatal("ignore-file must suppress the whole file")
	}
	if !index.HasDirectives() {
		t.Fatal("index must report directives")
	}
}

func TestIndexIgnoreNeverMatchesIgnoreFile(t *testing.T) {
	// "ignore" must not swallow "ignore-file" by prefix, and vice versa.
	path := writeSource(t,
		"package demo",
		"",
		"//go-arch-lint:ignore-filex",
		"//go-arch-lint:ignorefile",
		"//go-arch-lint:ignore-x",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if index.IsFileSuppressed(path) {
		t.Fatal("misspelled ignore-file variants must not suppress the file")
	}
	if index.IsLineSuppressed(path, 6, "example.com/app/internal/beta") {
		t.Fatal("misspelled ignore variants must not suppress the line")
	}
}

func TestIndexDirectiveInsideStringLiteral(t *testing.T) {
	// A directive-looking TEXT inside a string literal or a doc comment
	// that is not a line-comment directive... The scanner is line-based,
	// so text in strings after "//" IS matched. This test documents the
	// current contract: only lines whose comment part holds the
	// directive match — here the directive text sits before any "//",
	// so nothing matches.
	path := writeSource(t,
		"package demo",
		"",
		"const s = \"go-arch-lint:ignore\"",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if index.HasDirectives() {
		t.Fatal("directive text inside a string literal (no // prefix) must not match")
	}
}

func TestIndexURLCommentFalsePositive(t *testing.T) {
	// KNOWN SHARP EDGE (documented in this test): the line-based scanner
	// takes the text after the FIRST "//" as the comment. A URL in code
	// or in a string literal therefore contains "go-arch-lint:ignore"
	// only if it literally appears after a "//" — e.g. a doc link.
	// A plain https URL does NOT false-positive because the directive
	// prefix must start right after "//".
	path := writeSource(t,
		"package demo",
		"",
		"// See https://example.com/go-arch-lint:ignore for docs",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if index.IsLineSuppressed(path, 4, "example.com/app/internal/beta") {
		t.Fatal("URL text after // must not trigger the directive (prefix must match immediately)")
	}
}

func TestIndexMissingFileSkipped(t *testing.T) {
	index, err := NewIndexFromFiles([]string{filepath.Join(t.TempDir(), "gone.go")})
	if err != nil {
		t.Fatalf("missing file must be skipped, got error: %v", err)
	}
	if index.HasDirectives() {
		t.Fatal("missing file yields no directives")
	}
}

func TestIndexMultipleFilesIndependent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.go")
	b := filepath.Join(dir, "b.go")
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(a, "package demo\n\n//go-arch-lint:ignore\nimport \"example.com/app/internal/beta\"\n")
	write(b, "package demo\n\nimport \"example.com/app/internal/beta\"\n")

	index, err := NewIndexFromFiles([]string{a, b})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if !index.IsLineSuppressed(a, 4, "example.com/app/internal/beta") {
		t.Fatal("file a line 4 must be suppressed")
	}
	if index.IsLineSuppressed(b, 4, "example.com/app/internal/beta") {
		t.Fatal("file b has no directive — nothing suppressed")
	}
}

func TestIndexConsecutiveDirectivesApplyToNextCodeLine(t *testing.T) {
	// Two stacked directives both land on the single following code line.
	path := writeSource(t,
		"package demo",
		"",
		"//go-arch-lint:ignore beta",
		"//go-arch-lint:ignore gamma",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if !index.IsLineSuppressed(path, 5, "beta") {
		t.Fatal("first stacked directive must apply")
	}
	if !index.IsLineSuppressed(path, 5, "gamma") {
		t.Fatal("second stacked directive must apply")
	}
}

func TestIndexDirectiveThenBlankLineDoesNotReachFarCode(t *testing.T) {
	// A directive followed by a blank line: the pending set applies to
	// the NEXT line — the blank one — and is consumed. Real code two
	// lines below is NOT suppressed. This documents current behavior.
	path := writeSource(t,
		"package demo",
		"",
		"//go-arch-lint:ignore",
		"",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if index.IsLineSuppressed(path, 5, "example.com/app/internal/beta") {
		t.Fatal("directive must not cross a blank line (applies to the immediately next line)")
	}
}
