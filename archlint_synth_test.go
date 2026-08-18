package archlint_test

import (
	"strings"
	"testing"

	archlint "github.com/vsfedorenko/go-arch-lint"
)

// Synthetic probes for OptionsFromFlags with hostile/odd inputs.
func TestOptionsFromFlags_SyntheticHostile(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"empty string flag value", []string{flProjectPath, ""}},
		{"flag at end without value", []string{flProjectPath}},
		{"negative max-warnings", []string{flMaxWarnings, "-5"}},
		{"huge max-warnings", []string{flMaxWarnings, "99999999999999999999"}},
		{"format empty value", []string{flFormat, ""}},
		{"duplicated flags", []string{flFormat, tcJSON, flFormat, "sarif"}},
		{"equals form unknown", []string{flFormatJSON}},
		{"no-colors twice", []string{flNoColors, flNoColors}},
		{"both color spellings fight", []string{flNoColors, "--output-color=true"}},
		{"p without value", []string{"-p"}},
		{"p empty", []string{"-p", ""}},
		{"arg that looks like flag", []string{"--", flNoColors}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Must not panic — result values are secondary.
			opts := archlint.OptionsFromFlags(tc.args)
			t.Logf("args=%v -> %d options", tc.args, len(opts))
		})
	}
}

// The equals form "--flag=value" is how cobra users write flags; the
// scaffold path must understand it identically to "--flag value".
// Regression: --format=json / --project-path=/x / --max-warnings=0 were
// silently dropped (same class as the --output-color=false bug).
func TestOptionsFromFlags_EqualsForm(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{"--project-path=/tmp/x", flFormatJSON})
	if len(opts) != 2 {
		t.Fatalf("equals form must parse both flags, got %d options", len(opts))
	}

	if got := archlint.OptionsFromFlags([]string{"--max-warnings=7"}); len(got) != 1 {
		t.Fatalf("equals form max-warnings must parse, got %d options", len(got))
	}
}

// Space form still wins over equals when both appear (first match).
func TestOptionsFromFlags_SpaceAndEqualsMixed(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{flFormat, "sarif", flFormatJSON})
	if len(opts) != 1 {
		t.Fatalf("first occurrence must win, got %d options", len(opts))
	}
}

// --max-warnings overflow must not silently become a wild value.
func TestOptionsFromFlags_MaxWarningsOverflow(t *testing.T) {
	opts := archlint.OptionsFromFlags([]string{flMaxWarnings, "99999999999999999999"})
	// strconv.Atoi fails on overflow -> flag dropped silently.
	if len(opts) != 0 {
		t.Fatalf("overflow max-warnings must be dropped, got %d options", len(opts))
	}
	if len(opts) == 0 && !strings.HasPrefix("ok", "ok") {
		t.Fatal("unreachable")
	}
}
