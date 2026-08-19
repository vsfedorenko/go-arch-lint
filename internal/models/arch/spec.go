package arch

import (
	"regexp"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

type (
	Spec struct {
		RootDirectory       domain.Referable[string]
		WorkingDirectory    domain.Referable[string]
		ModuleName          domain.Referable[string]
		Allow               Allow
		Components          []Component
		Exclude             []domain.Referable[models.ResolvedPath]
		ExcludeFilesMatcher []domain.Referable[*regexp.Regexp]
		Integrity           Integrity
		Tiers               []Tier
		Naming              *Naming

		// InterfacePlacement holds interface-location rules (nil = off).
		InterfacePlacement *InterfacePlacement

		// Visibility holds export-visibility rules (nil = off).
		Visibility *Visibility
	}

	// InterfacePlacement holds interface-location convention rules.
	InterfacePlacement struct {
		MustLiveWithConsumer bool
	}

	// Visibility holds export-visibility convention rules: which
	// components may consume which component's exported API.
	Visibility struct {
		Rules []VisibilityRule
	}

	// VisibilityRule restricts consumers of Component's exported API to
	// Allowed (plus the component itself).
	VisibilityRule struct {
		Component string
		Allowed   []string
		Reference domain.Reference
	}

	// Naming holds packaging-name convention rules: banned package names
	// checked against every scanned project package.
	Naming struct {
		ForbiddenPackages []domain.Referable[string]
	}

	// Tier is one architectural layer: a name plus member component names.
	// Index 0 in Spec.Tiers is the highest layer — dependencies may only
	// flow downward (higher -> lower), never upward.
	Tier struct {
		Name       string
		Components []string
		Reference  domain.Reference
	}

	Allow struct {
		DepOnAnyVendor           domain.Referable[bool]
		DeepScan                 domain.Referable[bool]
		IgnoreNotFoundComponents domain.Referable[bool]
	}

	Component struct {
		Name                  domain.Referable[string]
		DeepScan              domain.Referable[bool]
		ResolvedPaths         []domain.Referable[models.ResolvedPath]
		AllowedProjectImports []domain.Referable[models.ResolvedPath]
		AllowedVendorGlobs    []domain.Referable[models.Glob]
		MayDependOn           []domain.Referable[string]
		CanUse                []domain.Referable[string]
		SpecialFlags          SpecialFlags
	}

	SpecialFlags struct {
		AllowAllProjectDeps domain.Referable[bool]
		AllowAllVendorDeps  domain.Referable[bool]
	}

	Integrity struct {
		DocumentNotices []Notice
		Suggestions     []Notice
	}

	Notice struct {
		Notice error
		Ref    domain.Reference
	}
)
