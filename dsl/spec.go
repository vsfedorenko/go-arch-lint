package dsl

import (
	"fmt"
	"path/filepath"
	"runtime"
)

// Spec is the entry point for the DSL. It creates a new SpecBuilder,
// sets it as the current context, and executes fn. DSL functions called
// inside fn populate the builder.
//
// Usage:
//
//	var _ = Spec(func() {
//	    Version(1)
//	    Workdir("internal")
//	    Component("main", "app")
//	})
func Spec(fn func()) {
	globalBuilder = newSpecBuilder()
	current = contextStack{spec: globalBuilder}

	fn()

	current = contextStack{}
}

// FlushSpec returns the populated SpecBuilder and resets the global state.
// It must be called after Spec(fn) has executed (typically at the start of
// archlint.RunCLI()).
func FlushSpec() (*SpecBuilder, error) {
	if globalBuilder == nil {
		return nil, fmt.Errorf("Spec() was not called — ensure your arch.go contains 'var _ = Spec(func() { ... })'")
	}

	builder := globalBuilder
	globalBuilder = nil
	current = contextStack{}

	return builder, nil
}

// callerRef returns the file (basename) and line of the DSL function call
// site (the user's arch.go). skip=1 means the immediate caller of the DSL
// function. The basename is returned so References stay stable and readable
// regardless of the machine building the config.
func callerRef(skip int) (file string, line int) {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "", 0
	}
	return filepath.Base(file), line
}
