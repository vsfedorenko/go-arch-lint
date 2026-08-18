package dsl

import (
	"fmt"

	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

// Version sets the DSL schema version (always 1 for v2.0).
func Version(v int) {
	requireSpec()

	file, line := callerRef(1)
	current.spec.Version = domain.NewReferable(v, domain.NewReferenceSingleLine(file, line, 0))
}

// Workdir sets the relative working directory for analysis.
func Workdir(path string) {
	requireSpec()

	file, line := callerRef(1)
	current.spec.Workdir = domain.NewReferable(path, domain.NewReferenceSingleLine(file, line, 0))
}

// Allow defines global rules. Call DepOnAnyVendor/DeepScan/IgnoreNotFoundComponents inside fn.
func Allow(fn func()) {
	requireSpec()

	current.inAllow = true
	fn()
	current.inAllow = false
}

// DepOnAnyVendor sets whether any project code may import any vendor lib.
func DepOnAnyVendor(b bool) {
	requireSpec()

	file, line := callerRef(1)
	current.spec.Allow.DepOnAnyVendor = domain.NewReferable(b, domain.NewReferenceSingleLine(file, line, 0))
}

// DeepScan enables/disables advanced AST analysis.
// Inside Allow(): sets global default. Inside Deps(): overrides per-component.
func DeepScan(b bool) {
	requireSpec()

	file, line := callerRef(1)
	ref := domain.NewReferable(b, domain.NewReferenceSingleLine(file, line, 0))

	if current.inAllow {
		current.spec.Allow.DeepScan = ref
		return
	}

	if current.dep != nil {
		current.dep.DeepScan = ref
	}
}

// IgnoreNotFoundComponents skips components not found by their glob.
func IgnoreNotFoundComponents(b bool) {
	requireSpec()

	file, line := callerRef(1)
	current.spec.Allow.IgnoreNotFoundComponents = domain.NewReferable(b, domain.NewReferenceSingleLine(file, line, 0))
}

// Exclude adds directories to exclude from analysis.
func Exclude(paths ...string) {
	requireSpec()

	file, line := callerRef(1)
	for _, p := range paths {
		current.spec.Exclude = append(current.spec.Exclude, domain.NewReferable(p, domain.NewReferenceSingleLine(file, line, 0)))
	}
}

// ExcludeFiles adds regex patterns to exclude matching files.
func ExcludeFiles(patterns ...string) {
	requireSpec()

	file, line := callerRef(1)
	for _, p := range patterns {
		current.spec.ExcludeFiles = append(current.spec.ExcludeFiles, domain.NewReferable(p, domain.NewReferenceSingleLine(file, line, 0)))
	}
}

// Component defines a named component mapping to one or more package paths.
func Component(name string, paths ...string) {
	requireSpec()

	if name == "" {
		panic(fmt.Errorf("Component name cannot be empty"))
	}
	file, line := callerRef(1)
	current.spec.Components[name] = ComponentEntry{
		RelativePaths: paths,
		Reference:     domain.NewReferenceSingleLine(file, line, 0),
	}
}

// Tiers declares an ordered list of architectural layers. Dependencies may
// only flow downward: a component in Tiers("domain", "app", "infra")[0]
// ("domain") may not depend on anything in a lower tier, "app" may depend
// on "domain" but not vice versa, and so on. Tier rules are checked
// against the ACTUAL import graph, independent of mayDependOn permissions.
//
//	Tiers("domain", "app", "infra")
//	Tier("domain", "user", "order")
//	Tier("app", "handler")
//	Tier("infra", "postgres", "redis")
func Tiers(names ...string) {
	requireSpec()

	if len(names) == 0 {
		panic(fmt.Errorf("Tiers requires at least one layer name"))
	}

	file, line := callerRef(1)

	seen := map[string]struct{}{}
	for _, name := range names {
		if name == "" {
			panic(fmt.Errorf("Tier name cannot be empty"))
		}
		if _, dup := seen[name]; dup {
			panic(fmt.Errorf("Tier '%s' declared twice in Tiers()", name))
		}
		seen[name] = struct{}{}
	}

	for _, existing := range current.spec.Tiers {
		if _, dup := seen[existing.Name]; dup {
			panic(fmt.Errorf("Tier '%s' declared twice", existing.Name))
		}
	}

	for _, name := range names {
		current.spec.Tiers = append(current.spec.Tiers, TierEntry{
			Name:      name,
			Reference: domain.NewReferenceSingleLine(file, line, 0),
		})
	}
}

// Tier assigns components to a layer declared by Tiers(). Must be called
// after the matching Tiers() call. A component may belong to one tier
// only; components not mentioned in any tier are unchecked by tier rules.
// Visibility declares export-visibility rules: which components may
// consume which component's exported API. Rules are collected.
//
//	Visibility(func() {
//	    VisibleTo("services") // internal: nobody else may import services
//	    VisibleTo("models", "services", "container")
//	})
func Visibility(fn func()) {
	requireSpec()

	file, line := callerRef(1)

	entry := current.spec.Visibility
	if entry == nil {
		entry = &VisibilityEntry{Reference: domain.NewReferenceSingleLine(file, line, 0)}
		current.spec.Visibility = entry
	}

	saved := current.inVisibility
	current.inVisibility = true
	defer func() { current.inVisibility = saved }()

	fn()
}

// VisibleTo restricts who may consume Component's exported API.
// Allowed components (plus the component itself) are the only legal
// consumers. With no Allowed arguments the component is fully internal.
func VisibleTo(component string, allowed ...string) {
	requireSpec()

	file, line := callerRef(1)

	if component == "" {
		panic(fmt.Errorf("VisibleTo component name cannot be empty"))
	}

	if current.spec.Visibility == nil {
		panic(fmt.Errorf("VisibleTo must be called inside Visibility(func(){...})"))
	}

	for _, a := range allowed {
		if a == "" {
			panic(fmt.Errorf("VisibleTo allowed component name cannot be empty"))
		}
		if a == component {
			panic(fmt.Errorf("VisibleTo allowed component '%s' is the component itself — self is always implicit", component))
		}
	}

	current.spec.Visibility.Rules = append(current.spec.Visibility.Rules, VisibilityRule{
		Component: component,
		Allowed:   allowed,
		Reference: domain.NewReferenceSingleLine(file, line, 0),
	})
}

func Tier(name string, components ...string) {
	requireSpec()

	if name == "" {
		panic(fmt.Errorf("Tier name cannot be empty"))
	}
	if len(components) == 0 {
		panic(fmt.Errorf("Tier '%s' requires at least one component", name))
	}

	tierIndex := -1
	for i, tier := range current.spec.Tiers {
		if tier.Name == name {
			tierIndex = i
			break
		}
	}
	if tierIndex == -1 {
		panic(fmt.Errorf("Tier '%s' is not declared — call Tiers(..., %q, ...) first", name, name))
	}

	for _, comp := range components {
		if _, known := current.spec.Components[comp]; !known {
			panic(fmt.Errorf("Tier '%s' references unknown component '%s' — declare it with Component() first", name, comp))
		}

		for _, tier := range current.spec.Tiers {
			for _, existing := range tier.Components {
				if existing == comp {
					panic(fmt.Errorf("Component '%s' is already assigned to tier '%s'", comp, tier.Name))
				}
			}
		}
	}

	current.spec.Tiers[tierIndex].Components = append(
		current.spec.Tiers[tierIndex].Components, components...,
	)
}

// Naming declares packaging-name conventions. Inside fn, use
// ForbiddenPackages to ban non-descriptive package names:
//
//	Naming(func() {
//		ForbiddenPackages("utils", "helpers", "common")
//	})
//
// Rules apply to every scanned project package, regardless of component.
func Naming(fn func()) {
	requireSpec()

	file, line := callerRef(1)

	entry := current.spec.Naming
	if entry == nil {
		entry = &NamingEntry{
			Reference: domain.NewReferenceSingleLine(file, line, 0),
		}
		current.spec.Naming = entry
	}

	previous := current.naming
	current.naming = entry
	defer func() { current.naming = previous }()

	fn()
}

// ForbiddenPackages bans the given package names anywhere in the project
// (must be called inside Naming()).
func ForbiddenPackages(names ...string) {
	requireSpec()

	if len(names) == 0 {
		panic(fmt.Errorf("ForbiddenPackages requires at least one name"))
	}
	if current.naming == nil {
		panic(fmt.Errorf("ForbiddenPackages must be called inside Naming()"))
	}

	file, line := callerRef(1)
	for _, name := range names {
		if name == "" {
			panic(fmt.Errorf("ForbiddenPackages name cannot be empty"))
		}
		current.naming.ForbiddenPackages = append(
			current.naming.ForbiddenPackages,
			domain.NewReferable(name, domain.NewReferenceSingleLine(file, line, 0)),
		)
	}
}

// Interfaces declares interface-placement conventions. Inside fn, use
// MustLiveWithConsumer to require that an interface used by exactly one
// other component is declared in that component (hexagonal ports live
// with the consumer, not the implementation):
//
//	Interfaces(func() {
//		MustLiveWithConsumer()
//	})
func Interfaces(fn func()) {
	requireSpec()

	file, line := callerRef(1)

	if current.spec.InterfacePlacement == nil {
		current.spec.InterfacePlacement = &InterfacePlacementEntry{
			Reference: domain.NewReferenceSingleLine(file, line, 0),
		}
	}

	previous := current.interfaces
	current.interfaces = current.spec.InterfacePlacement
	defer func() { current.interfaces = previous }()

	fn()
}

// MustLiveWithConsumer bans interfaces declared next to an implementation
// when a single other component is their only consumer (must be called
// inside Interfaces()).
func MustLiveWithConsumer() {
	requireSpec()

	if current.interfaces == nil {
		panic(fmt.Errorf("MustLiveWithConsumer must be called inside Interfaces()"))
	}

	file, line := callerRef(1)
	current.interfaces.MustLiveWithConsumer = true
	current.interfaces.Reference = domain.NewReferenceSingleLine(file, line, 0)
}

// Vendor defines a named vendor mapping to one or more import paths.
func Vendor(name string, importPaths ...string) {
	requireSpec()

	if name == "" {
		panic(fmt.Errorf("Vendor name cannot be empty"))
	}
	file, line := callerRef(1)
	current.spec.Vendors[name] = VendorEntry{
		ImportPaths: importPaths,
		Reference:   domain.NewReferenceSingleLine(file, line, 0),
	}
}

// CommonComponents marks components as importable by any project package.
func CommonComponents(names ...string) {
	requireSpec()

	file, line := callerRef(1)
	for _, n := range names {
		current.spec.CommonComponents = append(current.spec.CommonComponents, domain.NewReferable(n, domain.NewReferenceSingleLine(file, line, 0)))
	}
}

// CommonVendors marks vendors as importable by any project package.
func CommonVendors(names ...string) {
	requireSpec()

	file, line := callerRef(1)
	for _, n := range names {
		current.spec.CommonVendors = append(current.spec.CommonVendors, domain.NewReferable(n, domain.NewReferenceSingleLine(file, line, 0)))
	}
}

// Deps defines dependency rules for a component. Call MayDependOn/CanUse/etc inside fn.
func Deps(component string, fn func()) {
	requireSpec()

	if component == "" {
		panic(fmt.Errorf("Deps component name cannot be empty"))
	}

	file, line := callerRef(1)
	dep := DepEntry{
		Reference: domain.NewReferenceSingleLine(file, line, 0),
	}

	prevDep := current.dep
	current.dep = &dep
	fn()
	current.dep = prevDep

	current.spec.Deps[component] = dep
}

// MayDependOn lists components that this component may import.
func MayDependOn(components ...string) {
	requireSpec()

	file, line := callerRef(1)
	if current.dep == nil {
		panic(fmt.Errorf("MayDependOn called outside of Deps() callback"))
	}
	for _, c := range components {
		current.dep.MayDependOn = append(current.dep.MayDependOn, domain.NewReferable(c, domain.NewReferenceSingleLine(file, line, 0)))
	}
}

// CanUse lists vendors that this component may import.
func CanUse(vendors ...string) {
	requireSpec()

	file, line := callerRef(1)
	if current.dep == nil {
		panic(fmt.Errorf("CanUse called outside of Deps() callback"))
	}
	for _, v := range vendors {
		current.dep.CanUse = append(current.dep.CanUse, domain.NewReferable(v, domain.NewReferenceSingleLine(file, line, 0)))
	}
}

// AnyProjectDeps allows this component to import any other project package.
func AnyProjectDeps(b bool) {
	requireSpec()

	file, line := callerRef(1)
	if current.dep == nil {
		panic(fmt.Errorf("AnyProjectDeps called outside of Deps() callback"))
	}
	current.dep.AnyProjectDeps = domain.NewReferable(b, domain.NewReferenceSingleLine(file, line, 0))
}

// AnyVendorDeps allows this component to import any vendor package.
func AnyVendorDeps(b bool) {
	requireSpec()

	file, line := callerRef(1)
	if current.dep == nil {
		panic(fmt.Errorf("AnyVendorDeps called outside of Deps() callback"))
	}
	current.dep.AnyVendorDeps = domain.NewReferable(b, domain.NewReferenceSingleLine(file, line, 0))
}
