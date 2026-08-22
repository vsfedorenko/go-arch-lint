// Package core imports fmt (stdlib) and domain — a clean spec without
// any Vendor still passes: stdlib imports never need a Vendor rule.
package core

import (
	"fmt"

	"v2sem/domain"
)

// Order ties back to the domain.
type Order struct{ Owner domain.User }

// String uses the stdlib import so it is a real dependency.
func (o Order) String() string { return fmt.Sprintf("order of user %d", o.Owner.ID) }
