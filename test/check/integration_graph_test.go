package check_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// integration_graph_test.go pins the graph command's output hygiene: a
// scaffolded `graph` run must write the svg and print only the success
// banner. The d2 layout engine used to log "missing slog.Logger in
// context" WARNs with full goroutine stacks (two per run) onto stdout
// because the context carried no slog.Logger — found by probing the
// released v3.0.3 artifact as a consumer.

// runArchGraph runs the scaffolded graph CLI from the project's
// .go-arch-lint module, offline (repo replace + repo go.sum), mirroring
// runArchCheck.
func runArchGraph(t *testing.T, projectDir, outFile string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", "run", ".", "graph", "--project-path", projectDir, "--out", outFile)
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

// TestGraphOutputHasNoD2LoggerNoise pins the hygiene contract end-to-end:
// graph succeeds, writes the svg next to the expected path, and its
// output stays free of d2's internal logger diagnostics.
func TestGraphOutputHasNoD2LoggerNoise(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the launcher binary")
	}
	root := repoRoot(t)

	proj := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(proj, "internal", "app"), 0o755), "mkdir internal/app")       //nolint:gosec // test fixture dirs
	require.NoError(t, os.MkdirAll(filepath.Join(proj, "internal", "domain"), 0o755), "mkdir internal/domain") //nolint:gosec // test fixture dirs
	writeFile := func(rel, body string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(proj, rel), []byte(body), 0o600), "write %s", rel) //nolint:gosec // test fixture in t.TempDir()
	}
	writeFile("go.mod", "module fixt\n\ngo 1.25\n")
	writeFile("main.go", "package main\n\nfunc main() {}\n")
	writeFile("internal/domain/user.go", "package domain\n\ntype User struct{ ID int }\n")
	writeFile("internal/app/service.go", "package app\n\nimport \"fixt/internal/domain\"\n\nfunc Get(u domain.User) domain.User { return u }\n")

	scaffoldDefaultArchDir(t, proj, root)

	// A real user edits arch.go after init: two components with a Use rule
	// give the d2 layout engine edges to place — the code path where the
	// logger noise fired (a single-component scaffold graph stays trivial).
	writeFile(filepath.Join(".go-arch-lint", "arch.go"), `package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = dsl.Spec(func(s *dsl.SpecBuilder) {
	domain := s.Path("internal/domain")
	s.Path("internal/app", func() { s.Use(domain) })
})
`)

	outFile := filepath.Join(proj, "graph.svg")
	out, code := runArchGraph(t, proj, outFile)
	assert.Equal(t, 0, code, "graph must exit 0; exit %d.\noutput:\n%s", code, out)
	assert.Contains(t, out, "Graph outputted to", "expected success banner; output:\n%s", out)
	assert.NotContains(t, out, "missing slog.Logger", "d2 logger noise must not leak to CLI output; output:\n%s", out)
	assert.NotContains(t, out, "goroutine 1 [running]", "no stack traces in CLI output; output:\n%s", out)

	svg, err := os.ReadFile(outFile)
	require.NoError(t, err, "svg file must exist")
	assert.True(t, strings.HasPrefix(string(svg), "<?xml"), "svg content must be xml")
}
