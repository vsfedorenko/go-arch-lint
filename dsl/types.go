package dsl

import "github.com/vsfedorenko/go-arch-lint/internal/models/domain"

// SpecBuilder is the in-memory representation of the user's arch config,
// populated by DSL functions. It replaces the YAML decoder's ArchV3 struct.
type SpecBuilder struct {
	Version          domain.Referable[int]
	Workdir          domain.Referable[string]
	Allow            AllowEntry
	Exclude          []domain.Referable[string]
	ExcludeFiles     []domain.Referable[string]
	Vendors          map[string]VendorEntry
	CommonVendors    []domain.Referable[string]
	Components       map[string]ComponentEntry
	CommonComponents []domain.Referable[string]
	Deps             map[string]DepEntry

	// Tiers holds the ordered layer list (see Tiers()/Tier() builders).
	// Index 0 is the highest layer; dependencies may only flow downward.
	Tiers []TierEntry

	// Naming holds packaging-convention rules (see Naming()).
	Naming *NamingEntry

	// InterfacePlacement holds interface-location rules (see Interfaces()).
	InterfacePlacement *InterfacePlacementEntry
}

// InterfacePlacementEntry holds interface-placement convention rules.
type InterfacePlacementEntry struct {
	// MustLiveWithConsumer: an interface used by exactly one other
	// component must be declared in that component (hexagonal ports) —
	// not next to its implementation.
	MustLiveWithConsumer bool
	Reference            domain.Reference
}

// NamingEntry holds packaging-naming convention rules.
type NamingEntry struct {
	ForbiddenPackages []domain.Referable[string]
	Reference         domain.Reference
}

// TierEntry is one ordered layer: a name plus the components in it.
type TierEntry struct {
	Name       string
	Components []string
	Reference  domain.Reference
}

// AllowEntry holds global allow rules.
type AllowEntry struct {
	DepOnAnyVendor           domain.Referable[bool]
	DeepScan                 domain.Referable[bool]
	IgnoreNotFoundComponents domain.Referable[bool]
}

// VendorEntry holds a named vendor definition.
type VendorEntry struct {
	ImportPaths []string
	Reference   domain.Reference
}

// ComponentEntry holds a named component definition.
type ComponentEntry struct {
	RelativePaths []string
	Reference     domain.Reference
}

// DepEntry holds dependency rules for a component.
type DepEntry struct {
	MayDependOn    []domain.Referable[string]
	CanUse         []domain.Referable[string]
	AnyProjectDeps domain.Referable[bool]
	AnyVendorDeps  domain.Referable[bool]
	DeepScan       domain.Referable[bool]
	Reference      domain.Reference
}

func newSpecBuilder() *SpecBuilder {
	return &SpecBuilder{
		Vendors:    make(map[string]VendorEntry),
		Components: make(map[string]ComponentEntry),
		Deps:       make(map[string]DepEntry),
	}
}
