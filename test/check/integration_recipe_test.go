package check_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// integration_recipe_test.go pins the `init --recipe <name>` black-box
// contract: the scaffolded spec must run end-to-end through the delegated
// `check` command — pass on a conforming project, fail with an actionable
// violation on a broken one, and pass on a partial tree thanks to
// IgnoreNotFoundComponents(true).

// writeRecipeProject materializes a hexagonal-conforming project: core and
// adapters depend inward only. If violate is true, db additionally imports
// the http adapter — a hexagonal violation.
func writeRecipeProject(t *testing.T, dir string, violate bool) {
	t.Helper()
	for _, d := range []string{
		"internal/domain",
		"internal/core",
		"internal/adapter/http",
		"internal/adapter/db",
	} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil { //nolint:gosec // test fixture dirs
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	write := func(rel, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o600); err != nil { //nolint:gosec // test fixture in t.TempDir()
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("go.mod", "module fixt\n\ngo 1.25\n")
	write("internal/domain/model.go", "package domain\n\ntype User struct{ ID string }\n")
	write("internal/core/service.go", "package core\n\nimport \"fixt/internal/domain\"\n\nfunc Get() domain.User { return domain.User{ID: \"1\"} }\n")
	write("internal/adapter/http/handler.go", "package http\n\nimport \"fixt/internal/core\"\n\nfunc Handle() { _ = core.Get() }\n")
	dbBody := "package db\n\nimport \"fixt/internal/core\"\n\nfunc Load() { _ = core.Get() }\n"
	if violate {
		dbBody = "package db\n\nimport (\n\t\"fixt/internal/adapter/http\"\n\t\"fixt/internal/core\"\n)\n\nfunc Load() { _ = core.Get(); _ = http.Handle() }\n"
	}
	write("internal/adapter/db/repo.go", dbBody)
}

// runRecipeCheck delegates to `go run .` in the arch dir with the same
// offline env discipline as runArchLint, returning combined output and the
// child exit code.
func runRecipeCheck(t *testing.T, projectDir string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", "run", ".", "check", "--project-path", projectDir)
	cmd.Dir = filepath.Join(projectDir, ".go-arch-lint")
	cmd.Env = append(os.Environ(),
		"GOFLAGS=-mod=mod",
		"GOSUMDB=off",
		"GOPROXY=off",
	)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	code := 0
	if err != nil {
		code = parseChildExitCode(out.String(), 1)
	}
	return out.String(), code
}

// scaffoldRecipeArchDir builds the arch module the way `init --recipe
// hexagonal` + `go mod tidy` would: recipe spec, local replace, repo go.sum.
func scaffoldRecipeArchDir(t *testing.T, projectDir, repoRoot string) {
	t.Helper()
	archDir := filepath.Join(projectDir, ".go-arch-lint")
	if err := os.MkdirAll(archDir, 0o755); err != nil { //nolint:gosec // test fixture dirs
		t.Fatalf("mkdir arch dir: %v", err)
	}

	goMod := "module arch-lint-local\n\ngo 1.25\n\nrequire github.com/vsfedorenko/go-arch-lint v0.0.0\n\nreplace github.com/vsfedorenko/go-arch-lint => " + repoRoot + "\n"
	files := map[string]string{
		"go.mod":  goMod,
		"arch.go": hexagonalRecipeSpec(),
		"main.go": "package main\n\nimport (\n\t\"os\"\n\n\tarchlint \"github.com/vsfedorenko/go-arch-lint\"\n)\n\nfunc main() {\n\tarchlint.MustRun(spec, archlint.OptionsFromFlags(os.Args[1:])...)\n}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(archDir, name), []byte(content), 0o600); err != nil { //nolint:gosec // test fixture
			t.Fatalf("write %s: %v", name, err)
		}
	}

	sum, err := os.ReadFile(filepath.Join(repoRoot, "go.sum"))
	if err != nil {
		t.Fatalf("read repo go.sum: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archDir, "go.sum"), sum, 0o600); err != nil { //nolint:gosec // test fixture
		t.Fatalf("write go.sum: %v", err)
	}
}

// hexagonalRecipeSpec is byte-identical in shape to the recipe the launcher
// writes for `init --recipe hexagonal` (cmd/arch-lint/recipes.go). Duplicated
// here on purpose: the black-box test must fail if the shipped recipe and the
// pinned contract drift apart — see TestRecipeSpecMatchesLauncher for the
// drift guard, which reads the launcher's own const via a generated scaffold.
func hexagonalRecipeSpec() string {
	return `package main

import (
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

// Ports & adapters: the domain and core logic sit at the center; HTTP and DB
// adapters depend inward on core, never on each other.
var spec = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
		// Directories that do not exist yet are fine while you build the
		// project out; remove this once every component has code.
		IgnoreNotFoundComponents(true)
	})

	ExcludeFiles(` + "`^.*_test\\.go$`" + `)

	Component("domain", "domain")
	Component("core", "core")
	Component("http", "adapter/http")
	Component("db", "adapter/db")

	CommonComponents("domain")

	Deps("core", func() {
		MayDependOn("domain")
	})

	Deps("http", func() {
		MayDependOn("core")
	})

	Deps("db", func() {
		MayDependOn("core")
	})
})
`
}

// TestRecipeHexagonal_BlackBox runs the hexagonal recipe spec against a
// conforming project (expect OK, exit 0) and a violating one (expect the
// db→http violation surfaced, exit 1).
func TestRecipeHexagonal_BlackBox(t *testing.T) {
	root := repoRoot(t)

	okDir := t.TempDir()
	writeRecipeProject(t, okDir, false)
	scaffoldRecipeArchDir(t, okDir, root)
	out, code := runRecipeCheck(t, okDir)
	if code != 0 {
		t.Errorf("conforming project: exit %d, want 0.\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "No warnings found") {
		t.Errorf("conforming project: expected OK output, got:\n%s", out)
	}

	badDir := t.TempDir()
	writeRecipeProject(t, badDir, true)
	scaffoldRecipeArchDir(t, badDir, root)
	out, code = runRecipeCheck(t, badDir)
	if code != 1 {
		t.Errorf("violating project: exit %d, want 1.\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "adapter/http") || !strings.Contains(out, "repo.go") {
		t.Errorf("violating project: db→http violation not surfaced:\n%s", out)
	}
}

// TestRecipeHexagonal_PartialTree verifies the recipe's
// IgnoreNotFoundComponents(true): a project missing half the recipe's
// directories still lints (exit 0), so `init --recipe` is usable before the
// tree exists.
func TestRecipeHexagonal_PartialTree(t *testing.T) {
	root := repoRoot(t)

	dir := t.TempDir()
	writeRecipeProject(t, dir, false)
	if err := os.RemoveAll(filepath.Join(dir, "internal", "adapter")); err != nil {
		t.Fatalf("remove adapter dir: %v", err)
	}
	scaffoldRecipeArchDir(t, dir, root)
	out, code := runRecipeCheck(t, dir)
	if code != 0 {
		t.Errorf("partial tree: exit %d, want 0.\noutput:\n%s", code, out)
	}
}

// TestRecipeSpecMatchesLauncher guards against drift between the recipe
// pinned in this test and the one the launcher actually writes: run the real
// `init --recipe hexagonal` (via the built launcher binary) and compare the
// spec body modulo the package clause noise.
func TestRecipeSpecMatchesLauncher(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the launcher binary")
	}
	root := repoRoot(t)

	// Build the launcher exactly like CI does.
	launcher := filepath.Join(t.TempDir(), "arch-lint")
	build := exec.Command("go", "build", "-o", launcher, "./cmd/arch-lint")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build launcher: %v\n%s", err, out)
	}

	proj := t.TempDir()
	init := exec.Command(launcher, "init", "--recipe", "hexagonal", "-p", proj)
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("init --recipe hexagonal: %v\n%s", err, out)
	}

	got, err := os.ReadFile(filepath.Join(proj, ".go-arch-lint", "arch.go"))
	if err != nil {
		t.Fatalf("read scaffolded arch.go: %v", err)
	}
	if string(got) != hexagonalRecipeSpec() {
		t.Errorf("launcher recipe drifted from the pinned black-box spec.\nlauncher:\n%s\npinned:\n%s", got, hexagonalRecipeSpec())
	}
}
