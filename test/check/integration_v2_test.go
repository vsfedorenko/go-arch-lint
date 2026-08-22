package check_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// End-to-end coverage for the v2 Path-based DSL pipeline (dsl/dsl.Build ->
// V2SpecDocument -> archlint.RunV2) that PR #84 shipped without a single
// integration test: RunV2 was exported but never executed against a real
// project through the real check pipeline.
//
// Probing this gap live (consumer-style, /tmp module) surfaced three
// root-component bugs fixed alongside: Path(".") produced an empty
// component name (rendering "Component  shouldn't depend on"), Use(root)
// panicked with a misleading "never assigned from Path(...)" (the empty
// name collided with the zero-value PathID), and Use inside Path(".",
// func(){...}) panicked "must be called inside Path" (the from==""
// guard). The canonical root key is now ".".
//
// The fixture (test/check/project-v2) is a nested module: shop/domain is
// the leaf, shop/core uses it (allowed), legacy/** is a subtree component,
// and the module root itself is declared via Path("."). The runner module
// scaffolds a v2 spec around it and runs offline via a replace directive.

const archV2OKTpl = `package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3"
	"github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

func main() {
	build := dsl.Spec(func(s *dsl.SpecBuilder) {
		root := s.Path(".")
		domain := s.Path("shop/domain")

		s.Path("shop/core", func() {
			s.Use(domain)
		})

		s.Path("legacy/**", func() {
			s.Use(domain, root)
		})
	})
	archlint.MustRun(build,
		archlint.WithProjectPath("%s"),
		archlint.WithColors(false),
	)
}
`

// archV2ViolationsTpl runs the same clean spec while the fixture's
// shop/domain is rewritten to import shop/core upward (see
// TestV2PipelineViolations): nothing allows the edge and it closes a
// cycle, so both notices must fire.
const archV2ViolationsTpl = archV2OKTpl

// archV2BrokenDomainSrc rewrites the fixture's domain package into an
// upward importer for the duration of one test.
const archV2BrokenDomainSrc = `package domain

import "v2fixture/shop/core"

type Bad struct{ O core.Order }
`

// archV2MissingPathTpl declares a directory that does not exist: FSVerify
// must turn it into a config error (exit 2) with a did-you-mean hint.
const archV2MissingPathTpl = `package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3"
	"github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

func main() {
	build := dsl.Spec(func(s *dsl.SpecBuilder) {
		s.Path(".")
		typo := s.Path("shop/domian") // typo: the real sibling is shop/domain
		s.Path("shop/core", func() {
			s.Use(typo)
		})
	})
	archlint.MustRun(build,
		archlint.WithProjectPath("%s"),
		archlint.WithColors(false),
	)
}
`

func testProjectV2Dir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(repoRoot(t), "test", "check", "project-v2"))
	require.NoError(t, err, "abs")
	return abs
}

func TestV2PipelineClean(t *testing.T) {
	project := testProjectV2Dir(t)
	root := repoRoot(t)
	dir := scaffoldArch(t, root, fmt.Sprintf(archV2OKTpl, project))

	stdout, stderr, exitCode := runArchLint(t, dir)

	require.Equal(t, 0, exitCode, "exit code\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	assert.Contains(t, stdout, "OK - No warnings found", "stdout:\n%s\nstderr:\n%s", stdout, stderr)
}

func TestV2PipelineViolations(t *testing.T) {
	project := testProjectV2Dir(t)
	root := repoRoot(t)

	// Rewrite shop/domain into an upward importer for this test only.
	domainFile := filepath.Join(project, "shop", "domain", "user.go")
	orig, err := os.ReadFile(domainFile)
	require.NoError(t, err, "read fixture domain")
	t.Cleanup(func() {
		require.NoError(t, os.WriteFile(domainFile, orig, 0o600), "restore fixture domain") //nolint:gosec // test fixture: path is derived from the repo tree, not user input
	})
	require.NoError(t, os.WriteFile(domainFile, []byte(archV2BrokenDomainSrc), 0o600), "rewrite fixture domain") //nolint:gosec // test fixture: same repo-derived path

	dir := scaffoldArch(t, root, fmt.Sprintf(archV2ViolationsTpl, project))

	stdout, stderr, exitCode := runArchLint(t, dir)

	require.Equal(t, 1, exitCode, "violations must exit 1\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	assert.Contains(t, stdout, "shop/domain shouldn't depend on", "upward import reported\nstdout:\n%s", stdout)
	// The upward import closes a cycle: shop/core -> shop/domain -> shop/core.
	assert.Contains(t, stdout, "cycle", "cycle detected\nstdout:\n%s", stdout)
}

func TestV2PipelineMissingPath(t *testing.T) {
	project := testProjectV2Dir(t)
	root := repoRoot(t)
	dir := scaffoldArch(t, root, fmt.Sprintf(archV2MissingPathTpl, project))

	stdout, stderr, exitCode := runArchLint(t, dir)

	combined := stdout + stderr
	require.Equal(t, 2, exitCode, "config error must exit 2\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	assert.Contains(t, combined, "does not exist", "missing path named\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	assert.Contains(t, combined, "did you mean", "suggestion present\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	assert.NotContains(t, combined, "OK - No warnings found", "config errors must never print OK")
}
