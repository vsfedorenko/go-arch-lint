package dsl

import "fmt"

var current contextStack

type contextStack struct {
	spec         *SpecBuilder
	dep          *DepEntry
	inAllow      bool
	inVisibility bool
	naming       *NamingEntry
	interfaces   *InterfacePlacementEntry
}

// requireSpec panics with an actionable message when no spec is being
// built. Without the guard, a builder called after a nested Spec() (which
// resets the context on exit) dereferences a nil builder and crashes with
// a raw segmentation fault instead of a diagnosable error.
func requireSpec() {
	if current.spec == nil {
		panic(fmt.Errorf("this DSL function must be called inside Spec(func(){...}) — " +
			"the context was reset (a nested Spec() finished); check for a Spec() call inside your spec closure"))
	}
}
