package dsl

import (
	"path/filepath"
	"runtime"
)

type SpecDef struct {
	builder *SpecBuilder
}

func (s SpecDef) Builder() *SpecBuilder {
	return s.builder
}

func Spec(fn func()) SpecDef {
	builder := newSpecBuilder()
	current = contextStack{spec: builder}

	fn()

	current = contextStack{}
	return SpecDef{builder: builder}
}

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
