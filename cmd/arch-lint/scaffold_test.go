package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	// arch.go is the user-editable spec: v2 DSL import, Spec entry, NO runner.
	archgo, err := os.ReadFile(filepath.Join(archDir, "arch.go"))
	require.NoError(t, err, "read arch.go")
	assert.Contains(t, string(archgo), "Spec(func() {", "arch.go missing Spec entry: %s", archgo)
	assert.Contains(t, string(archgo), `. "github.com/vsfedorenko/go-arch-lint/v3/dsl"`, "arch.go missing dot-import: %s", archgo)
	assert.Contains(t, string(archgo), `Path(".")`, "arch.go missing the module-root fallback for an empty module: %s", archgo)
	assert.NotContains(t, string(archgo), "func main()", "arch.go must not contain the runner: %s", archgo)

	// main.go is the stable runner: command+flag passthrough, NO spec definition.
	maingo, err := os.ReadFile(filepath.Join(archDir, "main.go"))
	require.NoError(t, err, "read main.go")
	assert.Contains(t, string(maingo), "archlint.MustRunCLI(build, os.Args[1:])", "main.go missing command passthrough: %s", maingo)
	assert.NotContains(t, string(maingo), "dsl.Spec(", "main.go must not contain the spec: %s", maingo)
}

// TestScanGoDirs pins the scaffold contract for monorepos: nested modules
// and testdata leave the declared set and land in the exclude list instead —
// a fresh `init` must scaffold a spec `check` agrees with.
func TestScanGoDirs(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755), "mkdir %s", rel)
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600), "write %s", rel)
	}

	write(moduleFileName, "module fixt\n\ngo 1.25\n")
	write("main.go", "package main\n\nfunc main() {}\n")
	write("internal/app/app.go", "package app\n")
	write("internal/testdata/fixture.go", "package testdata\n")
	write("examples/sub/"+moduleFileName, "module fixt/examples/sub\n\ngo 1.25\n")
	write("examples/sub/main.go", "package sub\n")
	write("vendor/x/x.go", "package x\n")
	write(".hidden/h.go", "package h\n")

	dirs, excludes := scanGoDirs(root)

	assert.Equal(t, []string{".", "internal/app"}, dirs,
		"declared dirs must be root + plain Go dirs only, got %v", dirs)
	assert.Equal(t, []string{"examples/sub/**", "internal/testdata/**"}, excludes,
		"nested modules and testdata must be excluded explicitly, got %v", excludes)
}

// Shared fixture paths for spec-render tests (goconst).
const (
	tcDirApp      = "internal/app"
	tcDirDomain   = "internal/domain"
	tcDirHandlers = "internal/handlers"
	tcDirHello    = "internal/hello"
)

// TestV2SpecFromDirs_ExcludeLine pins the rendered spec: excluded globs
// appear in one Exclude call after the Path declarations.
func TestV2SpecFromDirs_ExcludeLine(t *testing.T) {
	spec := v2SpecFromDirs([]string{".", tcDirApp}, []string{"testdata/**"}, map[string][]string{
		".": {tcDirApp},
	})
	assert.Contains(t, spec, `Path("`+tcDirApp+`")`, "spec missing Path decl: %s", spec)
	assert.Contains(t, spec, `Exclude("testdata/**")`, "spec missing Exclude call: %s", spec)

	// Use edges render as Use(...) with declared-first variables: only
	// referenced targets become vars; sources stay as plain statements.
	assert.Regexp(t, `app := Path\("`+tcDirApp+`"\)`,
		spec, "dependency must be declared as a var first")
	assert.Regexp(t, `Path\(".", func\(\) \{ Use\(app\) \}\)`,
		spec, "root's Use rule references the dependency var")

	// no excludes -> no Exclude call and no comment noise
	plain := v2SpecFromDirs([]string{"."}, nil, nil)
	assert.NotContains(t, plain, "Exclude(", "unexpected Exclude in %s", plain)
	assert.Contains(t, plain, `Path(".")`, "empty module falls back to the root: %s", plain)
}

// TestSpecVarNames pins identifier derivation: root, sanitized basenames,
// collision suffixes and keyword escapes.
func TestSpecVarNames(t *testing.T) {
	names := specVarNames([]string{".", "internal/app", "pkg/app", "cmd/type", "weird/My-Dir"})
	assert.Equal(t, "root", names["."], "module root identifier")
	assert.Equal(t, "app", names["internal/app"], "basename identifier")
	assert.Equal(t, "app2", names["pkg/app"], "collision gets a numeric suffix")
	assert.Equal(t, "type_", names["cmd/type"], "keyword gets a trailing underscore")
	assert.Equal(t, "mydir", names["weird/My-Dir"], "symbols fold, case lowers")
}

// TestSpecDeclOrder pins dependencies-first ordering: a referenced path
// is declared before any path that Uses it.
func TestSpecDeclOrder(t *testing.T) {
	dirs := []string{".", tcDirHandlers, tcDirDomain}
	edges := map[string][]string{
		".":           {tcDirHandlers},
		tcDirHandlers: {tcDirDomain},
	}
	order := specDeclOrder(dirs, edges)
	pos := map[string]int{}
	for i, d := range order {
		pos[d] = i
	}
	assert.Less(t, pos["internal/domain"], pos["internal/handlers"],
		"domain must be declared before handlers: %v", order)
	assert.Less(t, pos["internal/handlers"], pos["."],
		"handlers must be declared before the root: %v", order)
}

// TestScanImports pins the import-edge scan: module-path matching,
// test-file skipping and self-import filtering.
func TestScanImports(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755), "mkdir %s", rel)
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600), "write %s", rel)
	}
	write("go.mod", "module fixt\n\ngo 1.25\n")
	write("main.go", "package main\n\nimport \"fixt/internal/hello\"\n")
	write("internal/hello/hello.go", "package hello\n")
	write("internal/hello/hello_test.go", "package hello_test\n\nimport _ \"fixt\"\n")

	edges := scanImports(root, []string{".", "internal/hello"})
	assert.Equal(t, map[string][]string{
		".": {"internal/hello"},
	}, edges, "only the non-test root->hello edge must be scanned")

	// no go.mod -> no edges, scaffold still renders declarations
	nomod := t.TempDir()
	assert.Empty(t, scanImports(nomod, []string{"."}), "missing go.mod disables the scan")
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
	edited := "package main\n\nimport (\n\t. \"github.com/vsfedorenko/go-arch-lint/v3/dsl\"\n)\n\nvar spec = Spec(func() {\n\tVersion(1)\n\tComponent(\"mine\", \"internal/mine/*\")\n})\n"
	require.NoError(t, os.WriteFile(archPath, []byte(edited), 0o600), "write edited arch.go") //nolint:gosec // test fixture in t.TempDir()

	// Regenerate ONLY the runner, as an upgrade would.
	mainPath := filepath.Join(".go-arch-lint", "main.go")
	require.NoError(t, os.WriteFile(mainPath, []byte(scaffoldMainGo), 0o600), "rewrite main.go") //nolint:gosec // test fixture in t.TempDir()

	got, err := os.ReadFile(archPath)
	require.NoError(t, err, "read arch.go")
	assert.Equal(t, edited, string(got), "arch.go changed after runner regeneration")
}

// Shared fixture path for flag-parsing table tests (goconst).
const tcTmpPath = "/tmp/x"

// Regression anchors for the fail-fast flag contract (goconst).
const (
	tcRecipeFlag  = "--recipe"
	tcPositional  = "myproject"
	tcLauncherCmd = "init"
)

// The flag parser accepts both "-p x" and "--project-path=x" forms (#41),
// and fails fast on every token it does not know: a value-less flag, a flag
// whose value is another flag, an unknown flag (the removed `--recipe`),
// and positional arguments — none of them may silently scaffold the default
// spec at the default path (the lenient-flag bug class, #48/#114).
func TestParseInitArgs(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantPath string
		wantErr  string
	}{
		{name: "none", args: nil, wantPath: "."},
		{name: "path space form", args: []string{"-p", tcTmpPath}, wantPath: tcTmpPath},
		{name: "path equals form", args: []string{"--project-path=" + tcTmpPath}, wantPath: tcTmpPath},
		{name: "path long space form", args: []string{flagProjectPath, tcTmpPath}, wantPath: tcTmpPath},
		{name: "value is next flag", args: []string{"--project-path", "-p"}, wantErr: "requires a value"},
		{name: "unknown flag", args: []string{tcRecipeFlag}, wantErr: "unknown flag or argument: " + tcRecipeFlag},
		{name: "removed recipe flag with value", args: []string{tcRecipeFlag, "hexagonal"}, wantErr: "unknown flag or argument: " + tcRecipeFlag},
		{name: "positional argument", args: []string{tcPositional}, wantErr: "unknown flag or argument: " + tcPositional},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, parseErr := parseInitArgs(tc.args)
			if tc.wantErr != "" {
				require.Error(t, parseErr, "parseInitArgs(%v) must fail", tc.args)
				assert.Contains(t, parseErr.Error(), tc.wantErr, "error must name the problem")
				return
			}
			require.NoError(t, parseErr)
			assert.Equal(t, tc.wantPath, p, "parseInitArgs(%v) path", tc.args)
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
	for _, args := range [][]string{{"-p"}, {"--project-path"}} {
		cleanup := chdirTemp(t)
		func() {
			defer cleanup()
			assert.Equal(t, 1, cmdInit(args))
			assert.NoFileExists(t, ".go-arch-lint", "init %v must not create the scaffold")
		}()
	}
}

// Black-box companion to the parseInitArgs table: through the REAL launcher
// binary, an unknown init flag must fail fast naming the token and must not
// create the scaffold. `--recipe` is the regression anchor — the flag was
// removed in v3 (#92), and the README advertised it long after; users typing
// it got a silently default scaffold with exit 0.
func TestLauncher_InitUnknownFlagFailsFast(t *testing.T) {
	bin := buildLauncher(t)
	dir := t.TempDir()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"removed recipe flag", []string{tcLauncherCmd, tcRecipeFlag}, "--recipe"},
		{"removed recipe flag with value", []string{tcLauncherCmd, tcRecipeFlag, "hexagonal"}, "--recipe"},
		{"arbitrary unknown flag", []string{tcLauncherCmd, "--totally-bogus"}, "--totally-bogus"},
		{"positional argument", []string{tcLauncherCmd, tcPositional}, "myproject"},
		{"flag value is a flag", []string{tcLauncherCmd, flagProjectPath, "-p"}, "requires a value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//nolint:gosec // intentional: runs the launcher binary built by this test with fixed table args
			cmd := exec.Command(bin, tt.args...)
			cmd.Dir = dir
			cmd.Env = []string{"PATH=" + t.TempDir(), "HOME=" + t.TempDir()}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr, "init %v must exit non-zero", tt.args)
			assert.Equal(t, 1, exitErr.ExitCode(), "init %v exit code", tt.args)
			assert.Contains(t, stderr.String(), tt.want, "stderr must name the offending token")
			if !strings.Contains(tt.want, "requires a value") {
				assert.NotContains(t, stderr.String(), "requires a value", "value-less message only for the value-less case")
			}
			assert.NoFileExists(t, filepath.Join(dir, ".go-arch-lint"), "no scaffold may be created")
			assert.Empty(t, stdout.String(), "nothing on stdout")
		})
	}
}
