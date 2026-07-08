package dsl

import (
	"fmt"
	"path/filepath"
	"runtime"
)

// SpecDef is a populated spec definition returned by Spec.
// Multiple SpecDefs can be merged and passed to archlint.Run / archlint.MustRun.
type SpecDef struct {
	builder *SpecBuilder
}

// Spec is the entry point for the DSL. It creates a new SpecBuilder,
// sets it as the current context, executes fn, and returns a SpecDef.
//
// Multiple Spec calls are supported — pass all returned SpecDefs to
// archlint.Run / archlint.MustRun and they are merged into one spec.
// Scalar fields (Version, Workdir, Allow*) use first-set-wins; slices
// accumulate; maps (Components, Vendors, Deps) use last-write-per-key.
func Spec(fn func()) SpecDef {
	globalBuilder = newSpecBuilder()
	current = contextStack{spec: globalBuilder}

	fn()

	current = contextStack{}
	return SpecDef{builder: globalBuilder}
}

// SetSpec injects a builder into the global slot so that FlushSpec
// (called by the decoder) can retrieve it. Used by archlint.Run after
// merging multiple SpecDefs.
func SetSpec(builder *SpecBuilder) {
	globalBuilder = builder
	current = contextStack{}
}

// MergeSpecs combines multiple SpecDefs into a single SpecBuilder.
// Returns nil if no specs are provided.
func MergeSpecs(specs ...SpecDef) *SpecBuilder {
	if len(specs) == 0 {
		return nil
	}

	merged := newSpecBuilder()
	mergedAny := false
	for _, sd := range specs {
		b := sd.builder
		if b == nil {
			continue
		}
		mergedAny = true

		if !merged.Version.Reference.Valid && b.Version.Reference.Valid {
			merged.Version = b.Version
		}
		if !merged.Workdir.Reference.Valid && b.Workdir.Reference.Valid {
			merged.Workdir = b.Workdir
		}
		if !merged.Allow.DepOnAnyVendor.Reference.Valid && b.Allow.DepOnAnyVendor.Reference.Valid {
			merged.Allow.DepOnAnyVendor = b.Allow.DepOnAnyVendor
		}
		if !merged.Allow.DeepScan.Reference.Valid && b.Allow.DeepScan.Reference.Valid {
			merged.Allow.DeepScan = b.Allow.DeepScan
		}
		if !merged.Allow.IgnoreNotFoundComponents.Reference.Valid && b.Allow.IgnoreNotFoundComponents.Reference.Valid {
			merged.Allow.IgnoreNotFoundComponents = b.Allow.IgnoreNotFoundComponents
		}

		merged.Exclude = append(merged.Exclude, b.Exclude...)
		merged.ExcludeFiles = append(merged.ExcludeFiles, b.ExcludeFiles...)
		merged.CommonComponents = append(merged.CommonComponents, b.CommonComponents...)
		merged.CommonVendors = append(merged.CommonVendors, b.CommonVendors...)

		for k, v := range b.Components {
			merged.Components[k] = v
		}
		for k, v := range b.Vendors {
			merged.Vendors[k] = v
		}
		for k, v := range b.Deps {
			merged.Deps[k] = v
		}
	}

	if !mergedAny {
		return nil
	}
	return merged
}

// FlushSpec returns the populated SpecBuilder and resets the global state.
func FlushSpec() (*SpecBuilder, error) {
	if globalBuilder == nil {
		return nil, fmt.Errorf("Spec() was not called — ensure your arch.go contains 'var _ = Spec(func() { ... })'")
	}

	builder := globalBuilder
	globalBuilder = nil
	current = contextStack{}

	return builder, nil
}

func callerRef(skip int) (file string, line int) {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "", 0
	}
	return filepath.Base(file), line
}
