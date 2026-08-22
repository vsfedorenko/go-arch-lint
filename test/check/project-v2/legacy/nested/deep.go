// Package nested is part of the legacy/** subtree component.
package nested

import "v2fixture/shop/domain"

// D references domain from deep inside the subtree.
type D struct{ U domain.User }
