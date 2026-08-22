// Package archlint provides the programmatic (library) API of go-arch-lint.
//
// Instead of shelling out to the go-arch-lint CLI, embed the check directly
// in Go code: build a spec with the [dsl] package, then call [Run] (or
// [MustRun] for CLI-style tools that map results straight to exit codes).
//
// A minimal check looks like:
//
//	import (
//		"github.com/vsfedorenko/go-arch-lint/v2"
//		. "github.com/vsfedorenko/go-arch-lint/v2/dsl"
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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/v2/dsl"
	v2 "github.com/vsfedorenko/go-arch-lint/v2/dsl/v2"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/app"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/spec/decoder"
)

// Option customizes a [Run] call.
type Option func(*config)

type config struct {
	projectPath       string
	maxWarnings       int
	useColors         bool
	format            models.Format
	outputType        models.OutputType
	outputJSONOneLine bool
	baselinePath      string
	baselineUpdate    bool
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

// WithOutputType selects the wrapper rendering of check results:
// "ascii" (default, human-readable) or "json" (the {Type, Payload}
// wrapper model — not the flat violation array; see [WithFormat]).
// Mirrors the CLI flag --output-type (alias --json).
func WithOutputType(t models.OutputType) Option {
	return func(c *config) { c.outputType = t }
}

// WithOutputJSONOneLine renders the json output type as a single-line
// payload (no indentation). Mirrors the CLI flag
// --output-json-one-line. Requires [WithOutputType] json — the run
// fails fast otherwise instead of silently ignoring the option.
func WithOutputJSONOneLine() Option {
	return func(c *config) { c.outputJSONOneLine = true }
}

// WithBaseline enables the incremental adoption mode: violations whose
// fingerprints are recorded in the baseline file at path are tolerated
// as known debt, and only NEW violations fail the check. Record the
// baseline with [WithBaselineUpdate]. Pair with the conventional
// .go-arch-lint/baseline.json committed to the repository.
func WithBaseline(path string) Option {
	return func(c *config) { c.baselinePath = path }
}

// WithBaselineUpdate switches the baseline mode from compare to record:
// the check writes the current full violation set to the baseline file
// instead of comparing against it (a run with an empty project state
// writes an empty baseline). Requires [WithBaseline].
func WithBaselineUpdate() Option {
	return func(c *config) { c.baselineUpdate = true }
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
//	--format text|json|sarif|junit|github-actions|html ("json" = flat violation array
//	                         for CI, "sarif" = SARIF 2.1.0 log for code scanning,
//	                         "junit" = JUnit XML report for test dashboards,
//	                         "github-actions" = workflow-command annotations,
//	                         "html" = standalone HTML report for humans/archives)
//	--output-type string   ascii (default) or json — the {Type, Payload} wrapper
//	                         model; --json is the alias for =json
//	--output-json-one-line render json output as a single line (no indentation);
//	                         requires json output, rejected otherwise
//	--baseline string       (baseline file with known violations; only NEW
//	                         violations fail the check — incremental adoption)
//	--baseline-update       (record current violations as the baseline)
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

	// --json is the documented alias for --output-type=json; an explicit
	// --output-type wins (the cobra layer rejects the conflicting pair,
	// here the alias just yields).
	outputType := stringFlag(args, "--output-type")
	if outputType == "" && boolFlag(args, "--json") {
		outputType = models.OutputTypeJSON
	}
	if outputType != "" {
		opts = append(opts, WithOutputType(outputType))
	}

	if boolFlag(args, "--output-json-one-line") {
		opts = append(opts, WithOutputJSONOneLine())
	}

	if baselinePath := stringFlag(args, "--baseline"); baselinePath != "" {
		opts = append(opts, WithBaseline(baselinePath))
	}

	if boolFlag(args, "--baseline-update") {
		opts = append(opts, WithBaselineUpdate())
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
// "-s value") occurrence, or "" when absent. The cobra-style equals
// form "--name=value" is recognized with the same semantics: users
// write flags both ways, and silently dropping one spelling makes CI
// pipelines misbehave (see the --output-color fix for the same class).
func stringFlag(args []string, names ...string) string {
	for i := range args {
		for _, name := range names {
			if args[i] == name && i+1 < len(args) {
				return args[i+1]
			}
			if v, ok := strings.CutPrefix(args[i], name+"="); ok {
				return v
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

	checkOpts := models.CheckOptions{
		ProjectPath:       cfg.projectPath,
		MaxWarnings:       cfg.maxWarnings,
		UseColors:         cfg.useColors,
		Format:            cfg.format,
		OutputType:        cfg.outputType,
		OutputJSONOneLine: cfg.outputJSONOneLine,
		BaselinePath:      cfg.baselinePath,
		BaselineUpdate:    cfg.baselineUpdate,
	}

	// Fail fast on incoherent flag combinations instead of silently
	// no-op'ing the run: --baseline-update without --baseline recorded
	// nothing and the check still ran (violations failed the build with
	// no baseline written) — confusing in CI. The rules live on
	// models.CheckOptions so the cobra entry path (scaffolded runners
	// via MustRunCLI) enforces the exact same contracts.
	if err := checkOpts.ValidateFlagPairs(); err != nil {
		return err
	}

	switch cfg.outputType {
	case "", models.OutputTypeDefault, models.OutputTypeASCII, models.OutputTypeJSON:
		// ok — empty/default falls back to ascii downstream
	default:
		return models.NewConfigError(fmt.Sprintf("unknown output-type %q: expected %s or %s (see --json)", cfg.outputType, models.OutputTypeASCII, models.OutputTypeJSON))
	}

	// dsl → services boundary: the public API accepts a dsl.SpecDef, the
	// internal app layer consumes a services-layer GoDecoder. Converting
	// here keeps internal/app free of dsl imports (see .go-arch-lint spec).
	return app.RunCheck(ctx, decoder.NewGoDecoder(spec.Builder()), checkOpts)
}

// RunV2 executes a check driven by a v2 Path-based DSL build (dsl/v2).
// It is the v2 counterpart of [Run]: the build is first verified against
// the real filesystem (missing directories and empty /** subtrees become
// config errors with file:line of the offending Path call), then the same
// check pipeline runs. Stage 2 of the v3 roadmap; the v1 Run stays until
// the /v3 module bump removes it.
func RunV2(build *v2.Build, opts ...Option) error {
	if build == nil {
		return fmt.Errorf("build is empty — ensure v2.Spec(...) was called")
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

	absProject, err := filepath.Abs(cfg.projectPath)
	if err != nil {
		return fmt.Errorf("resolve project path: %w", err)
	}

	doc := decoder.NewV2SpecDocument(build)
	if notices := doc.FSVerify(absProject); len(notices) > 0 {
		first := notices[0]
		return models.NewConfigError(fmt.Sprintf("%s:%d: %s", first.Ref.File, first.Ref.Line, first.Notice))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checkOpts := models.CheckOptions{
		ProjectPath:       cfg.projectPath,
		MaxWarnings:       cfg.maxWarnings,
		UseColors:         cfg.useColors,
		Format:            cfg.format,
		OutputType:        cfg.outputType,
		OutputJSONOneLine: cfg.outputJSONOneLine,
		BaselinePath:      cfg.baselinePath,
		BaselineUpdate:    cfg.baselineUpdate,
	}
	if err := checkOpts.ValidateFlagPairs(); err != nil {
		return err
	}

	return app.RunCheck(ctx, doc, checkOpts)
}

// RunCLI executes the delegated CLI command surface (check, mapping,
// graph, self-inspect, version) against spec, with args being the
// process arguments without the binary name (os.Args[1:], command
// included). This is what a scaffolded `.go-arch-lint/main.go` calls so
// that EVERY delegated command keeps its own behavior: the launcher
// forwards `mapping`/`graph`/`selfInspect` by name, and a runner that
// only calls [Run] silently degrades them all to a check run.
//
// Launcher-dialect spellings are translated: `-p` → `--project-path`,
// `--no-colors` → `--output-color=false`, `selfInspect` → the
// `self-inspect` command. A list without a command defaults to `check`
// (bare `go run .go-arch-lint/` keeps linting instead of printing help).
//
// The error contract matches [Run]: nil on success, a user-space error
// when check finds violations, a config error for invalid flags/specs.
// Unknown flags are rejected (exit 2) rather than silently ignored.
func RunCLI(spec dsl.SpecDef, args []string) error {
	if spec.Builder() == nil {
		return fmt.Errorf("spec is empty — ensure Spec() was called")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return app.RunCLI(ctx, decoder.NewGoDecoder(spec.Builder()), args)
}

// MustRunCLI runs the delegated CLI surface and exits the process with a
// conventional exit code (see [ExitCode]). This is what a scaffolded
// `.go-arch-lint/main.go` calls; errors that fail before the renderer
// runs (unknown command/flag) are printed to stderr so the exit is never
// silent.
func MustRunCLI(spec dsl.SpecDef, args []string) {
	if err := RunCLI(spec, args); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
		os.Exit(ExitCode(err))
	}
	os.Exit(ExitCodeOK)
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
// Errors that fail before the renderer runs (invalid option combinations)
// are printed to stderr so the exit is never silent.
func MustRun(spec dsl.SpecDef, opts ...Option) {
	if err := Run(spec, opts...); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
		os.Exit(ExitCode(err))
	}
	os.Exit(ExitCodeOK)
}

// MustRunV2 is [MustRun] for a v2 build (see [RunV2]).
func MustRunV2(build *v2.Build, opts ...Option) {
	if err := RunV2(build, opts...); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
		os.Exit(ExitCode(err))
	}
	os.Exit(ExitCodeOK)
}
