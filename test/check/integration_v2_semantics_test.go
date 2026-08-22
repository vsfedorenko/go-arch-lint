package check_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Black-box pins for v2 DSL semantics that consumer probing of the
// published v2.5.0 surfaced as undocumented: stdlib imports never need
// a Vendor rule, and every directory with Go files must be declared
// with Path — otherwise its files report "not attached to any
// component" and fail the check. The vendor-allowance round trip
// (external import denied without Vendor, allowed with it) is pinned
// here too because it is the documented headline rule of Use.
//
// Fixtures mirror the probe modules: project-v2-sem has core (fmt +
// domain, no external deps) and a spare extra/ component;
// project-v2-vendor has store (pgx + domain).

const archV2SemSpecTpl = `package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3"
	"github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

func main() {
	build := dsl.Spec(func(s *dsl.SpecBuilder) {
		domain := s.Path("domain")
		s.Path("core", func() {
			s.Use(domain)
		})
		%[1]s
	})
	archlint.MustRun(build,
		archlint.WithProjectPath("%[2]s"),
		archlint.WithColors(false),
	)
}
`

const archV2VendorAllowTpl = `package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3"
	"github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

func main() {
	build := dsl.Spec(func(s *dsl.SpecBuilder) {
		domain := s.Path("domain")
		pgx := s.Vendor("pgx", "github.com/jackc/pgx/v5")
		s.Path("store", func() {
			s.Use(domain, pgx)
		})
	})
	archlint.MustRun(build,
		archlint.WithProjectPath("%[1]s"),
		archlint.WithColors(false),
	)
}
`

const archV2VendorDenyTpl = `package main

import (
	"github.com/vsfedorenko/go-arch-lint/v3"
	"github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

func main() {
	build := dsl.Spec(func(s *dsl.SpecBuilder) {
		domain := s.Path("domain")
		s.Path("store", func() {
			s.Use(domain)
		})
	})
	archlint.MustRun(build,
		archlint.WithProjectPath("%[1]s"),
		archlint.WithColors(false),
	)
}
`

func testProjectV2SemDir(t *testing.T) string {
	t.Helper()
	return absFixtureDir(t, "project-v2-sem")
}

func testProjectV2VendorDir(t *testing.T) string {
	t.Helper()
	return absFixtureDir(t, "project-v2-vendor")
}

func absFixtureDir(t *testing.T, name string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(repoRoot(t), "test", "check", name))
	require.NoError(t, err, "abs")
	return abs
}

// TestV2StdlibNeverNeedsVendor: a spec with no Vendor at all still
// passes while the project imports fmt — stdlib is exempt by design.
func TestV2StdlibNeverNeedsVendor(t *testing.T) {
	project := testProjectV2SemDir(t)
	dir := scaffoldArch(t, repoRoot(t), fmt.Sprintf(archV2SemSpecTpl, `s.Path("extra")`, project))

	stdout, stderr, exitCode := runArchLint(t, dir)

	require.Equal(t, 0, exitCode, "stdlib import must not fail the check\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	assert.Contains(t, stdout, "OK - No warnings found", "clean run\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
}

// TestV2UndeclaredDirectoryFails: leaving "extra" (a directory with Go
// files) out of the spec is NOT silently ignored — its files report
// "not attached to any component" and the check exits 1.
func TestV2UndeclaredDirectoryFails(t *testing.T) {
	project := testProjectV2SemDir(t)
	dir := scaffoldArch(t, repoRoot(t), fmt.Sprintf(archV2SemSpecTpl, ``, project))

	stdout, stderr, exitCode := runArchLint(t, dir)

	require.Equal(t, 1, exitCode, "undeclared directory must fail\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	assert.Contains(t, stdout, "not attached to any component", "not-matched file named\nstdout:\n%s", stdout)
	assert.Contains(t, stdout, "extra/util.go", "the undeclared file is named\nstdout:\n%s", stdout)
}

// TestV2VendorRoundTrip: the same fixture is red while pgx is not
// vendored and green once Vendor("pgx", ...) + Use(..., pgx) allow it.
func TestV2VendorRoundTrip(t *testing.T) {
	project := testProjectV2VendorDir(t)

	deny := scaffoldArch(t, repoRoot(t), fmt.Sprintf(archV2VendorDenyTpl, project))
	stdoutD, stderrD, exitD := runArchLint(t, deny)
	require.Equal(t, 1, exitD, "unvendored external import must fail\nstdout:\n%s\nstderr:\n%s", stdoutD, stderrD)
	assert.Contains(t, stdoutD, "github.com/jackc/pgx/v5", "the denied vendor is named\nstdout:\n%s", stdoutD)

	allow := scaffoldArch(t, repoRoot(t), fmt.Sprintf(archV2VendorAllowTpl, project))
	stdoutA, stderrA, exitA := runArchLint(t, allow)
	require.Equal(t, 0, exitA, "vendored external import must pass\nstdout:\n%s\nstderr:\n%s", stdoutA, stderrA)
	assert.Contains(t, stdoutA, "OK - No warnings found", "clean run\nstdout:\n%s\nstderr:\n%s", stdoutA, stderrA)
}
