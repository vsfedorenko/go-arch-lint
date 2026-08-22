package validator

import (
	"path/filepath"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/services/spec"
)

// fakeDoc is an in-memory spec.Document for validator tests: the v1 DSL
// builder it used to ride on is gone (v3), and the validator contract is
// the Document surface itself, so the tests build that surface directly.
type fakeDoc struct {
	version    int
	workdir    string
	components spec.Components
	deps       spec.Dependencies
	vendors    spec.Vendors
	commonCom  []domain.Referable[string]
	commonVen  []domain.Referable[string]
	exclDirs   []domain.Referable[string]
	exclFiles  []domain.Referable[string]
	depOnVen   bool
	deepScan   bool
	ignoreNF   bool
	tiers      []spec.Tier
}

func (d *fakeDoc) Version() domain.Referable[int] {
	return domain.NewReferable(d.version, domain.NewEmptyReference())
}

func (d *fakeDoc) WorkingDirectory() domain.Referable[string] {
	return domain.NewReferable(d.workdir, domain.NewEmptyReference())
}

func (d *fakeDoc) Options() spec.Options {
	return fakeOptions{depOnVen: d.depOnVen, deepScan: d.deepScan, ignoreNF: d.ignoreNF}
}

func (d *fakeDoc) ExcludedDirectories() []domain.Referable[string] { return d.exclDirs }
func (d *fakeDoc) ExcludedFilesRegExp() []domain.Referable[string] { return d.exclFiles }
func (d *fakeDoc) Vendors() spec.Vendors                           { return d.vendors }
func (d *fakeDoc) CommonVendors() []domain.Referable[string]       { return d.commonVen }
func (d *fakeDoc) Components() spec.Components                     { return d.components }
func (d *fakeDoc) CommonComponents() []domain.Referable[string]    { return d.commonCom }
func (d *fakeDoc) Dependencies() spec.Dependencies                 { return d.deps }
func (d *fakeDoc) Tiers() []spec.Tier                              { return d.tiers }
func (d *fakeDoc) Naming() spec.Naming                             { return nil }
func (d *fakeDoc) Visibility() spec.Visibility                     { return nil }
func (d *fakeDoc) InterfacePlacement() spec.InterfacePlacement     { return nil }

type fakeOptions struct {
	depOnVen bool
	deepScan bool
	ignoreNF bool
}

func (o fakeOptions) IsDependOnAnyVendor() domain.Referable[bool] {
	return domain.NewReferable(o.depOnVen, domain.NewEmptyReference())
}

func (o fakeOptions) DeepScan() domain.Referable[bool] {
	return domain.NewReferable(o.deepScan, domain.NewEmptyReference())
}

func (o fakeOptions) IgnoreNotFoundComponents() domain.Referable[bool] {
	return domain.NewReferable(o.ignoreNF, domain.NewEmptyReference())
}

type fakeComponent struct{ paths []string }

func (c fakeComponent) RelativePaths() []models.Glob {
	out := make([]models.Glob, 0, len(c.paths))
	for _, p := range c.paths {
		out = append(out, models.Glob(p))
	}
	return out
}

type fakeVendor struct{ imports []string }

func (v fakeVendor) ImportPaths() []models.Glob {
	out := make([]models.Glob, 0, len(v.imports))
	for _, p := range v.imports {
		out = append(out, models.Glob(p))
	}
	return out
}

type fakeDep struct {
	mayDependOn []string
	canUse      []string
	anyProject  bool
	anyVendor   bool
	deepScan    bool
}

func (r fakeDep) MayDependOn() []domain.Referable[string] { return referStrings(r.mayDependOn) }
func (r fakeDep) CanUse() []domain.Referable[string]      { return referStrings(r.canUse) }
func (r fakeDep) AnyProjectDeps() domain.Referable[bool] {
	return domain.NewReferable(r.anyProject, domain.NewEmptyReference())
}

func (r fakeDep) AnyVendorDeps() domain.Referable[bool] {
	return domain.NewReferable(r.anyVendor, domain.NewEmptyReference())
}

func (r fakeDep) DeepScan() domain.Referable[bool] {
	return domain.NewReferable(r.deepScan, domain.NewEmptyReference())
}

func referStrings(in []string) []domain.Referable[string] {
	out := make([]domain.Referable[string], 0, len(in))
	for _, s := range in {
		out = append(out, domain.NewReferable(s, domain.NewEmptyReference()))
	}
	return out
}

func referableComponent(c fakeComponent, r domain.Reference) domain.Referable[spec.Component] {
	return domain.Referable[spec.Component]{Value: c, Reference: r}
}

func referableVendor(v fakeVendor, r domain.Reference) domain.Referable[spec.Vendor] {
	return domain.Referable[spec.Vendor]{Value: v, Reference: r}
}

func referableDep(d fakeDep, r domain.Reference) domain.Referable[spec.DependencyRule] {
	return domain.Referable[spec.DependencyRule]{Value: d, Reference: r}
}

// fakePathResolver resolves glob paths against the real testdata tree.
type fakePathResolver struct{}

func (fakePathResolver) Resolve(absPath string) ([]string, error) {
	return filepath.Glob(absPath)
}
