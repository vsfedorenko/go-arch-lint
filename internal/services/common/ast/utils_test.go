package ast

import (
	"go/token"
	"testing"

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
	if !ref.Valid {
		t.Fatal("position with a line must be valid")
	}
	if ref.File != tcFile || ref.Line != 12 || ref.Column != 7 {
		t.Fatalf("got %+v, want file=a.go line=12 col=7", ref)
	}
	if ref.LineFrom != 12 || ref.LineTo != 12 {
		t.Fatalf("single-line reference must span one line: %+v", ref)
	}
}

func TestPositionFromToken_ZeroLineInvalid(t *testing.T) {
	// token.Position zero value (or NoPos) resolves to Line 0: the
	// reference must be marked invalid so downstream rendering skips it
	// instead of pointing at "line 0".
	ref := PositionFromToken(token.Position{Filename: tcFile})
	if ref.Valid {
		t.Fatal("zero-line position must produce an invalid reference")
	}
	if ref.Line != 0 {
		t.Fatalf("invalid reference must keep line 0, got %d", ref.Line)
	}
}

func TestPositionFromToken_ZeroLineStillCarriesFile(t *testing.T) {
	ref := PositionFromToken(token.Position{Filename: "b.go"})
	// The file survives: diagnostics can still attribute the problem
	// even without a line.
	if ref.File != "b.go" {
		t.Fatalf("file must survive invalidation, got %q", ref.File)
	}
	if _, ok := any(ref).(domain.Reference); !ok {
		t.Fatal("returns a domain.Reference")
	}
}
