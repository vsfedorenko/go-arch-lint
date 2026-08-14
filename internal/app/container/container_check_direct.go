package container

import (
	"context"

	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

func (c *Container) RunCheck(ctx context.Context, spec dsl.SpecDef, opts models.CheckOptions) error {
	c.specBuilder = spec.Builder()

	format := opts.Format
	if format == models.FormatDefault || format == "" {
		format = models.FormatText
	}

	c.flags = models.FlagsRoot{
		UseColors:  opts.UseColors,
		OutputType: models.OutputTypeASCII,
		Format:     format,
	}

	in := models.CmdCheckIn{
		ProjectPath: opts.ProjectPath,
		ArchFile:    models.DefaultArchFileName,
		MaxWarnings: opts.MaxWarnings,
	}

	model, err := c.commandCheckOperation().Behave(ctx, in)
	return c.ProvideRenderer().RenderModel(model, err)
}
