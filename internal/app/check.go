package app

import (
	"context"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/app/container"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
)

// RunCheck executes a check driven by an in-process spec. The decoder (which
// wraps the dsl.SpecBuilder built by the user's arch.go) is supplied by the
// caller — the public archlint package entry point performs the dsl→services
// conversion, keeping this layer (app → container/models only, per the
// .go-arch-lint dependency rules) free of dsl and services imports.
func RunCheck(ctx context.Context, spec container.SpecDecoder, opts models.CheckOptions) error {
	di := newContainer()
	err := di.RunCheck(ctx, spec, opts)
	reportSystemError(err)
	return err
}
