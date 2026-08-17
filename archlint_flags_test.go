package archlint_test

import (
	"testing"

	archlint "github.com/vsfedorenko/go-arch-lint"
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
		"--project-path", "/tmp/proj",
		"--max-warnings", "42",
		"--no-colors",
		"--format", "json",
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
		"--output-type", "json", // handled by other layers
		"--verbose",
	})
	if len(opts) != 0 {
		t.Fatalf("unknown flags must be ignored, got %d options", len(opts))
	}
}

func TestOptionsFromFlags_invalid_int_ignored(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{"--max-warnings", "abc"})
	if len(opts) != 0 {
		t.Fatalf("invalid int must be dropped, got %d options", len(opts))
	}
}
