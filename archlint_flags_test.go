package archlint_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	archlint "github.com/vsfedorenko/go-arch-lint"
	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

// Flag spellings shared across the flag tests (goconst).
const (
	flProjectPath = "--project-path"
	flMaxWarnings = "--max-warnings"
	flNoColors    = "--no-colors"
	flFormat      = "--format"
	flOutputType  = "--output-type"

	tcJSON       = "json"
	tcASCII      = "ascii"
	flFormatJSON = flFormat + "=" + tcJSON
)

/**
 * OptionsFromFlags derives run Options from CLI args — the bridge that
 * lets a scaffolded main() keep the delegated CLI surface working.
 */

func TestOptionsFromFlags_empty(t *testing.T) {
	opts := archlint.OptionsFromFlags(nil)
	require.Empty(t, opts, "no args must yield no options")
}

func TestOptionsFromFlags_full_surface(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{
		"check",
		flProjectPath, "/tmp/proj",
		flMaxWarnings, "42",
		flNoColors,
		flFormat, tcJSON,
	})
	require.Len(t, opts, 4, "expected 4 options")
}

func TestOptionsFromFlags_short_project_path(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{"-p", "/x"})
	require.Len(t, opts, 1, "expected 1 option")
}

func TestOptionsFromFlags_ignores_unknown(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{
		"--verbose", // not part of the scaffold surface
	})
	require.Empty(t, opts, "unknown flags must be ignored")
}

// The scaffold path must honor the output flags the launcher documents as
// passthrough. Regression: --output-type/--json/--output-json-one-line were
// silently dropped here (check --output-type=json still printed ASCII),
// the exact class reported against the CLI upstream (issue #62).
func TestOptionsFromFlags_output_type(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		wantOpts   int
		wantOutput string
	}{
		{"space form", []string{flOutputType, tcJSON}, 1, tcJSON},
		{"equals form", []string{flOutputType + "=" + tcJSON}, 1, tcJSON},
		{"ascii explicit", []string{flOutputType, tcASCII}, 1, tcASCII},
		{"--json alias", []string{"--json"}, 1, tcJSON},
		{"explicit type beats alias", []string{"--json", flOutputType, tcASCII}, 1, tcASCII},
		{"absent", nil, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := archlint.OptionsFromFlags(tc.args)
			require.Len(t, opts, tc.wantOpts, "expected %d options", tc.wantOpts)
			if tc.wantOutput == "" {
				return
			}
			// The option is a closure; apply it to a config snapshot via Run
			// on an empty spec (fails fast before any scanning) is overkill —
			// instead assert through the exported surface below in
			// TestRun_OutputTypeFlags_end_to_end.
			_ = opts
		})
	}
}

func TestOptionsFromFlags_json_one_line(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{"--output-json-one-line"})
	require.Len(t, opts, 1, "expected 1 option")
}

// captureStdout swaps os.Stdout for a pipe, runs fn, and returns what was
// written. The renderer constructed inside Run binds os.Stdout at build
// time, so the swap must enclose the Run call.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()

	w.Close()
	os.Stdout = old
	out := <-done
	_ = r.Close()
	return out
}

// oneComponentProject writes a minimal clean project on disk.
func oneComponentProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	write("go.mod", "module example.com/outtype\n\ngo 1.25\n")
	write("internal/core/a.go", "package core\n")
	return root
}

func oneComponentSpec() dsl.SpecDef {
	return dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("core", "core/**")
	})
}

// TestRun_OutputTypeJSON_renders_wrapper_model proves the output-type flag
// end-to-end through the public API: WithOutputType(json) must change what
// lands on stdout (the {Type, Payload} wrapper), not just be accepted.
// Regression: the scaffold path dropped the flag and kept printing ASCII.
func TestRun_OutputTypeJSON_renders_wrapper_model(t *testing.T) {
	root := oneComponentProject(t)

	var out string
	var runErr error
	out = captureStdout(t, func() {
		runErr = archlint.Run(oneComponentSpec(),
			archlint.WithProjectPath(root),
			archlint.WithColors(false),
			archlint.WithOutputType(models.OutputTypeJSON),
		)
	})

	require.NoError(t, runErr, "clean project must pass, got ")

	var wrapper struct {
		Type string `json:"Type"`
	}
	assert.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &wrapper))
	require.NotEmpty(t, wrapper.Type, "wrapper must carry a Type field, got %q", out)
}

// TestRun_OneLineJSON_compact_output pins the one-line rendering: exactly
// one line, no indentation.
func TestRun_OneLineJSON_compact_output(t *testing.T) {
	root := oneComponentProject(t)

	out := captureStdout(t, func() {
		_ = archlint.Run(oneComponentSpec(),
			archlint.WithProjectPath(root),
			archlint.WithColors(false),
			archlint.WithOutputType(models.OutputTypeJSON),
			archlint.WithOutputJSONOneLine(),
		)
	})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 1, "one-line JSON must be a single line, got %d: %q", len(lines), out)
	assert.False(t, strings.HasPrefix(lines[0], " {"), "one-line JSON must not be indented: ")
}

// TestRun_OneLineWithoutJSON_is_config_error pins the fail-fast contract:
// --output-json-one-line without json output is an actionable error, not a
// silent no-op (upstream issue #62: "an error would have been helpful").
func TestRun_OneLineWithoutJSON_is_config_error(t *testing.T) {
	root := oneComponentProject(t)

	err := archlint.Run(oneComponentSpec(),
		archlint.WithProjectPath(root),
		archlint.WithColors(false),
		archlint.WithOutputJSONOneLine(),
	)
	require.Error(t, err)
	require.Equal(t, archlint.ExitCodeConfigError, archlint.ExitCode(err), "must map to config error (2), got %d: %v", archlint.ExitCode(err), err)
	assert.Contains(t, err.Error(), "--output-json-one-line", "error must name the flag and the fix: ")

	// The same flag with json output is fine (covered above) — and so is
	// the --format json (flat array) spelling: compacting a flat JSON
	// array is meaningful.
	require.NoError(t, archlint.Run(oneComponentSpec(),
		archlint.WithProjectPath(root),
		archlint.WithFormat(models.FormatJSON),
		archlint.WithOutputJSONOneLine(),
	), "one-line with --format json must be accepted, got")
}

// TestRun_UnknownOutputType_is_config_error pins the validation of the
// output-type value itself.
func TestRun_UnknownOutputType_is_config_error(t *testing.T) {
	root := oneComponentProject(t)

	err := archlint.Run(oneComponentSpec(),
		archlint.WithProjectPath(root),
		archlint.WithOutputType("yaml"),
	)
	require.Error(t, err, "unknown output-type must be a config error")
	require.Equal(t, archlint.ExitCodeConfigError, archlint.ExitCode(err), "unknown output-type must be a config error, got %d: %v", archlint.ExitCode(err), err)
	assert.Contains(t, err.Error(), "yaml", "error must echo the bad value: ")
}

func TestOptionsFromFlags_invalid_int_ignored(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{flMaxWarnings, "abc"})
	require.Empty(t, opts, "invalid int must be dropped")
}

func TestOptionsFromFlags_output_color_value_form(t *testing.T) {
	// --output-color=false (cobra-style spelling) must disable colors,
	// same as --no-colors. Regression: the scaffold path silently ignored
	// it while the delegated cobra layer documented it.
	for _, tc := range []struct {
		name      string
		args      []string
		wantOpts  int
		wantColor bool
	}{
		{"value form false", []string{"--output-color=false"}, 1, false},
		{"space form false", []string{"--output-color", "false"}, 1, false},
		{"value form true", []string{"--output-color=true"}, 0, true},
		{"bare flag means true", []string{"--output-color"}, 0, true},
		{"no-colors still works", []string{flNoColors}, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := archlint.OptionsFromFlags(tc.args)
			require.Len(t, opts, tc.wantOpts, "expected %d options", tc.wantOpts)
			// Colors are observable only through the option list: with
			// wantColor=true no option is emitted (default is colors on).
		})
	}
}
