package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
)

// operation_test.go pins the version operation's sourcing rules: ldflags
// values win when set; "dev" falls back to compiled module metadata (which
// in `go test` binaries carries (devel)); the supported-schema range is a
// stable contract.

// Explicit ldflags values are surfaced verbatim.
func TestBehave_LdFlagsValues(t *testing.T) {
	op := NewOperation("v2.4.1", "2026-08-21T12:00:00Z", "deadbeef")

	out, err := op.Behave()
	require.NoError(t, err)
	assert.Equal(t, "v2.4.1", out.LinterVersion)
	assert.Equal(t, "2026-08-21T12:00:00Z", out.BuildTime)
	assert.Equal(t, "deadbeef", out.CommitHash)
	assert.Equal(t, "1 .. 1", out.GoArchFileSupported, "supported schema range is part of the contract")
}

// UnknownVersion ("dev") triggers the compiled-meta fallback; under
// `go test` the binary metadata is (devel), which still resolves.
func TestBehave_DevFallsBackToCompiledMeta(t *testing.T) {
	op := NewOperation(models.UnknownVersion, "", "")

	out, err := op.Behave()
	require.NoError(t, err, "(devel) build info must resolve in test binaries")
	assert.NotEmpty(t, out.LinterVersion)
}
