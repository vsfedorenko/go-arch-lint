package app

import (
	"fmt"
	"os"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/app/container"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
)

// newContainer wires the DI container with build-time constants.
// Single source of truth for container construction across all entry points.
func newContainer() *container.Container {
	return container.NewContainer(Version, BuildTime, CommitHash)
}

// reportSystemError writes non-user-space errors to stderr.
// UserSpaceError and ConfigError are silent: they are already explained in
// structured (ascii/json) output.
func reportSystemError(err error) {
	if err == nil || models.IsUserSpaceError(err) || models.IsConfigError(err) {
		return
	}
	fmt.Fprintln(os.Stderr, err)
}
