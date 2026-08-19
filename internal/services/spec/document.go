package spec

import (
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

// spec layout:
//   decoder   - decode different config file formats (v1 ... latest) into one single Document interface
//   validator - will validate Document interface (check integrity between config fields)
//   assembler - will assemble arch.Spec from validated Document interface
//
// all other operations (business logic) code will use arch.Spec object for reading config values.

type (
	// ComponentName is abstraction useful for mapping real packages to one Component.
	ComponentName = string

	// VendorName is abstraction useful for mapping real vendor packages to one Vendor.
	VendorName = string

	Vendors      = map[VendorName]domain.Referable[Vendor]
	Components   = map[ComponentName]domain.Referable[Component]
	Dependencies = map[ComponentName]domain.Referable[DependencyRule]

	Document interface {
		// Version of spec (scheme of document)
		Version() domain.Referable[int]

		// WorkingDirectory relative to root, prepend this to all path's from spec
		WorkingDirectory() domain.Referable[string]

		// Options is global spec options
		Options() Options

		// ExcludedDirectories from analyze, each contain relative directory name
		// List of directories
		// examples:
		// 	- internal/test
		//	- vendor
		//	- .idea
		ExcludedDirectories() []domain.Referable[string]

		// ExcludedFilesRegExp from analyze, each project file will be matched with this regexp rules
		// List of regexp's
		// examples:
		// 	- "^.*_test\\.go$"
		ExcludedFilesRegExp() []domain.Referable[string]

		// Vendors (map)
		Vendors() Vendors

		// CommonVendors is list of Vendors that can be imported to any project package
		CommonVendors() []domain.Referable[string]

		// Components (map)
		Components() Components

		// CommonComponents is List of Components that can be imported to any project package
		CommonComponents() []domain.Referable[string]

		// Dependencies map between Components and DependencyRule`s
		Dependencies() Dependencies

		// Tiers is the ordered list of architectural layers (may be empty).
		// Index 0 is the highest layer; dependencies may only flow downward.
		Tiers() []Tier

		// Naming holds packaging-convention rules (nil when absent).
		Naming() Naming

		// Visibility holds export-visibility rules (nil when absent).
		Visibility() Visibility

		// InterfacePlacement holds interface-location rules (nil when absent).
		InterfacePlacement() InterfacePlacement
	}

	// InterfacePlacement is the interface-location conventions section of
	// a Document. See Document.InterfacePlacement.
	InterfacePlacement interface {
		// MustLiveWithConsumer requires single-consumer interfaces to be
		// declared in the consuming component.
		MustLiveWithConsumer() bool
	}

	// Naming is the packaging-conventions section of a Document: banned
	// package names. See Document.Naming.
	Naming interface {
		// ForbiddenPackages is the list of banned package names.
		ForbiddenPackages() []domain.Referable[string]
	}

	// Visibility is the export-visibility section of a Document: which
	// components may consume which component's API. See Document.Visibility.
	Visibility interface {
		// Rules is the list of visibility restrictions.
		Rules() []VisibilityRule
	}

	// VisibilityRule is one declared restriction: exports of Component may
	// only be consumed by Allowed (plus the component itself).
	VisibilityRule struct {
		Component string
		Allowed   []string
		Reference domain.Reference
	}

	// Tier is one declared architectural layer: a name plus its member
	// components. Part of the Document surface (see Document.Tiers).
	Tier struct {
		Name       string
		Components []string
		Reference  domain.Reference
	}

	Options interface {
		// IsDependOnAnyVendor allows all project code depend on any third party vendor lib
		// analyze will not check imports with not local namespace's
		IsDependOnAnyVendor() domain.Referable[bool]

		// DeepScan turn on usage of advanced AST linter
		// this is default behavior since v3+ configs
		DeepScan() domain.Referable[bool]

		// IgnoreNotFoundComponents skips components that are not found by their glob
		// disabled by default
		IgnoreNotFoundComponents() domain.Referable[bool]
	}

	Vendor interface {
		// ImportPaths is list of full import vendor qualified path
		// example:
		// 	- golang.org/x/mod/modfile
		// 	- example.com/*/libs/**
		ImportPaths() []models.Glob
	}

	Component interface {
		// RelativePaths can contain glob's
		// example:
		// 	- internal/service/*/models/**
		// 	- /
		// 	- tests/**
		RelativePaths() []models.Glob
	}

	DependencyRule interface {
		// MayDependOn is list of Component names, that can be imported to described component
		MayDependOn() []domain.Referable[string]

		// CanUse is list of Vendor names, that can be imported to described component
		CanUse() []domain.Referable[string]

		// AnyProjectDeps allow component to import any other local namespace packages
		AnyProjectDeps() domain.Referable[bool]

		// AnyVendorDeps allow component to import any other vendor namespace packages
		AnyVendorDeps() domain.Referable[bool]

		// DeepScan overrides deepScan global option
		DeepScan() domain.Referable[bool]
	}
)
