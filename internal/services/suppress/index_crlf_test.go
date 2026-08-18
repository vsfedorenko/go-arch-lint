package suppress

import (
	"os"
	"path/filepath"
	"testing"
)

// CRLF line endings (Windows checkouts) must not break directive
// extraction: the scanner splits on \n, and a trailing \r would end up
// glued to the argument or the directive itself.
func TestIndexCRLFLineEndings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "win.go")
	body := "package demo\r\n\r\n//go-arch-lint:ignore beta\r\nimport \"example.com/app/internal/beta\"\r\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	// The directive carries "beta\r" as its argument if \r is not
	// trimmed: then neither "beta" nor the import path's last segment
	// ("beta") matches, and the violation fires on a suppressed line.
	if !index.IsLineSuppressed(path, 4, "example.com/app/internal/beta") {
		t.Fatal("CRLF: argument filter must still match the dependency target")
	}
	if !index.IsLineSuppressed(path, 4, "beta") {
		t.Fatal("CRLF: exact argument must match (trailing \\r must be trimmed)")
	}
}

func TestIndexCRTrailingDirective(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "win2.go")
	body := "package demo\r\n\r\nimport \"example.com/app/internal/beta\" //go-arch-lint:ignore\r\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	index, err := NewIndexFromFiles([]string{path})
	if err != nil {
		t.Fatalf("NewIndexFromFiles: %v", err)
	}
	if !index.IsLineSuppressed(path, 3, "example.com/app/internal/beta") {
		t.Fatal("CRLF: trailing directive without argument must suppress its line")
	}
}
