package archlint

import (
	"context"
	"fmt"
	"os"

	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/app"
	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/services/spec/decoder"
)

type Option func(*config)

type config struct {
	projectPath string
	maxWarnings int
	useColors   bool
	format      models.Format
}

func WithProjectPath(path string) Option {
	return func(c *config) { c.projectPath = path }
}

func WithMaxWarnings(n int) Option {
	return func(c *config) { c.maxWarnings = n }
}

func WithColors(b bool) Option {
	return func(c *config) { c.useColors = b }
}

// WithFormat sets the output format for check results.
// Use models.FormatJSON for a flat JSON array of violations,
// or models.FormatText (the default) for human-readable ASCII.
func WithFormat(format models.Format) Option {
	return func(c *config) { c.format = format }
}

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
