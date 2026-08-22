package container

import (
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
)

type Container struct {
	version    string
	buildTime  string
	commitHash string

	flags           models.FlagsRoot
	externalDecoder SpecDecoder
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
