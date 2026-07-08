package app

import (
	"errors"
	"fmt"
	"os"

	"github.com/vsfedorenko/go-arch-lint/internal/app/container"
	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

// newContainer wires the DI container with build-time constants.
// Single source of truth for container construction across all entry points.
func newContainer() *container.Container {
	return container.NewContainer(Version, BuildTime, CommitHash)
}

// reportSystemError writes non-user-space errors to stderr.
// UserSpaceError is silent: it is already explained in structured (ascii/json) output.
func reportSystemError(err error) {
	if err == nil || errors.Is(err, models.UserSpaceError{}) {
		return
	}
	fmt.Fprintln(os.Stderr, err)
}
