package decoder

import (
	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
	"github.com/vsfedorenko/go-arch-lint/internal/services/spec"
)

// GoSpecDocument implements spec.Document by wrapping a dsl.SpecBuilder.
// It replaces the YAML-based ArchV3 struct.
type GoSpecDocument struct {
	builder *dsl.SpecBuilder
}

func NewGoSpecDocument(builder *dsl.SpecBuilder) *GoSpecDocument {
	return &GoSpecDocument{builder: builder}
}

func (d *GoSpecDocument) Version() domain.Referable[int] {
	return d.builder.Version
}

func (d *GoSpecDocument) WorkingDirectory() domain.Referable[string] {
	workdir := d.builder.Workdir
	if workdir.Value == "" {
		return domain.NewEmptyReferable("./")
	}
	return workdir
}

func (d *GoSpecDocument) Options() spec.Options {
	return &goOptions{allow: d.builder.Allow}
}

func (d *GoSpecDocument) ExcludedDirectories() []domain.Referable[string] {
	return d.builder.Exclude
}

func (d *GoSpecDocument) ExcludedFilesRegExp() []domain.Referable[string] {
	return d.builder.ExcludeFiles
}

func (d *GoSpecDocument) Vendors() spec.Vendors {
	result := make(spec.Vendors, len(d.builder.Vendors))
	for name, vendor := range d.builder.Vendors {
		result[name] = domain.NewReferable(spec.Vendor(goVendor{paths: vendor.ImportPaths}), vendor.Reference)
	}
	return result
}

func (d *GoSpecDocument) CommonVendors() []domain.Referable[string] {
	return d.builder.CommonVendors
}

func (d *GoSpecDocument) Components() spec.Components {
	result := make(spec.Components, len(d.builder.Components))
	for name, comp := range d.builder.Components {
		result[name] = domain.NewReferable(spec.Component(goComponent{paths: comp.RelativePaths}), comp.Reference)
	}
	return result
}

func (d *GoSpecDocument) CommonComponents() []domain.Referable[string] {
	return d.builder.CommonComponents
}

func (d *GoSpecDocument) Tiers() []spec.Tier {
	result := make([]spec.Tier, len(d.builder.Tiers))
	for i, tier := range d.builder.Tiers {
		result[i] = spec.Tier{
			Name:       tier.Name,
			Components: append([]string(nil), tier.Components...),
			Reference:  tier.Reference,
		}
	}
	return result
}

func (d *GoSpecDocument) InterfacePlacement() spec.InterfacePlacement {
	if d.builder.InterfacePlacement == nil {
		return nil
	}
	return &goInterfacePlacement{entry: d.builder.InterfacePlacement}
}

func (d *GoSpecDocument) Naming() spec.Naming {
	if d.builder.Naming == nil {
		return nil
	}
	return &goNaming{entry: d.builder.Naming}
}

func (d *GoSpecDocument) Dependencies() spec.Dependencies {
	result := make(spec.Dependencies, len(d.builder.Deps))
	for name, dep := range d.builder.Deps {
		result[name] = domain.NewReferable(spec.DependencyRule(&goDependencyRule{dep: dep}), dep.Reference)
	}
	return result
}

// --- goInterfacePlacement implements spec.InterfacePlacement ---

type goInterfacePlacement struct {
	entry *dsl.InterfacePlacementEntry
}

func (ip *goInterfacePlacement) MustLiveWithConsumer() bool {
	return ip.entry.MustLiveWithConsumer
}

// --- goNaming implements spec.Naming ---

type goNaming struct {
	entry *dsl.NamingEntry
}

func (n *goNaming) ForbiddenPackages() []domain.Referable[string] {
	return n.entry.ForbiddenPackages
}

// --- goOptions implements spec.Options ---

type goOptions struct {
	allow dsl.AllowEntry
}

func (o *goOptions) IsDependOnAnyVendor() domain.Referable[bool] {
	return o.allow.DepOnAnyVendor
}

func (o *goOptions) DeepScan() domain.Referable[bool] {
	if o.allow.DeepScan.Reference.Valid {
		return o.allow.DeepScan
	}
	// default true since v3+
	return domain.NewEmptyReferable(true)
}

func (o *goOptions) IgnoreNotFoundComponents() domain.Referable[bool] {
	if o.allow.IgnoreNotFoundComponents.Reference.Valid {
		return o.allow.IgnoreNotFoundComponents
	}
	return domain.NewEmptyReferable(false)
}

// --- goVendor implements spec.Vendor ---

type goVendor struct {
	paths []string
}

func (v goVendor) ImportPaths() []models.Glob {
	result := make([]models.Glob, 0, len(v.paths))
	for _, p := range v.paths {
		result = append(result, models.Glob(p))
	}
	return result
}

// --- goComponent implements spec.Component ---

type goComponent struct {
	paths []string
}

func (c goComponent) RelativePaths() []models.Glob {
	result := make([]models.Glob, 0, len(c.paths))
	for _, p := range c.paths {
		result = append(result, models.Glob(p))
	}
	return result
}

// --- goDependencyRule implements spec.DependencyRule ---

type goDependencyRule struct {
	dep dsl.DepEntry
}

func (r *goDependencyRule) MayDependOn() []domain.Referable[string] {
	return r.dep.MayDependOn
}

func (r *goDependencyRule) CanUse() []domain.Referable[string] {
	return r.dep.CanUse
}

func (r *goDependencyRule) AnyProjectDeps() domain.Referable[bool] {
	return r.dep.AnyProjectDeps
}

func (r *goDependencyRule) AnyVendorDeps() domain.Referable[bool] {
	return r.dep.AnyVendorDeps
}

func (r *goDependencyRule) DeepScan() domain.Referable[bool] {
	if r.dep.DeepScan.Reference.Valid {
		return r.dep.DeepScan
	}
	return domain.NewEmptyReferable(false)
}

// --- GoDecoder implements archDecoder ---

// GoDecoder implements the archDecoder interface by reading from the
// in-memory dsl.SpecBuilder (populated by the user's arch.go).
// The archFile parameter is ignored — the spec is already in-process.
type GoDecoder struct {
	builder *dsl.SpecBuilder
}

func NewGoDecoder(builder *dsl.SpecBuilder) *GoDecoder {
	return &GoDecoder{builder: builder}
}

func (d *GoDecoder) Decode(_ string) (spec.Document, []arch.Notice, error) {
	document := NewGoSpecDocument(d.builder)
	return document, []arch.Notice{}, nil
}
