package archlint

import (
	"context"
	"fmt"
	"os"

	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/app"
	"github.com/vsfedorenko/go-arch-lint/internal/models"
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

	return app.RunCheck(ctx, spec, models.CheckOptions{
		ProjectPath: cfg.projectPath,
		MaxWarnings: cfg.maxWarnings,
		UseColors:   cfg.useColors,
		Format:      cfg.format,
	})
}

func MustRun(spec dsl.SpecDef, opts ...Option) {
	if err := Run(spec, opts...); err != nil {
		os.Exit(1)
	}
}
