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
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("chdir back %s: %v", orig, err)
		}
	}
}

func TestCmdInit_CreatesScaffold(t *testing.T) {
	_, cleanup := chdirTemp(t)
	defer cleanup()

	if code := cmdInit(nil); code != 0 {
		t.Fatalf("cmdInit returned %d, want 0", code)
	}

	archDir := ".go-arch-lint"
	for _, name := range []string{"go.mod", "arch.go", "main.go"} {
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

	// arch.go is the user-editable spec: DSL import, Spec entry, NO runner.
	archgo, err := os.ReadFile(filepath.Join(archDir, "arch.go"))
	if err != nil {
		t.Fatalf("read arch.go: %v", err)
	}
	if !strings.Contains(string(archgo), "Spec(func()") {
		t.Errorf("arch.go missing Spec entry: %s", archgo)
	}
	if !strings.Contains(string(archgo), `"github.com/vsfedorenko/go-arch-lint/dsl"`) {
		t.Errorf("arch.go missing dsl import: %s", archgo)
	}
	if strings.Contains(string(archgo), "func main()") {
		t.Errorf("arch.go must not contain the runner: %s", archgo)
	}

	// main.go is the stable runner: flag passthrough, NO spec definition.
	maingo, err := os.ReadFile(filepath.Join(archDir, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(maingo), "archlint.MustRun(spec, archlint.OptionsFromFlags(os.Args[1:])...)") {
		t.Errorf("main.go missing archlint.MustRun with flag passthrough: %s", maingo)
	}
	if strings.Contains(string(maingo), "Spec(func()") {
		t.Errorf("main.go must not contain the spec: %s", maingo)
	}
}

// The scaffolded ExcludeFiles regex must escape the dot with ONE backslash
// (`\.`), so it actually matches *_test.go files. A historical double escape
// (`\\.`) required a literal backslash in file names and never matched.
func TestScaffold_ExcludeFilesRegexEscapesDotOnce(t *testing.T) {
	_, cleanup := chdirTemp(t)
	defer cleanup()

	if code := cmdInit(nil); code != 0 {
		t.Fatalf("cmdInit returned %d, want 0", code)
	}

	archgo, err := os.ReadFile(filepath.Join(".go-arch-lint", "arch.go"))
	if err != nil {
		t.Fatalf("read arch.go: %v", err)
	}
	if strings.Contains(string(archgo), `\\.`) {
		t.Errorf("arch.go ExcludeFiles has a double backslash (matches no files):\n%s", archgo)
	}
	if !strings.Contains(string(archgo), "^.*_test\\.go$") {
		t.Errorf("arch.go missing ExcludeFiles pattern ^.*_test\\.go$:\n%s", archgo)
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

// The split exists so the runner can be regenerated without touching the
// user's spec: overwriting main.go with a fresh scaffold copy must keep
// arch.go byte-identical.
func TestScaffoldSplit_RegenerateRunnerKeepsSpec(t *testing.T) {
	_, cleanup := chdirTemp(t)
	defer cleanup()

	if code := cmdInit(nil); code != 0 {
		t.Fatalf("cmdInit returned %d, want 0", code)
	}

	// Simulate the user editing their spec.
	archPath := filepath.Join(".go-arch-lint", "arch.go")
	edited := "package main\n\nimport (\n\t. \"github.com/vsfedorenko/go-arch-lint/dsl\"\n)\n\nvar spec = Spec(func() {\n\tVersion(1)\n\tComponent(\"mine\", \"internal/mine/*\")\n})\n"
	if err := os.WriteFile(archPath, []byte(edited), 0o600); err != nil { //nolint:gosec // test fixture in t.TempDir()
		t.Fatalf("write edited arch.go: %v", err)
	}

	// Regenerate ONLY the runner, as an upgrade would.
	mainPath := filepath.Join(".go-arch-lint", "main.go")
	if err := os.WriteFile(mainPath, []byte(scaffoldMainGo), 0o600); err != nil { //nolint:gosec // test fixture in t.TempDir()
		t.Fatalf("rewrite main.go: %v", err)
	}

	got, err := os.ReadFile(archPath)
	if err != nil {
		t.Fatalf("read arch.go: %v", err)
	}
	if string(got) != edited {
		t.Errorf("arch.go changed after runner regeneration:\n%s", got)
	}
}

// Recipe names reused across table tests (goconst); must match the
// launcher's registry.
const (
	tcRecipeClean     = "clean"
	tcRecipeDDD       = "ddd"
	tcRecipeHexagonal = "hexagonal"
	tcTmpPath         = "/tmp/x"
)

// Every recipe must scaffold into a compiling-eligible arch.go: DSL import,
// Spec entry, no runner, single-backslash regex, and a comment marking the
// recipe so users can trace where the file came from.
func TestCmdInit_Recipes(t *testing.T) {
	for _, recipe := range []string{tcRecipeClean, tcRecipeHexagonal, tcRecipeDDD} {
		t.Run(recipe, func(t *testing.T) {
			_, cleanup := chdirTemp(t)
			defer cleanup()

			code := cmdInit([]string{recipeFlag, recipe})
			if code != 0 {
				t.Fatalf("cmdInit --recipe %s returned %d, want 0", recipe, code)
			}

			archgo, err := os.ReadFile(filepath.Join(".go-arch-lint", "arch.go"))
			if err != nil {
				t.Fatalf("read arch.go: %v", err)
			}
			s := string(archgo)
			if !strings.Contains(s, "Spec(func()") {
				t.Errorf("%s: arch.go missing Spec entry", recipe)
			}
			if !strings.Contains(s, `. "github.com/vsfedorenko/go-arch-lint/dsl"`) {
				t.Errorf("%s: arch.go missing dsl import", recipe)
			}
			if strings.Contains(s, "func main()") {
				t.Errorf("%s: arch.go must not contain the runner", recipe)
			}
			if strings.Contains(s, `\\.`) {
				t.Errorf("%s: ExcludeFiles double-backslash bug present", recipe)
			}
			// Recipes tolerate not-yet-created directories.
			if !strings.Contains(s, "IgnoreNotFoundComponents(true)") {
				t.Errorf("%s: recipe must set IgnoreNotFoundComponents(true)", recipe)
			}

			// go.mod and main.go are shared with the plain scaffold.
			gomod, err := os.ReadFile(filepath.Join(".go-arch-lint", "go.mod"))
			if err != nil || !strings.Contains(string(gomod), "module arch-lint-local") {
				t.Errorf("%s: bad go.mod: %v", recipe, err)
			}
			maingo, err := os.ReadFile(filepath.Join(".go-arch-lint", "main.go"))
			if err != nil || !strings.Contains(string(maingo), "archlint.MustRun") {
				t.Errorf("%s: bad main.go: %v", recipe, err)
			}
		})
	}
}

func TestCmdInit_UnknownRecipe(t *testing.T) {
	_, cleanup := chdirTemp(t)
	defer cleanup()

	code := cmdInit([]string{recipeFlag, "bogus"})
	if code != 1 {
		t.Errorf("unknown recipe exit = %d, want 1", code)
	}
	// Nothing must be created on failure.
	if _, err := os.Stat(".go-arch-lint"); !os.IsNotExist(err) {
		t.Errorf(".go-arch-lint must not exist after failed recipe")
	}
}

// The flag parser accepts both "--recipe x" and "--recipe=x", mirroring the
// equals-form support shipped for other flags (#41).
func TestParseInitArgs(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantPath   string
		wantRecipe string
	}{
		{"none", nil, ".", ""},
		{"space form", []string{"--recipe", tcRecipeDDD}, ".", tcRecipeDDD},
		{"equals form", []string{"--recipe=" + tcRecipeDDD}, ".", tcRecipeDDD},
		{"path space form", []string{"-p", tcTmpPath}, tcTmpPath, ""},
		{"path equals form", []string{"--project-path=" + tcTmpPath}, tcTmpPath, ""},
		{"both", []string{"-p", tcTmpPath, "--recipe=" + tcRecipeClean}, tcTmpPath, tcRecipeClean},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, r := parseInitArgs(tc.args)
			if p != tc.wantPath || r != tc.wantRecipe {
				t.Errorf("parseInitArgs(%v) = (%q, %q), want (%q, %q)", tc.args, p, r, tc.wantPath, tc.wantRecipe)
			}
		})
	}
}
