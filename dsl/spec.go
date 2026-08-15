package dsl

import (
	"path/filepath"
	"runtime"
)

// SpecDef is a built architecture spec, produced by [Spec] or [MergeSpecs].
// It is an opaque handle: pass it to archlint.Run (or the CLI) to lint.
type SpecDef struct {
	builder *SpecBuilder
}

// Builder exposes the underlying [SpecBuilder]. It is used internally by the
// check pipeline; user code rarely needs it.
func (s SpecDef) Builder() *SpecBuilder {
	return s.builder
}

// Spec defines the architecture: components, allowed dependencies, vendors
// and scan options. It must be called exactly once per configuration, with
// all DSL builder calls (Version, Workdir, Component, Deps, ...) inside fn.
//
// The canonical usage is a package-level variable in `.go-arch-lint/arch.go`,
// so the spec is built at init time and picked up by the scaffolded main:
//
//	var _ = Spec(func() {
//		Version(1)
//		Workdir("internal")
//		Component("handler", "handlers/*")
//		Deps("handler", func() { MayDependOn("service") })
//	})
func Spec(fn func()) SpecDef {
	builder := newSpecBuilder()
	current = contextStack{spec: builder}

	fn()

	current = contextStack{}
	return SpecDef{builder: builder}
}

// MergeSpecs combines several specs into one: the first set value wins for
// scalar fields (Version, Workdir, Allow flags), while components, vendors,
// deps, excludes and common lists are concatenated. Later specs override
// earlier ones on component/vendor/dep name collisions. Useful for keeping
// per-module specs in separate files and linting them as a single project.
func MergeSpecs(specs ...SpecDef) SpecDef {
	if len(specs) == 0 {
		return SpecDef{}
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
		return SpecDef{}
	}
	return SpecDef{builder: merged}
}

func callerRef(skip int) (file string, line int) {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "", 0
	}
	return filepath.Base(file), line
}
