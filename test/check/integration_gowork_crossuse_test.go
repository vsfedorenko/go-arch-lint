package check_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// integration_gowork_crossuse_test.go pins the go.work cross-module
// contract: a workspace member with its OWN module path (example.com/y, not
// example.com/root/two/y) declared as a component must be a first-class
// dependency — Path("two/y/**") attaches its files, Use(y) allows the root
// component to import it, and the import classifies as project code, not
// vendor. Probe-found on v3.1.6: the import was classified vendor and the
// allowed import path was built from the root module, so a legitimate
// cross-module Use always fired "shouldn't depend".

// TestGoWorkCrossModuleUseGreen scaffolds root module + sibling member with
// an independent module path and asserts the allowed cross-import is green
// through the real launcher.
func TestGoWorkCrossModuleUseGreen(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the launcher binary")
	}
	root := repoRoot(t)

	proj := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(proj, "two", "y", "api"), 0o755), "mkdir two/y/api") //nolint:gosec // test fixture dirs
	write := func(rel, body string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(proj, rel), []byte(body), 0o600), "write %s", rel) //nolint:gosec // test fixture in t.TempDir()
	}
	write("go.mod", "module example.com/root\n\ngo 1.25\n")
	write("go.work", "go 1.25\n\nuse (\n\t.\n\t./two/y\n)\n")
	write("main.go", "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/y/api\"\n)\n\nfunc main() { fmt.Println(api.Hello()) }\n")
	write("two/y/go.mod", "module example.com/y\n\ngo 1.25\n")
	write("two/y/api/api.go", "package api\n\nfunc Hello() string { return \"hello\" }\n")

	launcher := buildLauncherFor(t, root)
	scaffoldDefaultArchDir(t, proj, root)

	// Path("two/y/**") attaches the member's files; Use(y) allows the root
	// component to import it — the exact spec the README documents.
	spec := `package main

import (
	. "github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = Spec(func() {
	y := Path("two/y/**")
	Path(".", func() {
		Use(y)
	})
})
`
	require.NoError(t, os.WriteFile(filepath.Join(proj, ".go-arch-lint", "arch.go"), []byte(spec), 0o600), "write arch spec") //nolint:gosec // test fixture in t.TempDir()

	out, code := runLauncherCheck(t, launcher, proj)
	assert.Equal(t, 0, code, "allowed cross-module Use must be green; exit %d.\noutput:\n%s", code, out)
	assert.Contains(t, stripANSI(out), "No warnings found", "expected OK banner for allowed cross-module import")
}

// TestGoWorkCrossModuleUseForbidden asserts the same layout WITHOUT the
// Use allowance still fails: the fix must not silently allow everything.
func TestGoWorkCrossModuleUseForbidden(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the launcher binary")
	}
	root := repoRoot(t)

	proj := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(proj, "two", "y", "api"), 0o755), "mkdir two/y/api") //nolint:gosec // test fixture dirs
	write := func(rel, body string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(proj, rel), []byte(body), 0o600), "write %s", rel) //nolint:gosec // test fixture in t.TempDir()
	}
	write("go.mod", "module example.com/root\n\ngo 1.25\n")
	write("go.work", "go 1.25\n\nuse (\n\t.\n\t./two/y\n)\n")
	write("main.go", "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/y/api\"\n)\n\nfunc main() { fmt.Println(api.Hello()) }\n")
	write("two/y/go.mod", "module example.com/y\n\ngo 1.25\n")
	write("two/y/api/api.go", "package api\n\nfunc Hello() string { return \"hello\" }\n")

	launcher := buildLauncherFor(t, root)
	scaffoldDefaultArchDir(t, proj, root)

	// No Use(y): the import must stay a violation.
	spec := `package main

import (
	. "github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = Spec(func() {
	Path("two/y/**")
	Path(".")
})
`
	require.NoError(t, os.WriteFile(filepath.Join(proj, ".go-arch-lint", "arch.go"), []byte(spec), 0o600), "write arch spec") //nolint:gosec // test fixture in t.TempDir()

	out, code := runLauncherCheck(t, launcher, proj)
	assert.Equal(t, 1, code, "unallowed cross-module import must fail; exit %d.\noutput:\n%s", code, out)
	assert.Contains(t, stripANSI(out), "shouldn't depend on example.com/y/api", "violation must name the member import")
}

// stripANSI removes color escape sequences so sentence-level assertions do
// not depend on the color codes interleaved inside the violation text.
func stripANSI(s string) string {
	const ansi = "\x1b"
	out := make([]rune, 0, len(s))
	inEscape := false
	for _, r := range s {
		switch {
		case r == 27: // ESC begins an escape sequence
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			out = append(out, r)
		}
	}
	_ = ansi
	return string(out)
}
