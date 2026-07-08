package container

import (
	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

type Container struct {
	version    string
	buildTime  string
	commitHash string

	flags       models.FlagsRoot
	specBuilder *dsl.SpecBuilder
}

func NewContainer(
	version string,
	buildTime string,
	commitHash string,
) *Container {
	return &Container{
		version:    version,
		buildTime:  buildTime,
		commitHash: commitHash,
	}
}
