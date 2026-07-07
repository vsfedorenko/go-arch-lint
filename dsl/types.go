package dsl

import "github.com/fe3dback/go-arch-lint/internal/models/common"

// SpecBuilder is the in-memory representation of the user's arch config,
// populated by DSL functions. It replaces the YAML decoder's ArchV3 struct.
type SpecBuilder struct {
	Version          common.Referable[int]
	Workdir          common.Referable[string]
	Allow            AllowEntry
	Exclude          []common.Referable[string]
	ExcludeFiles     []common.Referable[string]
	Vendors          map[string]VendorEntry
	CommonVendors    []common.Referable[string]
	Components       map[string]ComponentEntry
	CommonComponents []common.Referable[string]
	Deps             map[string]DepEntry
}

// AllowEntry holds global allow rules.
type AllowEntry struct {
	DepOnAnyVendor           common.Referable[bool]
	DeepScan                 common.Referable[bool]
	IgnoreNotFoundComponents common.Referable[bool]
}

// VendorEntry holds a named vendor definition.
type VendorEntry struct {
	ImportPaths []string
	Reference   common.Reference
}

// ComponentEntry holds a named component definition.
type ComponentEntry struct {
	RelativePaths []string
	Reference     common.Reference
}

// DepEntry holds dependency rules for a component.
type DepEntry struct {
	MayDependOn    []common.Referable[string]
	CanUse         []common.Referable[string]
	AnyProjectDeps common.Referable[bool]
	AnyVendorDeps  common.Referable[bool]
	DeepScan       common.Referable[bool]
	Reference      common.Reference
}

func newSpecBuilder() *SpecBuilder {
	return &SpecBuilder{
		Vendors:    make(map[string]VendorEntry),
		Components: make(map[string]ComponentEntry),
		Deps:       make(map[string]DepEntry),
	}
}
