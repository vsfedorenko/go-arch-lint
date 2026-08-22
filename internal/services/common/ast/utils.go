package ast

import (
	"go/token"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

func PositionFromToken(pos token.Position) domain.Reference {
	ref := domain.NewReferenceSingleLine(
		pos.Filename,
		pos.Line,
		pos.Column,
	)

	if pos.Line == 0 {
		ref.Valid = false
		ref.Line = 0

		return ref
	}

	return ref
}
