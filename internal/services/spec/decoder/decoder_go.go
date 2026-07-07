package decoder

import (
	"github.com/fe3dback/go-arch-lint/dsl"
	"github.com/fe3dback/go-arch-lint/internal/models"
	"github.com/fe3dback/go-arch-lint/internal/models/common"
	"github.com/fe3dback/go-arch-lint/internal/services/spec"
)

// GoSpecDocument implements spec.Document by wrapping a dsl.SpecBuilder.
// It replaces the YAML-based ArchV3 struct.
type GoSpecDocument struct {
	builder *dsl.SpecBuilder
}

func NewGoSpecDocument(builder *dsl.SpecBuilder) *GoSpecDocument {
	return &GoSpecDocument{builder: builder}
}

func (d *GoSpecDocument) Version() common.Referable[int] {
	return d.builder.Version
}

func (d *GoSpecDocument) WorkingDirectory() common.Referable[string] {
	workdir := d.builder.Workdir
	if workdir.Value == "" {
		return common.NewEmptyReferable("./")
	}
	return workdir
}

func (d *GoSpecDocument) Options() spec.Options {
	return &goOptions{allow: d.builder.Allow}
}

func (d *GoSpecDocument) ExcludedDirectories() []common.Referable[string] {
	return d.builder.Exclude
}

func (d *GoSpecDocument) ExcludedFilesRegExp() []common.Referable[string] {
	return d.builder.ExcludeFiles
}

func (d *GoSpecDocument) Vendors() spec.Vendors {
	result := make(spec.Vendors, len(d.builder.Vendors))
	for name, vendor := range d.builder.Vendors {
		result[name] = common.NewReferable(spec.Vendor(goVendor{paths: vendor.ImportPaths}), vendor.Reference)
	}
	return result
}

func (d *GoSpecDocument) CommonVendors() []common.Referable[string] {
	return d.builder.CommonVendors
}

func (d *GoSpecDocument) Components() spec.Components {
	result := make(spec.Components, len(d.builder.Components))
	for name, comp := range d.builder.Components {
		result[name] = common.NewReferable(spec.Component(goComponent{paths: comp.RelativePaths}), comp.Reference)
	}
	return result
}

func (d *GoSpecDocument) CommonComponents() []common.Referable[string] {
	return d.builder.CommonComponents
}

func (d *GoSpecDocument) Dependencies() spec.Dependencies {
	result := make(spec.Dependencies, len(d.builder.Deps))
	for name, dep := range d.builder.Deps {
		result[name] = common.NewReferable(spec.DependencyRule(&goDependencyRule{dep: dep}), dep.Reference)
	}
	return result
}

// --- goOptions implements spec.Options ---

type goOptions struct {
	allow dsl.AllowEntry
}

func (o *goOptions) IsDependOnAnyVendor() common.Referable[bool] {
	return o.allow.DepOnAnyVendor
}

func (o *goOptions) DeepScan() common.Referable[bool] {
	if o.allow.DeepScan.Reference.Valid {
		return o.allow.DeepScan
	}
	// default true since v3+
	return common.NewEmptyReferable(true)
}

func (o *goOptions) IgnoreNotFoundComponents() common.Referable[bool] {
	if o.allow.IgnoreNotFoundComponents.Reference.Valid {
		return o.allow.IgnoreNotFoundComponents
	}
	return common.NewEmptyReferable(false)
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

func (r *goDependencyRule) MayDependOn() []common.Referable[string] {
	return r.dep.MayDependOn
}

func (r *goDependencyRule) CanUse() []common.Referable[string] {
	return r.dep.CanUse
}

func (r *goDependencyRule) AnyProjectDeps() common.Referable[bool] {
	return r.dep.AnyProjectDeps
}

func (r *goDependencyRule) AnyVendorDeps() common.Referable[bool] {
	return r.dep.AnyVendorDeps
}

func (r *goDependencyRule) DeepScan() common.Referable[bool] {
	if r.dep.DeepScan.Reference.Valid {
		return r.dep.DeepScan
	}
	return common.NewEmptyReferable(false)
}
