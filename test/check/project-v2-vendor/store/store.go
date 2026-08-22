// Package store imports an external package (pgx): without a Vendor
// rule in Use it is a violation; with one it is clean.
package store

import (
	"github.com/jackc/pgx/v5"

	"v2vend/domain"
)

// Store keeps users.
type Store struct{ U domain.User }

// Err re-exports the pgx reference so the import is used.
var Err = pgx.ErrNoRows
