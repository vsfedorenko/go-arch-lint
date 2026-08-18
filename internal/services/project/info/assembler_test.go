package info

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ProjectInfo assembles the project descriptor: absolute root, arch-file
// path, go.mod presence, and module name extraction. These tests drive it
// against a real temp-dir project layout.

func writeProject(t *testing.T, goModBody string) string {
	t.Helper()
	root := t.TempDir()
	if goModBody != "" {
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goModBody), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestProjectInfo_HappyPath(t *testing.T) {
	root := writeProject(t, "module example.com/myapp\n\ngo 1.25\n")

	info, err := NewAssembler().ProjectInfo(root, ".go-arch-lint/arch.go")
	if err != nil {
		t.Fatalf("ProjectInfo: %v", err)
	}
	abs, _ := filepath.Abs(root)
	if info.Directory != abs {
		t.Fatalf("directory = %q, want %q", info.Directory, abs)
	}
	if info.ModuleName != "example.com/myapp" {
		t.Fatalf("module = %q", info.ModuleName)
	}
	if info.GoModFilePath != filepath.Join(abs, "go.mod") {
		t.Fatalf("go.mod path = %q", info.GoModFilePath)
	}
	if !strings.HasSuffix(info.GoArchFilePath, ".go-arch-lint/arch.go") {
		t.Fatalf("arch path = %q", info.GoArchFilePath)
	}
}

func TestProjectInfo_RelativeRoot(t *testing.T) {
	// Relative roots (the CLI default "./") resolve against CWD.
	root := writeProject(t, "module m\n")
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	info, err := NewAssembler().ProjectInfo(".", ".go-arch-lint/arch.go")
	if err != nil {
		t.Fatalf("ProjectInfo: %v", err)
	}
	if !filepath.IsAbs(info.Directory) {
		t.Fatalf("directory must be absolute, got %q", info.Directory)
	}
}

func TestProjectInfo_MissingGoMod(t *testing.T) {
	root := writeProject(t, "")

	_, err := NewAssembler().ProjectInfo(root, ".go-arch-lint/arch.go")
	if err == nil {
		t.Fatal("missing go.mod must fail")
	}
	if !strings.Contains(err.Error(), "not found project 'go.mod'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectInfo_BrokenGoMod(t *testing.T) {
	root := writeProject(t, "this is not a go.mod file !!!\n")

	// modfile.ParseLax is tolerant of unknown lines, but broken syntax
	// must surface as a module-name error, not a crash.
	_, err := NewAssembler().ProjectInfo(root, ".go-arch-lint/arch.go")
	if err == nil && !goModIsParseable(root) {
		t.Fatalf("expected an error or a parseable go.mod, got err=%v", err)
	}
}

func goModIsParseable(root string) bool {
	body, err := os.ReadFile(filepath.Join(root, "go.mod"))
	return err == nil && strings.Contains(string(body), "module")
}

func TestProjectInfo_GoModWithoutModule(t *testing.T) {
	root := writeProject(t, "go 1.25\n")

	_, err := NewAssembler().ProjectInfo(root, ".go-arch-lint/arch.go")
	if err == nil {
		t.Fatal("go.mod without a module directive must fail module-name extraction")
	}
	if !strings.Contains(err.Error(), "failed get module name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectInfo_ArchFileURLRejected(t *testing.T) {
	root := writeProject(t, "module m\n")

	_, err := NewAssembler().ProjectInfo(root, "https://example.com/arch.go")
	if err == nil {
		t.Fatal("URL arch files must be rejected in v2 Go DSL mode")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectInfo_AbsoluteArchFilePath(t *testing.T) {
	root := writeProject(t, "module m\n")
	absArch := filepath.Join(root, "elsewhere.go")

	info, err := NewAssembler().ProjectInfo(root, absArch)
	if err != nil {
		t.Fatalf("ProjectInfo: %v", err)
	}
	if info.GoArchFilePath != absArch {
		t.Fatalf("absolute arch path must pass through, got %q", info.GoArchFilePath)
	}
}
