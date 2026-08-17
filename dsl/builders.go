package dsl

import (
	"fmt"

	"github.com/vsfedorenko/go-arch-lint/internal/models/common"
)

// Version sets the DSL schema version (always 1 for v2.0).
func Version(v int) {
	file, line := callerRef(1)
	current.spec.Version = common.NewReferable(v, common.NewReferenceSingleLine(file, line, 0))
}

// Workdir sets the relative working directory for analysis.
func Workdir(path string) {
	file, line := callerRef(1)
	current.spec.Workdir = common.NewReferable(path, common.NewReferenceSingleLine(file, line, 0))
}

// Allow defines global rules. Call DepOnAnyVendor/DeepScan/IgnoreNotFoundComponents inside fn.
func Allow(fn func()) {
	current.inAllow = true
	fn()
	current.inAllow = false
}

// DepOnAnyVendor sets whether any project code may import any vendor lib.
func DepOnAnyVendor(b bool) {
	file, line := callerRef(1)
	current.spec.Allow.DepOnAnyVendor = common.NewReferable(b, common.NewReferenceSingleLine(file, line, 0))
}

// DeepScan enables/disables advanced AST analysis.
// Inside Allow(): sets global default. Inside Deps(): overrides per-component.
func DeepScan(b bool) {
	file, line := callerRef(1)
	ref := common.NewReferable(b, common.NewReferenceSingleLine(file, line, 0))

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
	file, line := callerRef(1)
	current.spec.Allow.IgnoreNotFoundComponents = common.NewReferable(b, common.NewReferenceSingleLine(file, line, 0))
}

// Exclude adds directories to exclude from analysis.
func Exclude(paths ...string) {
	file, line := callerRef(1)
	for _, p := range paths {
		current.spec.Exclude = append(current.spec.Exclude, common.NewReferable(p, common.NewReferenceSingleLine(file, line, 0)))
	}
}

// ExcludeFiles adds regex patterns to exclude matching files.
func ExcludeFiles(patterns ...string) {
	file, line := callerRef(1)
	for _, p := range patterns {
		current.spec.ExcludeFiles = append(current.spec.ExcludeFiles, common.NewReferable(p, common.NewReferenceSingleLine(file, line, 0)))
	}
}

// Component defines a named component mapping to one or more package paths.
func Component(name string, paths ...string) {
	if name == "" {
		panic(fmt.Errorf("Component name cannot be empty"))
	}
	file, line := callerRef(1)
	current.spec.Components[name] = ComponentEntry{
		RelativePaths: paths,
		Reference:     common.NewReferenceSingleLine(file, line, 0),
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
			Reference: common.NewReferenceSingleLine(file, line, 0),
		})
	}
}

// Tier assigns components to a layer declared by Tiers(). Must be called
// after the matching Tiers() call. A component may belong to one tier
// only; components not mentioned in any tier are unchecked by tier rules.
func Tier(name string, components ...string) {
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

// Vendor defines a named vendor mapping to one or more import paths.
func Vendor(name string, importPaths ...string) {
	if name == "" {
		panic(fmt.Errorf("Vendor name cannot be empty"))
	}
	file, line := callerRef(1)
	current.spec.Vendors[name] = VendorEntry{
		ImportPaths: importPaths,
		Reference:   common.NewReferenceSingleLine(file, line, 0),
	}
}

// CommonComponents marks components as importable by any project package.
func CommonComponents(names ...string) {
	file, line := callerRef(1)
	for _, n := range names {
		current.spec.CommonComponents = append(current.spec.CommonComponents, common.NewReferable(n, common.NewReferenceSingleLine(file, line, 0)))
	}
}

// CommonVendors marks vendors as importable by any project package.
func CommonVendors(names ...string) {
	file, line := callerRef(1)
	for _, n := range names {
		current.spec.CommonVendors = append(current.spec.CommonVendors, common.NewReferable(n, common.NewReferenceSingleLine(file, line, 0)))
	}
}

// Deps defines dependency rules for a component. Call MayDependOn/CanUse/etc inside fn.
func Deps(component string, fn func()) {
	if component == "" {
		panic(fmt.Errorf("Deps component name cannot be empty"))
	}

	file, line := callerRef(1)
	dep := DepEntry{
		Reference: common.NewReferenceSingleLine(file, line, 0),
	}

	prevDep := current.dep
	current.dep = &dep
	fn()
	current.dep = prevDep

	current.spec.Deps[component] = dep
}

// MayDependOn lists components that this component may import.
func MayDependOn(components ...string) {
	file, line := callerRef(1)
	if current.dep == nil {
		panic(fmt.Errorf("MayDependOn called outside of Deps() callback"))
	}
	for _, c := range components {
		current.dep.MayDependOn = append(current.dep.MayDependOn, common.NewReferable(c, common.NewReferenceSingleLine(file, line, 0)))
	}
}

// CanUse lists vendors that this component may import.
func CanUse(vendors ...string) {
	file, line := callerRef(1)
	if current.dep == nil {
		panic(fmt.Errorf("CanUse called outside of Deps() callback"))
	}
	for _, v := range vendors {
		current.dep.CanUse = append(current.dep.CanUse, common.NewReferable(v, common.NewReferenceSingleLine(file, line, 0)))
	}
}

// AnyProjectDeps allows this component to import any other project package.
func AnyProjectDeps(b bool) {
	file, line := callerRef(1)
	if current.dep == nil {
		panic(fmt.Errorf("AnyProjectDeps called outside of Deps() callback"))
	}
	current.dep.AnyProjectDeps = common.NewReferable(b, common.NewReferenceSingleLine(file, line, 0))
}

// AnyVendorDeps allows this component to import any vendor package.
func AnyVendorDeps(b bool) {
	file, line := callerRef(1)
	if current.dep == nil {
		panic(fmt.Errorf("AnyVendorDeps called outside of Deps() callback"))
	}
	current.dep.AnyVendorDeps = common.NewReferable(b, common.NewReferenceSingleLine(file, line, 0))
}
