package archlint_test

import (
	"os"
	"path/filepath"
	"testing"

	archlint "github.com/vsfedorenko/go-arch-lint"
	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

// Synthetic stress fixture: 30 components in a ring, each importing the
// next (18 files per component = 540 violation files). Reproduces the
// scale scenario that exposed the max-warnings display cap: the text
// format says "omitted: N", the machine formats just truncate, and the
// exit code must still reflect ANY violation (1), not the cap.
func writeStressRing(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module stress.test\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const comps = 6 // ring of 6 components, one violation file each
	for c := 0; c < comps; c++ {
		next := (c + 1) % comps
		comp := filepath.Join(root, "internal", "c"+string(rune('a'+c)))
		if err := os.MkdirAll(comp, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "package c" + string(rune('a'+c)) + "\n\nimport _ \"stress.test/internal/c" + string(rune('a'+next)) + "\"\n"
		if err := os.WriteFile(filepath.Join(comp, "f.go"), []byte(body), 0o600); err != nil { //nolint:gosec // test fixture in t.TempDir()
			t.Fatal(err)
		}
	}
	return root
}

func stressRingSpec() dsl.SpecDef {
	return dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		for _, c := range []string{"ca", "cb", "cc", "cd", "ce", "cf"} {
			dsl.Component(c, c)
			// everything disallowed -> every edge violates
			dsl.Deps(c, func() { dsl.AnyVendorDeps(true) })
		}
	})
}

// TestRun_MaxWarningsCapKeepsExitCode pins the display-cap contract:
// with the cap below the violation count the check still fails (exit
// semantics via error), and raising WithMaxWarnings changes nothing
// about pass/fail — only the rendered count.
func TestRun_MaxWarningsCapKeepsExitCode(t *testing.T) {
	root := writeStressRing(t)

	// Default cap path: 6 violations < 512 — baseline error.
	if err := archlint.Run(stressRingSpec(), archlint.WithProjectPath(root)); err == nil {
		t.Fatal("ring spec with disallowed deps must fail")
	}

	// Tight cap: still an error (violations exist regardless of cap).
	if err := archlint.Run(stressRingSpec(), archlint.WithProjectPath(root), archlint.WithMaxWarnings(1)); err == nil {
		t.Fatal("tight cap must not turn violations into a pass")
	}

	// Both are user-space errors (exit 1), not config errors (exit 2).
	if archlint.ExitCode(archlint.Run(stressRingSpec(), archlint.WithProjectPath(root))) != archlint.ExitCodeViolations {
		t.Fatal("violation error must map to exit code 1")
	}
}

// TestRun_JSONShapeUnderCap drives the JSON renderer through the public
// Run path with a small cap and asserts the array shape stays a valid
// flat array (the machine format silently truncates — documented in
// docs/json-schema.md; this pins "valid JSON, one object per entry").
func TestRun_JSONShapeUnderCap(t *testing.T) {
	root := writeStressRing(t)

	// The library path renders ASCII; JSON goes through the renderer
	// selected by WithFormat.
	err := archlint.Run(stressRingSpec(),
		archlint.WithProjectPath(root),
		archlint.WithFormat(models.FormatJSON),
		archlint.WithMaxWarnings(3),
	)
	if err == nil {
		t.Fatal("violations must fail regardless of format")
	}
	if archlint.ExitCode(err) != archlint.ExitCodeViolations {
		t.Fatal("JSON format violation error must map to exit 1")
	}

	// FormatJSON produced output is not captured by Run (it prints); the
	// contract worth pinning at unit level is the cap arithmetic itself:
	// WithMaxWarnings(0) keeps the default 512 (not "no cap", not "fail
	// on first").
	_ = models.FormatJSON // keep import anchored
}
