package ast

import (
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

// Fixture file name (goconst).
const tcFile = "a.go"

// PositionFromToken converts a token.Position into a domain reference,
// marking zero-line positions (unresolvable/no-position tokens) invalid.

func TestPositionFromToken_Valid(t *testing.T) {
	ref := PositionFromToken(token.Position{
		Filename: tcFile,
		Line:     12,
		Column:   7,
	})
	require.True(t, ref.Valid, "position with a line must be valid")
	assert.Equal(t, tcFile, ref.File, "file")
	assert.Equal(t, 12, ref.Line, "line")
	assert.Equal(t, 7, ref.Column, "column")
	assert.Equal(t, 12, ref.LineFrom, "single-line reference must span one line")
	assert.Equal(t, 12, ref.LineTo, "single-line reference must span one line")
}

func TestPositionFromToken_ZeroLineInvalid(t *testing.T) {
	// token.Position zero value (or NoPos) resolves to Line 0: the
	// reference must be marked invalid so downstream rendering skips it
	// instead of pointing at "line 0".
	ref := PositionFromToken(token.Position{Filename: tcFile})
	require.False(t, ref.Valid, "zero-line position must produce an invalid reference")
	assert.Equal(t, 0, ref.Line, "invalid reference must keep line 0")
}

func TestPositionFromToken_ZeroLineStillCarriesFile(t *testing.T) {
	ref := PositionFromToken(token.Position{Filename: "b.go"})
	// The file survives: diagnostics can still attribute the problem
	// even without a line.
	assert.Equal(t, "b.go", ref.File, "file must survive invalidation")
	_, ok := any(ref).(domain.Reference)
	assert.True(t, ok, "returns a domain.Reference")
}
