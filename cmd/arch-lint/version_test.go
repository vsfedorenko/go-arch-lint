package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/app"
)

// The `version` command must report the module build info when the binary
// was installed via `go install ...@version` (no ldflags): app defaults say
// "dev", and debug.ReadBuildInfo carries the real module version. The
// goreleaser path (ldflags set) is unaffected: a non-dev version short-
// circuits straight to the ldflags values.
func TestRun_VersionReportsBuildInfo(t *testing.T) {
	old := os.Args
	os.Args = []string{"arch-lint", "version"}
	defer func() { os.Args = old }()

	var buf strings.Builder
	restore := swapStdout(&buf)

	code := run()
	restore()

	assert.Equal(t, 0, code, "version must exit 0")
	line := buf.String()
	assert.NotContains(t, line, "launcher dev ",
		"version output fell back to the raw ldflags default %q (app.Version=%q)", line, app.Version)
	assert.Contains(t, line, "go-arch-lint launcher ", "unexpected version line: %q", line)
}

// swapStdout redirects os.Stdout into w and returns a restore func that
// flushes the captured output back into w before restoring the original.
func swapStdout(w *strings.Builder) func() {
	old := os.Stdout
	r, pipeW, _ := os.Pipe()
	os.Stdout = pipeW

	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(w, r)
		close(done)
	}()

	return func() {
		_ = pipeW.Close()
		<-done
		_ = r.Close()
		os.Stdout = old
	}
}
