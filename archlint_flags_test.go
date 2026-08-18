package archlint_test

import (
	"testing"

	archlint "github.com/vsfedorenko/go-arch-lint"
)

// Flag spellings shared across the flag tests (goconst).
const (
	flProjectPath = "--project-path"
	flMaxWarnings = "--max-warnings"
	flNoColors    = "--no-colors"
	flFormat      = "--format"

	tcJSON       = "json"
	flFormatJSON = flFormat + "=json"
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
		"--output-type", tcJSON, // handled by other layers
		"--verbose",
	})
	if len(opts) != 0 {
		t.Fatalf("unknown flags must be ignored, got %d options", len(opts))
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
