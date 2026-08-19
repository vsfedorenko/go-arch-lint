package suppress

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The suite drives the exported surface of the directive index: file
// scanning, line/file matching, argument filters, and the parser edge
// cases the e2e suppress tests cannot reach individually.

func writeSource(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "src.go")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
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
	require.NoError(t, err, "NewIndexFromFiles")
	assert.False(t, index.HasDirectives(), "plain source must yield no directives")
	assert.False(t, index.IsLineSuppressed(path, 7, "anything"), "nothing is suppressed without directives")
	assert.False(t, index.IsFileSuppressed(path), "file is not suppressed without directives")
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
	require.NoError(t, err, "NewIndexFromFiles")
	assert.True(t, index.IsLineSuppressed(path, 4, "example.com/app/internal/beta"),
		"standalone directive must suppress the next code line")
	assert.False(t, index.IsLineSuppressed(path, 3, "example.com/app/internal/beta"),
		"the comment line itself is not the target")
	assert.False(t, index.IsLineSuppressed(path, 5, "example.com/app/internal/beta"),
		"lines after the target are not suppressed")
}

func TestIndexTrailingDirectiveSameLine(t *testing.T) {
	// Directive after code applies to its own line.
	path := writeSource(t,
		"package demo",
		"",
		"import \"example.com/app/internal/beta\" //go-arch-lint:ignore",
	)

	index, err := NewIndexFromFiles([]string{path})
	require.NoError(t, err, "NewIndexFromFiles")
	assert.True(t, index.IsLineSuppressed(path, 3, "example.com/app/internal/beta"),
		"trailing directive must suppress its own line")
}

func TestIndexDirectiveWithTargetArgument(t *testing.T) {
	path := writeSource(t,
		"package demo",
		"",
		"//go-arch-lint:ignore beta",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	require.NoError(t, err, "NewIndexFromFiles")

	// Full import path matches by last segment.
	assert.True(t, index.IsLineSuppressed(path, 4, "example.com/app/internal/beta"),
		"argument must match the dependency target by last path segment")
	// Exact argument match.
	assert.True(t, index.IsLineSuppressed(path, 4, "beta"),
		"argument must match the exact target")
	// Different target is NOT suppressed.
	assert.False(t, index.IsLineSuppressed(path, 4, "example.com/app/internal/gamma"),
		"argument filter must not suppress other targets")
}

func TestIndexDirectiveWithMultipleArguments(t *testing.T) {
	path := writeSource(t,
		"package demo",
		"",
		"//go-arch-lint:ignore beta gamma",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	require.NoError(t, err, "NewIndexFromFiles")
	assert.True(t, index.IsLineSuppressed(path, 4, "gamma"),
		"second argument must suppress its target")
	assert.False(t, index.IsLineSuppressed(path, 4, "delta"),
		"unlisted target must not be suppressed")
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
	require.NoError(t, err, "NewIndexFromFiles")
	assert.True(t, index.IsLineSuppressed(path, 5, "anything-at-all"),
		"argument-less directive must reset the filter to any-target")
}

func TestIndexIgnoreFile(t *testing.T) {
	path := writeSource(t,
		"package demo",
		"//go-arch-lint:ignore-file",
		"",
		"import \"example.com/app/internal/beta\"",
	)

	index, err := NewIndexFromFiles([]string{path})
	require.NoError(t, err, "NewIndexFromFiles")
	assert.True(t, index.IsFileSuppressed(path), "ignore-file must suppress the whole file")
	assert.True(t, index.HasDirectives(), "index must report directives")
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
	require.NoError(t, err, "NewIndexFromFiles")
	assert.False(t, index.IsFileSuppressed(path),
		"misspelled ignore-file variants must not suppress the file")
	assert.False(t, index.IsLineSuppressed(path, 6, "example.com/app/internal/beta"),
		"misspelled ignore variants must not suppress the line")
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
	require.NoError(t, err, "NewIndexFromFiles")
	assert.False(t, index.HasDirectives(),
		"directive text inside a string literal (no // prefix) must not match")
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
	require.NoError(t, err, "NewIndexFromFiles")
	assert.False(t, index.IsLineSuppressed(path, 4, "example.com/app/internal/beta"),
		"URL text after // must not trigger the directive (prefix must match immediately)")
}

func TestIndexMissingFileSkipped(t *testing.T) {
	index, err := NewIndexFromFiles([]string{filepath.Join(t.TempDir(), "gone.go")})
	require.NoError(t, err, "missing file must be skipped")
	assert.False(t, index.HasDirectives(), "missing file yields no directives")
}

func TestIndexMultipleFilesIndependent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.go")
	b := filepath.Join(dir, "b.go")
	write := func(path, body string) {
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	}
	write(a, "package demo\n\n//go-arch-lint:ignore\nimport \"example.com/app/internal/beta\"\n")
	write(b, "package demo\n\nimport \"example.com/app/internal/beta\"\n")

	index, err := NewIndexFromFiles([]string{a, b})
	require.NoError(t, err, "NewIndexFromFiles")
	assert.True(t, index.IsLineSuppressed(a, 4, "example.com/app/internal/beta"),
		"file a line 4 must be suppressed")
	assert.False(t, index.IsLineSuppressed(b, 4, "example.com/app/internal/beta"),
		"file b has no directive — nothing suppressed")
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
	require.NoError(t, err, "NewIndexFromFiles")
	assert.True(t, index.IsLineSuppressed(path, 5, "beta"), "first stacked directive must apply")
	assert.True(t, index.IsLineSuppressed(path, 5, "gamma"), "second stacked directive must apply")
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
	require.NoError(t, err, "NewIndexFromFiles")
	assert.False(t, index.IsLineSuppressed(path, 5, "example.com/app/internal/beta"),
		"directive must not cross a blank line (applies to the immediately next line)")
}
