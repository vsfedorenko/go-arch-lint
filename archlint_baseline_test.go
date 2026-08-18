package archlint_test

import (
	"strings"
	"testing"

	archlint "github.com/vsfedorenko/go-arch-lint"
	"github.com/vsfedorenko/go-arch-lint/dsl"
)

// Synthetic baseline-mode contracts (found by probing the freshly merged
// --baseline feature):
//
//  1. --baseline-update without --baseline silently no-op'ed (nothing
//     recorded, exit 1 from ordinary violations) — now a ConfigError.
//  2. A broken/missing baseline file rendered the empty check model:
//     "OK - No warnings found" on stdout with exit 2 — actively
//     misleading in CI.
func TestRun_BaselineUpdateWithoutBaseline_IsConfigError(t *testing.T) {
	spec := dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Component("main", "app")
	})

	err := archlint.Run(spec,
		archlint.WithProjectPath("../"),
		archlint.WithBaselineUpdate(),
	)
	if err == nil {
		t.Fatal("--baseline-update without --baseline must fail")
	}
	if archlint.ExitCode(err) != archlint.ExitCodeConfigError {
		t.Fatalf("must map to config error (2), got %d: %v", archlint.ExitCode(err), err)
	}
	if !strings.Contains(err.Error(), "requires --baseline") {
		t.Fatalf("error must name the missing flag: %v", err)
	}
}
