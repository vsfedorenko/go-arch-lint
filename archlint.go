package archlint

import (
	"os"

	"github.com/fe3dback/go-arch-lint/internal/app"
)

// RunCLI executes the arch-lint CLI. It is the entry point called by
// generated main.go files in .go-arch-lint/ directories.
//
// Version/buildTime/commitHash are injected from the user's module
// (via the dsl package version or ldflags).
func RunCLI() int {
	return app.Execute()
}

// MustRunCLI is like RunCLI but calls os.Exit with the result code.
func MustRunCLI() {
	os.Exit(RunCLI())
}
