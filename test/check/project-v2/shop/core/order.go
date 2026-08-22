// Package core uses domain (allowed by Use) — clean edge.
package core

import "v2fixture/shop/domain"

// Order ties back to the domain.
type Order struct{ Owner domain.User }
