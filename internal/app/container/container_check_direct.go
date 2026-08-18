package container

import (
	"context"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/services/spec"
)

// SpecDecoder is the port for an in-process arch spec source. The
// services-layer decoder.GoDecoder satisfies it; the concrete value is
// injected through RunCheck by the archlint package entry point, which is
// the single place allowed to touch both the public dsl API and internal
// services — keeping the app/container layer dsl-free (see
// .go-arch-lint/main.go dependency rules).
type SpecDecoder interface {
	Decode(archFile string) (spec.Document, []arch.Notice, error)
}

// RunCheck executes a check driven by an in-process spec (spec), applying
// the CLI-equivalent flags derived from opts.
func (c *Container) RunCheck(ctx context.Context, spec SpecDecoder, opts models.CheckOptions) error {
	c.externalDecoder = spec

	format := opts.Format
	if format == models.FormatDefault || format == "" {
		format = models.FormatText
	}

	// An unset output type means ascii (the human-readable default). The
	// public archlint package validates user-provided values; anything
	// arriving here empty falls back rather than being passed to the
	// renderer, which would otherwise panic on an unknown type.
	outputType := opts.OutputType
	if outputType == "" || outputType == models.OutputTypeDefault {
		outputType = models.OutputTypeASCII
	}

	c.flags = models.FlagsRoot{
		UseColors:         opts.UseColors,
		OutputType:        outputType,
		OutputJsonOneLine: opts.OutputJSONOneLine,
		Format:            format,
	}

	in := models.CmdCheckIn{
		ProjectPath:    opts.ProjectPath,
		ArchFile:       models.DefaultArchFileName,
		MaxWarnings:    opts.MaxWarnings,
		BaselinePath:   opts.BaselinePath,
		BaselineUpdate: opts.BaselineUpdate,
	}

	model, err := c.commandCheckOperation().Behave(ctx, in)
	return c.ProvideRenderer().RenderModel(model, err)
}
