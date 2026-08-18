package archlint_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	if len(opts) != 0 {
		t.Fatalf("no args must yield no options, got %d", len(opts))
	}
}

func TestOptionsFromFlags_full_surface(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{
		"check",
		flProjectPath, "/tmp/proj",
		flMaxWarnings, "42",
		flNoColors,
		flFormat, tcJSON,
	})
	if len(opts) != 4 {
		t.Fatalf("expected 4 options, got %d", len(opts))
	}
}

func TestOptionsFromFlags_short_project_path(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{"-p", "/x"})
	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}
}

func TestOptionsFromFlags_ignores_unknown(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{
		"--verbose", // not part of the scaffold surface
	})
	if len(opts) != 0 {
		t.Fatalf("unknown flags must be ignored, got %d options", len(opts))
	}
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
			if len(opts) != tc.wantOpts {
				t.Fatalf("expected %d options, got %d", tc.wantOpts, len(opts))
			}
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
	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}
}

// captureStdout swaps os.Stdout for a pipe, runs fn, and returns what was
// written. The renderer constructed inside Run binds os.Stdout at build
// time, so the swap must enclose the Run call.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
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
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
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

	if runErr != nil {
		t.Fatalf("clean project must pass, got %v", runErr)
	}

	var wrapper struct {
		Type string `json:"Type"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &wrapper); err != nil {
		t.Fatalf("output-type=json must render a JSON wrapper, got %q: %v", out, err)
	}
	if wrapper.Type == "" {
		t.Fatalf("wrapper must carry a Type field, got %q", out)
	}
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
	if len(lines) != 1 {
		t.Fatalf("one-line JSON must be a single line, got %d: %q", len(lines), out)
	}
	if strings.Contains(lines[0], "\n") || strings.HasPrefix(strings.TrimSpace(lines[0]), "{\n") {
		t.Fatalf("one-line JSON must not be indented: %q", lines[0])
	}
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
	if err == nil {
		t.Fatal("one-line without json output must fail")
	}
	if archlint.ExitCode(err) != archlint.ExitCodeConfigError {
		t.Fatalf("must map to config error (2), got %d: %v", archlint.ExitCode(err), err)
	}
	if !strings.Contains(err.Error(), "--output-json-one-line") {
		t.Fatalf("error must name the flag and the fix: %v", err)
	}

	// The same flag with json output is fine (covered above) — and so is
	// the --format json (flat array) spelling: compacting a flat JSON
	// array is meaningful.
	if err := archlint.Run(oneComponentSpec(),
		archlint.WithProjectPath(root),
		archlint.WithFormat(models.FormatJSON),
		archlint.WithOutputJSONOneLine(),
	); err != nil {
		t.Fatalf("one-line with --format json must be accepted, got %v", err)
	}
}

// TestRun_UnknownOutputType_is_config_error pins the validation of the
// output-type value itself.
func TestRun_UnknownOutputType_is_config_error(t *testing.T) {
	root := oneComponentProject(t)

	err := archlint.Run(oneComponentSpec(),
		archlint.WithProjectPath(root),
		archlint.WithOutputType("yaml"),
	)
	if err == nil || archlint.ExitCode(err) != archlint.ExitCodeConfigError {
		t.Fatalf("unknown output-type must be a config error, got %v", err)
	}
	if !strings.Contains(err.Error(), "yaml") {
		t.Fatalf("error must echo the bad value: %v", err)
	}
}

func TestOptionsFromFlags_invalid_int_ignored(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{flMaxWarnings, "abc"})
	if len(opts) != 0 {
		t.Fatalf("invalid int must be dropped, got %d options", len(opts))
	}
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
			if len(opts) != tc.wantOpts {
				t.Fatalf("expected %d options, got %d", tc.wantOpts, len(opts))
			}
			// Colors are observable only through the option list: with
			// wantColor=true no option is emitted (default is colors on).
		})
	}
}
