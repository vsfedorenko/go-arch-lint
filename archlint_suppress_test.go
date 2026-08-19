package archlint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	archlint "github.com/vsfedorenko/go-arch-lint/v2"
	"github.com/vsfedorenko/go-arch-lint/v2/dsl"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
)

/**
 * Black-box tests for //go-arch-lint:ignore suppression directives:
 * a consumer annotates a known violation in source and the check
 * passes, while un-annotated violations still fail it.
 */

// writeSuppressProject creates a project where internal/alpha imports
// internal/beta (disallowed by the test spec) and internal/gamma
// imports internal/alpha (also disallowed). Files whose violations
// should be suppressed carry the given directive lines verbatim.
func writeSuppressProject(t *testing.T, alphaDirective, gammaDirective string) string {
	t.Helper()

	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}

	write("go.mod", "module example.com/suppress\n\ngo 1.25\n")

	alphaSrc := "package alpha\n\n"
	if alphaDirective != "" {
		alphaSrc += alphaDirective + "\n"
	}
	alphaSrc += "import _ \"example.com/suppress/internal/beta\"\n"
	write("internal/alpha/a.go", alphaSrc)

	gammaSrc := "package gamma\n\n"
	if gammaDirective != "" {
		gammaSrc += gammaDirective + "\n"
	}
	gammaSrc += "import _ \"example.com/suppress/internal/alpha\"\n"
	write("internal/gamma/g.go", gammaSrc)

	write("internal/beta/b.go", "package beta\n")

	return root
}

// suppressSpec: nobody may depend on anybody (both imports violate).
func suppressSpec() dsl.SpecDef {
	return dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("alpha", "alpha/**")
		dsl.Component("beta", "beta/**")
		dsl.Component("gamma", "gamma/**")
	})
}

func TestRun_ignore_directive_suppresses_violation(t *testing.T) {
	// gamma's import carries the directive; alpha's does not.
	root := writeSuppressProject(t, "", "//go-arch-lint:ignore")

	err := archlint.Run(suppressSpec(), archlint.WithProjectPath(root))
	require.Error(t, err)
	require.True(t, models.IsUserSpaceError(err), "violations must be a UserSpaceError, got %T: ")
}

func TestRun_ignore_directive_on_same_line_suppresses_violation(t *testing.T) {
	// Trailing directive on the import line itself.
	root := writeSuppressProject(t, "", "import _ \"example.com/suppress/internal/alpha\" //go-arch-lint:ignore")

	err := archlint.Run(suppressSpec(), archlint.WithProjectPath(root))
	require.Error(t, err)
	require.True(t, models.IsUserSpaceError(err), "violations must be a UserSpaceError, got %T: ")
}

func TestRun_ignore_directive_with_target_argument(t *testing.T) {
	// The argument names the dependency target ("alpha"); a directive
	// naming a different target must NOT suppress the violation.
	root := writeSuppressProject(t, "", "//go-arch-lint:ignore beta")

	err := archlint.Run(suppressSpec(), archlint.WithProjectPath(root))
	require.Error(t, err)
	require.True(t, models.IsUserSpaceError(err), "violations must be a UserSpaceError, got %T: ")
}

func TestRun_ignore_file_directive_suppresses_whole_file(t *testing.T) {
	// gamma's file is fully suppressed; alpha's violation stays.
	root := writeSuppressProject(t, "", "//go-arch-lint:ignore-file")

	err := archlint.Run(suppressSpec(), archlint.WithProjectPath(root))
	require.Error(t, err)
}

func TestRun_all_violations_suppressed_passes(t *testing.T) {
	root := writeSuppressProject(t, "//go-arch-lint:ignore", "//go-arch-lint:ignore alpha")

	err := archlint.Run(suppressSpec(), archlint.WithProjectPath(root))
	require.NoError(t, err, "fully suppressed project must pass, got")
	assert.Equal(t, archlint.ExitCodeOK, archlint.ExitCode(err))
}
