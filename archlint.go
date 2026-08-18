// Package archlint provides the programmatic (library) API of go-arch-lint.
//
// Instead of shelling out to the go-arch-lint CLI, embed the check directly
// in Go code: build a spec with the [dsl] package, then call [Run] (or
// [MustRun] for CLI-style tools that map results straight to exit codes).
//
// A minimal check looks like:
//
//	import (
//		"github.com/vsfedorenko/go-arch-lint"
//		. "github.com/vsfedorenko/go-arch-lint/dsl"
//	)
//
//	spec := Spec(func() {
//		Version(1)
//		Workdir("internal")
//		Component("handler", "handlers/*")
//		Component("service", "services/**")
//		Deps("handler", func() { MayDependOn("service") })
//	})
//
//	if err := archlint.Run(spec, archlint.WithProjectPath(".")); err != nil {
//		log.Fatal(err)
//	}
//
// Exit codes: [ExitCode] maps a Run error to the conventional linter exit
// code (0 ok / 1 violations / 2 config error), see [ExitCode] for details.
package archlint

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/app"
	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/services/spec/decoder"
)

// Option customizes a [Run] call.
type Option func(*config)

type config struct {
	projectPath string
	maxWarnings int
	useColors   bool
	format      models.Format
}

// WithProjectPath sets the absolute or relative path of the project root to
// lint. Defaults to "../" — the parent of the typical .go-arch-lint/ module
// (adjust when linting from outside the scaffold layout).
func WithProjectPath(path string) Option {
	return func(c *config) { c.projectPath = path }
}

// WithMaxWarnings caps the number of reported violations; a check that finds
// more than n violations fails. Defaults to 512.
func WithMaxWarnings(n int) Option {
	return func(c *config) { c.maxWarnings = n }
}

// WithColors toggles ANSI colors in text output. Defaults to true.
func WithColors(b bool) Option {
	return func(c *config) { c.useColors = b }
}

// WithFormat sets the output format for check results.
// Use models.FormatJSON for a flat JSON array of violations,
// models.FormatSARIF for a SARIF 2.1.0 log (GitHub Code Scanning),
// models.FormatJUnit for a JUnit-style XML report (CI test dashboards),
// models.FormatGitHubActions for GitHub Actions workflow commands
// (::error/::notice PR annotations), or models.FormatText (the default)
// for human-readable ASCII.
func WithFormat(format models.Format) Option {
	return func(c *config) { c.format = format }
}

// OptionsFromFlags derives Options from the process's command-line flags
// (os.Args). This is what a scaffolded `.go-arch-lint/main.go` passes to
// [Run] so the delegated CLI surface keeps working:
//
//	--project-path string   (default "../")
//	-p string               (short form of --project-path)
//	--max-warnings int      (default 512)
//	--no-colors             (disable ANSI colors)
//	--output-color=false    (cobra-style value form of --no-colors)
//	--format text|json|sarif|junit|github-actions ("json" = flat violation array
//	                         for CI, "sarif" = SARIF 2.1.0 log for code scanning,
//	                         "junit" = JUnit XML report for test dashboards,
//	                         "github-actions" = workflow-command annotations)
//
// Unknown flags are ignored rather than rejected: the launcher may pass
// extra flags meant for other layers.
func OptionsFromFlags(args []string) []Option {
	opts := []Option{}

	projectPath := stringFlag(args, "--project-path", "-p")
	if projectPath != "" {
		opts = append(opts, WithProjectPath(projectPath))
	}

	if maxWarnings, ok := intFlag(args, "--max-warnings"); ok {
		opts = append(opts, WithMaxWarnings(maxWarnings))
	}

	if !colorFlagEnabled(args) {
		opts = append(opts, WithColors(false))
	}

	if format := stringFlag(args, "--format"); format != "" {
		opts = append(opts, WithFormat(format))
	}

	return opts
}

// colorFlagEnabled reports whether ANSI colors stay on. Colors are disabled
// by the standalone --no-colors flag or by the cobra-style value form
// --output-color=false (the delegated cobra layer documents --output-color;
// the scaffold path must honor the same spelling).
func colorFlagEnabled(args []string) bool {
	if boolFlag(args, "--no-colors") {
		return false
	}
	if v, ok := boolValueFlag(args, "--output-color"); ok {
		return v
	}
	return true
}

// boolValueFlag parses a cobra-style "--name=value" (or "--name value")
// boolean flag. Returns the value and whether the flag was present.
func boolValueFlag(args []string, name string) (bool, bool) {
	prefix := name + "="
	for i, a := range args {
		if strings.HasPrefix(a, prefix) {
			v, err := strconv.ParseBool(strings.TrimPrefix(a, prefix))
			return v, err == nil
		}
		if a == name {
			// "--output-color false" and bare "--output-color" both mean
			// true (cobra's default), unless the next token parses as a
			// boolean value.
			if i+1 < len(args) {
				if v, err := strconv.ParseBool(args[i+1]); err == nil {
					return v, true
				}
			}
			return true, true
		}
	}
	return false, false
}

// stringFlag returns the value of the first "--name value" (or
// "-s value") occurrence, or "" when absent.
func stringFlag(args []string, names ...string) string {
	for i := 0; i < len(args); i++ {
		for _, name := range names {
			if args[i] == name && i+1 < len(args) {
				return args[i+1]
			}
		}
	}
	return ""
}

// intFlag parses the value of the first "--name value" occurrence.
func intFlag(args []string, name string) (int, bool) {
	raw := stringFlag(args, name)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return n, true
}

// boolFlag reports whether a standalone "--name" flag is present.
func boolFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

// Run lints the project described by spec and returns an error describing
// the result.
//
// A nil return means the architecture check passed. A non-nil error is
// either a violations error ("check ran and found problems", test with
// [models.IsUserSpaceError]) or a config/system error (spec does not
// compile, unreadable project, internal failure). Use [ExitCode] to map the
// error to a process exit code, or [MustRun] for the common CLI pattern.
//
// Run is safe for sequential use; concurrent Run calls are not synchronized.
func Run(spec dsl.SpecDef, opts ...Option) error {
	if spec.Builder() == nil {
		return fmt.Errorf("spec is empty — ensure Spec() was called")
	}

	cfg := config{
		projectPath: "../",
		maxWarnings: 512,
		useColors:   true,
		format:      models.FormatText,
	}
	for _, o := range opts {
		o(&cfg)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// dsl → services boundary: the public API accepts a dsl.SpecDef, the
	// internal app layer consumes a services-layer GoDecoder. Converting
	// here keeps internal/app free of dsl imports (see .go-arch-lint spec).
	return app.RunCheck(ctx, decoder.NewGoDecoder(spec.Builder()), models.CheckOptions{
		ProjectPath: cfg.projectPath,
		MaxWarnings: cfg.maxWarnings,
		UseColors:   cfg.useColors,
		Format:      cfg.format,
	})
}

// Exit codes follow the linter convention (same as golangci-lint):
//
//	0 — success, no violations
//	1 — architecture violations found (or warnings threshold exceeded)
//	2 — configuration / system error (spec does not compile, unreadable
//	    project, internal failure)
//
// CI pipelines can branch on this: fail the build on 1, page a maintainer
// on 2 (a broken config lints nothing).
const (
	ExitCodeOK          = 0
	ExitCodeViolations  = 1
	ExitCodeConfigError = 2
)

// ExitCode maps a Run error to the process exit code.
// A nil error means the check passed: 0. UserSpaceError marks "check ran and
// found violations": 1. ConfigError and anything else is a config/system
// failure: 2.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return ExitCodeOK
	case models.IsUserSpaceError(err):
		return ExitCodeViolations
	default:
		return ExitCodeConfigError
	}
}

// MustRun runs the check and exits the process with a conventional exit code
// (see ExitCode). This is what a scaffolded `.go-arch-lint/main.go` calls.
func MustRun(spec dsl.SpecDef, opts ...Option) {
	os.Exit(ExitCode(Run(spec, opts...)))
}
