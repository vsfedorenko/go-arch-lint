package archlint_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	archlint "github.com/vsfedorenko/go-arch-lint"
	"github.com/vsfedorenko/go-arch-lint/dsl"
)

// writeBaselineFixture creates a module with two components where a
// depends on b (a dependency violation under the fixture spec), plus N
// extra violation files if extraViolations > 0. Returns the project root.
func writeBaselineFixture(t *testing.T, extraViolations int) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module baseline.test\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// a -> b is a violation (a may not depend on anything).
	writeViolation := func(dir, pkg, imp string) {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "package " + pkg + "\n\nimport _ \"" + imp + "\"\n"
		if err := os.WriteFile(filepath.Join(root, dir, "f.go"), []byte(body), 0o600); err != nil { //nolint:gosec // test fixture in t.TempDir()
			t.Fatal(err)
		}
	}

	writeViolation("internal/a", "a", "baseline.test/internal/b")
	if err := os.MkdirAll(filepath.Join(root, "internal", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "b", "f.go"), []byte("package b\n"), 0o600); err != nil { //nolint:gosec // test fixture in t.TempDir()
		t.Fatal(err)
	}

	for i := 0; i < extraViolations; i++ {
		dir := "internal/x" + string(rune('a'+i))
		writeViolation(dir, "x"+string(rune('a'+i)), "baseline.test/internal/b")
	}
	return root
}

func baselineSpec() dsl.SpecDef {
	return dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("a", "a/**")
		dsl.Component("b", "b/**")
		// no Deps block for "a": a component without dependency rules may
		// not depend on anything — exactly the fixture we need.
	})
}

// TestRun_BaselineRecordAndCompare walks the full incremental-adoption
// cycle through the public API:
//
//  1. record: violations still reported (exit 1 semantics unchanged),
//     baseline file written with a fingerprint per violation;
//  2. compare, nothing new: check passes (nil error) — known debt is
//     tolerated;
//  3. compare, new violation added: check fails as a user-space error
//     (exit 1), not a config error;
//  4. missing baseline file in compare mode: config error (exit 2) —
//     a missing baseline must never silently pass the check.
func TestRun_BaselineRecordAndCompare(t *testing.T) {
	root := writeBaselineFixture(t, 0)
	baselineFile := filepath.Join(root, ".go-arch-lint", "baseline.json")

	// 1. Without a baseline the violations fail the check.
	err := archlint.Run(baselineSpec(), archlint.WithProjectPath(root))
	if err == nil {
		t.Fatal("fixture must produce a dependency violation")
	}
	if archlint.ExitCode(err) != archlint.ExitCodeViolations {
		t.Fatalf("plain violation must map to exit 1, got %d", archlint.ExitCode(err))
	}

	// Record the baseline. The run still reports the violations (record
	// does not magically fix anything), but writes the file.
	err = archlint.Run(baselineSpec(),
		archlint.WithProjectPath(root),
		archlint.WithBaseline(baselineFile),
		archlint.WithBaselineUpdate(),
	)
	if err == nil {
		t.Fatal("record mode still reports the recorded violations")
	}
	raw, readErr := os.ReadFile(baselineFile)
	if readErr != nil {
		t.Fatalf("baseline file must be written: %v", readErr)
	}

	var doc struct {
		SchemeVersion int               `json:"schemeVersion"`
		Fingerprints  map[string]string `json:"fingerprints"`
	}
	if jsonErr := json.Unmarshal(raw, &doc); jsonErr != nil {
		t.Fatalf("baseline must be valid JSON: %v", jsonErr)
	}
	if doc.SchemeVersion != 1 {
		t.Fatalf("baseline scheme version = %d, want 1", doc.SchemeVersion)
	}
	if len(doc.Fingerprints) != 1 {
		t.Fatalf("baseline must hold 1 fingerprint, got %d (%v)", len(doc.Fingerprints), doc.Fingerprints)
	}

	// 2. Compare with an unchanged project: passes.
	if err := archlint.Run(baselineSpec(),
		archlint.WithProjectPath(root),
		archlint.WithBaseline(baselineFile),
	); err != nil {
		t.Fatalf("known debt must be tolerated: %v", err)
	}

	// 3. A NEW violation appears: user-space failure again.
	root2 := writeBaselineFixture(t, 1)
	baseline2 := filepath.Join(root2, ".go-arch-lint", "baseline.json")
	if err := archlint.Run(baselineSpec(),
		archlint.WithProjectPath(root2),
		archlint.WithBaseline(baseline2),
		archlint.WithBaselineUpdate(),
	); err == nil {
		t.Fatal("record run must report violations")
	}
	// Baseline recorded on the ORIGINAL fixture (one violation), then
	// compared against a project with one EXTRA violation.
	if err := archlint.Run(baselineSpec(),
		archlint.WithProjectPath(root2),
		archlint.WithBaseline(baselineFile),
	); err == nil {
		t.Fatal("new violation must fail the check")
	}
	if archlint.ExitCode(err) != archlint.ExitCodeViolations {
		t.Fatalf("new violation must map to exit 1, got %d", archlint.ExitCode(err))
	}

	// 4. Compare mode without a baseline file: config error, never a pass.
	err = archlint.Run(baselineSpec(),
		archlint.WithProjectPath(root),
		archlint.WithBaseline(filepath.Join(root, "missing-baseline.json")),
	)
	if err == nil {
		t.Fatal("missing baseline must not pass the check")
	}
	if archlint.ExitCode(err) != archlint.ExitCodeConfigError {
		t.Fatalf("missing baseline must map to exit 2, got %d", archlint.ExitCode(err))
	}
}

// TestRun_BaselineFingerprintIgnoresLineNumbers pins the stability
// contract: a fingerprint must survive edits that only shift line
// numbers, otherwise every unrelated edit above a violation would
// resurrect it as "new".
func TestRun_BaselineFingerprintIgnoresLineNumbers(t *testing.T) {
	root := writeBaselineFixture(t, 0)
	baselineFile := filepath.Join(root, ".go-arch-lint", "baseline.json")

	if err := archlint.Run(baselineSpec(),
		archlint.WithProjectPath(root),
		archlint.WithBaseline(baselineFile),
		archlint.WithBaselineUpdate(),
	); err == nil {
		t.Fatal("record run must report violations")
	}

	// Grow the violating file ABOVE the import (a comment header): the
	// violation moves to a different line but stays the same debt.
	target := filepath.Join(root, "internal", "a", "f.go")
	shifted := "// package header line 1\n// package header line 2\n// package header line 3\n"
	if err := os.WriteFile(target, []byte(shifted+"package a\n\nimport _ \"baseline.test/internal/b\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := archlint.Run(baselineSpec(),
		archlint.WithProjectPath(root),
		archlint.WithBaseline(baselineFile),
	); err != nil {
		t.Fatalf("line-shifted violation must stay known: %v", err)
	}
}
