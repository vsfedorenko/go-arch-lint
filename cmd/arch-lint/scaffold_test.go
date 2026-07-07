package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdirTemp creates a temp dir, chdirs into it, and returns a cleanup func.
func chdirTemp(t *testing.T) (dir string, cleanup func()) {
	t.Helper()
	dir = t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	return dir, func() {
		_ = os.Chdir(orig)
	}
}

func TestCmdInit_CreatesScaffold(t *testing.T) {
	_, cleanup := chdirTemp(t)
	defer cleanup()

	code := cmdInit(nil)
	if code != 0 {
		t.Fatalf("cmdInit returned %d, want 0", code)
	}

	archDir := ".go-arch-lint"
	for _, name := range []string{"go.mod", "main.go", "arch.go"} {
		path := filepath.Join(archDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected file %s: %v", path, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("file %s is empty", path)
		}
	}

	// Verify go.mod has the local module declaration (require line is added by 'go mod tidy')
	gomod, err := os.ReadFile(filepath.Join(archDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.Contains(string(gomod), "module arch-lint-local") {
		t.Errorf("go.mod missing module declaration: %s", gomod)
	}

	// Verify main.go imports archlint
	maingo, err := os.ReadFile(filepath.Join(archDir, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(maingo), "archlint.MustRunCLI()") {
		t.Errorf("main.go missing archlint.MustRunCLI(): %s", maingo)
	}

	// Verify arch.go uses DSL Spec
	archgo, err := os.ReadFile(filepath.Join(archDir, "arch.go"))
	if err != nil {
		t.Fatalf("read arch.go: %v", err)
	}
	if !strings.Contains(string(archgo), "Spec(func()") {
		t.Errorf("arch.go missing Spec entry: %s", archgo)
	}
}

func TestCmdInit_AlreadyExists(t *testing.T) {
	_, cleanup := chdirTemp(t)
	defer cleanup()

	// First init succeeds
	if code := cmdInit(nil); code != 0 {
		t.Fatalf("first cmdInit returned %d, want 0", code)
	}

	// Second init must fail with code 1
	code := cmdInit(nil)
	if code != 1 {
		t.Errorf("second cmdInit returned %d, want 1", code)
	}
}
