package app

import (
	"context"

	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

func RunCheck(ctx context.Context, spec dsl.SpecDef, opts models.CheckOptions) error {
	di := newContainer()
	err := di.RunCheck(ctx, spec, opts)
	reportSystemError(err)
	return err
}
