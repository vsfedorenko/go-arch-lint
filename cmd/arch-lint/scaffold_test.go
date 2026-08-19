package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chdirTemp creates a temp dir, chdirs into it, and returns a cleanup func.
func chdirTemp(t *testing.T) (cleanup func()) {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	require.NoError(t, err, "getwd")
	require.NoError(t, os.Chdir(dir))
	return func() {
		assert.NoError(t, os.Chdir(orig))
	}
}

func TestCmdInit_CreatesScaffold(t *testing.T) {
	cleanup := chdirTemp(t)
	defer cleanup()

	assert.Equal(t, 0, cmdInit(nil))

	archDir := ".go-arch-lint"
	for _, name := range []string{"go.mod", "arch.go", "main.go"} {
		path := filepath.Join(archDir, name)
		info, err := os.Stat(path)
		if err != nil {
			assert.FileExists(t, path, "expected file: %v", err)
			continue
		}
		assert.NotZero(t, info.Size(), "file %s is empty", path)
	}

	// Verify go.mod has the local module declaration (require line is added by 'go mod tidy')
	gomod, err := os.ReadFile(filepath.Join(archDir, "go.mod"))
	require.NoError(t, err, "read go.mod")
	assert.Contains(t, string(gomod), "module arch-lint-local", "go.mod missing module declaration: %s", gomod)

	// arch.go is the user-editable spec: DSL import, Spec entry, NO runner.
	archgo, err := os.ReadFile(filepath.Join(archDir, "arch.go"))
	require.NoError(t, err, "read arch.go")
	assert.Contains(t, string(archgo), "Spec(func()", "arch.go missing Spec entry: %s", archgo)
	assert.Contains(t, string(archgo), `"github.com/vsfedorenko/go-arch-lint/dsl"`, "arch.go missing dsl import: %s", archgo)
	assert.NotContains(t, string(archgo), "func main()", "arch.go must not contain the runner: %s", archgo)

	// main.go is the stable runner: flag passthrough, NO spec definition.
	maingo, err := os.ReadFile(filepath.Join(archDir, "main.go"))
	require.NoError(t, err, "read main.go")
	assert.Contains(t, string(maingo), "archlint.MustRun(spec, archlint.OptionsFromFlags(os.Args[1:])...)", "main.go missing flag passthrough: %s", maingo)
	assert.NotContains(t, string(maingo), "Spec(func()", "main.go must not contain the spec: %s", maingo)
}

// The scaffolded ExcludeFiles regex must escape the dot with ONE backslash
// (`\.`), so it actually matches *_test.go files. A historical double escape
// (`\\.`) required a literal backslash in file names and never matched.
func TestScaffold_ExcludeFilesRegexEscapesDotOnce(t *testing.T) {
	cleanup := chdirTemp(t)
	defer cleanup()

	assert.Equal(t, 0, cmdInit(nil))

	archgo, err := os.ReadFile(filepath.Join(".go-arch-lint", "arch.go"))
	require.NoError(t, err, "read arch.go")
	assert.NotContains(t, string(archgo), `\\.`, "ExcludeFiles double backslash matches no files:\n%s", archgo)
	assert.Contains(t, string(archgo), "^.*_test\\.go$", "arch.go missing ExcludeFiles pattern:\n%s", archgo)
}

func TestCmdInit_AlreadyExists(t *testing.T) {
	cleanup := chdirTemp(t)
	defer cleanup()

	// First init succeeds
	assert.Equal(t, 0, cmdInit(nil))

	// Second init must fail with code 1
	code := cmdInit(nil)
	assert.Equal(t, 1, code, "second cmdInit returned %d, want 1")
}

// The split exists so the runner can be regenerated without touching the
// user's spec: overwriting main.go with a fresh scaffold copy must keep
// arch.go byte-identical.
func TestScaffoldSplit_RegenerateRunnerKeepsSpec(t *testing.T) {
	cleanup := chdirTemp(t)
	defer cleanup()

	assert.Equal(t, 0, cmdInit(nil))

	// Simulate the user editing their spec.
	archPath := filepath.Join(".go-arch-lint", "arch.go")
	edited := "package main\n\nimport (\n\t. \"github.com/vsfedorenko/go-arch-lint/dsl\"\n)\n\nvar spec = Spec(func() {\n\tVersion(1)\n\tComponent(\"mine\", \"internal/mine/*\")\n})\n"
	require.NoError(t, os.WriteFile(archPath, []byte(edited), 0o600), "write edited arch.go") //nolint:gosec // test fixture in t.TempDir()

	// Regenerate ONLY the runner, as an upgrade would.
	mainPath := filepath.Join(".go-arch-lint", "main.go")
	require.NoError(t, os.WriteFile(mainPath, []byte(scaffoldMainGo), 0o600), "rewrite main.go") //nolint:gosec // test fixture in t.TempDir()

	got, err := os.ReadFile(archPath)
	require.NoError(t, err, "read arch.go")
	assert.Equal(t, edited, string(got), "arch.go changed after runner regeneration")
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
			cleanup := chdirTemp(t)
			defer cleanup()

			code := cmdInit([]string{recipeFlag, recipe})
			require.Equal(t, 0, code, "cmdInit --recipe %s returned %d, want 0")

			archgo, err := os.ReadFile(filepath.Join(".go-arch-lint", "arch.go"))
			require.NoError(t, err, "read arch.go")
			s := string(archgo)
			assert.Contains(t, s, "Spec(func()", "%s: arch.go missing Spec entry", recipe)
			assert.Contains(t, s, `. "github.com/vsfedorenko/go-arch-lint/dsl"`, "%s: arch.go missing dsl import", recipe)
			assert.NotContains(t, s, "func main()", "%s: arch.go must not contain the runner", recipe)
			assert.NotContains(t, s, `\\.`, "%s: ExcludeFiles double-backslash bug present", recipe)
			// Recipes tolerate not-yet-created directories.
			assert.Contains(t, s, "IgnoreNotFoundComponents(true)", "%s: recipe must set IgnoreNotFoundComponents(true)", recipe)

			// go.mod and main.go are shared with the plain scaffold.
			gomod, err := os.ReadFile(filepath.Join(".go-arch-lint", "go.mod"))
			require.NoError(t, err, "%s: bad go.mod", recipe)
			assert.Contains(t, string(gomod), "module arch-lint-local", "%s: bad go.mod", recipe)
			maingo, err := os.ReadFile(filepath.Join(".go-arch-lint", "main.go"))
			require.NoError(t, err, "%s: bad main.go", recipe)
			assert.Contains(t, string(maingo), "archlint.MustRun", "%s: bad main.go", recipe)
		})
	}
}

func TestCmdInit_UnknownRecipe(t *testing.T) {
	cleanup := chdirTemp(t)
	defer cleanup()

	code := cmdInit([]string{recipeFlag, "bogus"})
	assert.Equal(t, 1, code, "unknown recipe exit = %d, want 1")
	// Nothing must be created on failure.
	assert.NoFileExists(t, ".go-arch-lint", "must not exist after failed recipe")
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
			p, r, parseErr := parseInitArgs(tc.args)
			require.NoError(t, parseErr)
			assert.Equal(t, tc.wantPath, p, "parseInitArgs(%v) path", tc.args)
			assert.Equal(t, tc.wantRecipe, r, "parseInitArgs(%v) recipe", tc.args)
		})
	}
}

// Regression: `init --help` previously scaffolded a project instead of
// showing help; `--recipe` without a value silently created the DEFAULT
// spec while the user believed they had chosen a recipe.
func TestCmdInit_HelpDoesNotScaffold(t *testing.T) {
	cleanup := chdirTemp(t)
	defer cleanup()

	assert.Equal(t, 0, cmdInit([]string{flagHelp}))
	_, err := os.Stat(".go-arch-lint")
	require.True(t, os.IsNotExist(err), "init --help must not create the scaffold")
}

func TestCmdInit_ValuelessFlagsFailFast(t *testing.T) {
	for _, args := range [][]string{{"--recipe"}, {"-p"}, {"--project-path"}} {
		cleanup := chdirTemp(t)
		func() {
			defer cleanup()
			assert.Equal(t, 1, cmdInit(args))
			assert.NoFileExists(t, ".go-arch-lint", "init %v must not create the scaffold")
		}()
	}
}
