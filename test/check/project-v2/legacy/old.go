// Package legacy is one /** subtree component: everything under it is a
// single unit, its imports of shop/domain are allowed by Use.
package legacy

import "v2fixture/shop/domain"

// L references domain.
type L struct{ U domain.User }
