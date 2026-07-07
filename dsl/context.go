package dsl

// globalBuilder is the singleton SpecBuilder populated by DSL functions.
// It is set by Spec() and consumed by FlushSpec().
var globalBuilder *SpecBuilder

// contextStack tracks the current builder context for nested DSL calls
// (e.g., inside Deps() callback, the current DepEntry).
type contextStack struct {
	spec *SpecBuilder
	dep  *DepEntry
	// allow context is set inside Allow() callback
	inAllow bool
}

var current contextStack

func resetSpecBuilder() {
	globalBuilder = nil
	current = contextStack{}
}
