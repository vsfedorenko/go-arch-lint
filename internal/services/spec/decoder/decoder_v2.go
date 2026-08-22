package decoder

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	v2 "github.com/vsfedorenko/go-arch-lint/v2/dsl/v2"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/services/spec"
)

// V2SpecDocument implements spec.Document on top of the v2 Path-based
// DSL build (dsl/v2.Build). The mapping is intentionally thin:
//
//	Path("a/b")        -> one Component whose RelativePaths is ["a/b"]
//	                    ("a/b/**" subtrees keep the /** suffix in the glob)
//	Vendor(n, import)  -> one Vendor entry
//	Use(from, to...)   -> DependencyRule{MayDependOn: paths, CanUse: vendors}
//
// Everything the v2 language does not have (tiers, naming, visibility,
// interface placement, global allow flags) reports its zero value; the
// checker then enforces exactly the v2 semantics: nothing may use
// anything unless a Use said so.
type V2SpecDocument struct {
	build *v2.Build
}

// NewV2SpecDocument wraps a finished v2 build.
func NewV2SpecDocument(b *v2.Build) *V2SpecDocument {
	return &V2SpecDocument{build: b}
}

// FSVerify checks every declared path against the real filesystem under
// root and returns one notice per problem: missing directories, and
// /**-subtrees without a single Go file. Unknown references inside Use
// rules cannot occur (the DSL validates IDs at build time); a path that
// exists but was never declared surfaces through the checker's
// not-matched warnings instead.
func (d *V2SpecDocument) FSVerify(root string) []arch.Notice {
	notices := []arch.Notice{}

	names := append([]string(nil), d.build.Order...)
	sort.Strings(names)

	for _, name := range names {
		e := d.build.Paths[name]
		abs := filepath.Join(root, filepath.FromSlash(e.Full))

		st, err := os.Stat(abs)
		switch {
		case err != nil || !st.IsDir():
			if e.Subtree {
				notices = append(notices, arch.Notice{
					Notice: fmt.Errorf("Path(%q): directory %q does not exist", e.Full+"/**", e.Full),
					Ref:    ref(e.File, e.Line),
				})
				continue
			}
			notices = append(notices, arch.Notice{
				Notice: fmt.Errorf("Path(%q): directory %q does not exist — did you mean %q?", e.Full, e.Full, v2.Suggest(e.Full, siblingDirs(root, e.Full))),
				Ref:    ref(e.File, e.Line),
			})
		case e.Subtree && !hasGoFiles(abs):
			notices = append(notices, arch.Notice{
				Notice: fmt.Errorf("Path(%q): no Go files under %q", e.Full+"/**", e.Full),
				Ref:    ref(e.File, e.Line),
			})
		}
	}
	return notices
}

func ref(file string, line int) domain.Reference {
	return domain.NewReferenceSingleLine(file, line, 0)
}

// siblingDirs lists the immediate subdirectories of the parent of p —
// the candidates for a did-you-mean suggestion.
func siblingDirs(root, p string) []string {
	parent := filepath.Dir(filepath.FromSlash(p))
	entries, err := os.ReadDir(filepath.Join(root, parent))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, filepath.ToSlash(filepath.Join(parent, e.Name())))
		}
	}
	return out
}

func hasGoFiles(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // unreadable subtrees are skipped, not fatal
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".go") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// Decode implements the container.SpecDecoder port: the build is
// already parsed Go, decoding is a no-op wrap.
func (d *V2SpecDocument) Decode(_ string) (spec.Document, []arch.Notice, error) {
	return d, []arch.Notice{}, nil
}

// --- spec.Document ---

func (d *V2SpecDocument) Version() domain.Referable[int] {
	return domain.NewReferable(1, domain.NewEmptyReference())
}

func (d *V2SpecDocument) WorkingDirectory() domain.Referable[string] {
	// The v2 language has no workdir: paths are module-root relative.
	return domain.NewReferable("./", domain.NewEmptyReference())
}

func (d *V2SpecDocument) Options() spec.Options {
	return v2Options{}
}

func (d *V2SpecDocument) ExcludedDirectories() []domain.Referable[string] {
	// .go-arch-lint and vendor are excluded by the pipeline's defaults,
	// not by the document.
	return nil
}

func (d *V2SpecDocument) ExcludedFilesRegExp() []domain.Referable[string] {
	return nil
}

func (d *V2SpecDocument) Vendors() spec.Vendors {
	out := spec.Vendors{}
	for name, v := range d.build.Vendors {
		out[name] = domain.NewReferable(spec.Vendor(v2Vendor{importPath: v.ImportPath}), ref(v.File, v.Line))
	}
	return out
}

func (d *V2SpecDocument) CommonVendors() []domain.Referable[string] { return nil }

func (d *V2SpecDocument) Components() spec.Components {
	out := spec.Components{}
	for _, name := range d.build.Order {
		e := d.build.Paths[name]
		glob := e.Full
		if e.Subtree {
			glob += "/**"
		}
		out[name] = domain.NewReferable(
			spec.Component(v2Component{paths: []models.Glob{models.Glob(glob)}}),
			ref(e.File, e.Line),
		)
	}
	return out
}

func (d *V2SpecDocument) CommonComponents() []domain.Referable[string] { return nil }

func (d *V2SpecDocument) Dependencies() spec.Dependencies {
	out := spec.Dependencies{}
	for from, u := range d.build.Uses {
		rule := v2Dependency{
			mayDependOn: u.Paths,
			canUse:      u.Vends,
		}
		out[from] = domain.NewReferable(spec.DependencyRule(rule), ref(u.File, u.Line))
	}
	return out
}

func (d *V2SpecDocument) Tiers() []spec.Tier          { return nil }
func (d *V2SpecDocument) Naming() spec.Naming         { return nil }
func (d *V2SpecDocument) Visibility() spec.Visibility { return nil }

func (d *V2SpecDocument) InterfacePlacement() spec.InterfacePlacement { return nil }

// --- leaf implementations ---

type v2Options struct{}

func (v2Options) IsDependOnAnyVendor() domain.Referable[bool] {
	// Without CanUse rules every vendor import is a violation — the v2
	// default. Vendors named in Use(...) get explicit CanUse entries.
	return domain.NewReferable(false, domain.NewEmptyReference())
}

func (v2Options) DeepScan() domain.Referable[bool] {
	return domain.NewReferable(false, domain.NewEmptyReference())
}

func (v2Options) IgnoreNotFoundComponents() domain.Referable[bool] {
	// v2 paths are verified against the FS at build time; a declared path
	// with no code is an error, not something to ignore.
	return domain.NewReferable(false, domain.NewEmptyReference())
}

type v2Vendor struct{ importPath string }

func (v v2Vendor) ImportPaths() []models.Glob {
	return []models.Glob{models.Glob(v.importPath)}
}

type v2Component struct{ paths []models.Glob }

func (c v2Component) RelativePaths() []models.Glob { return c.paths }

type v2Dependency struct {
	mayDependOn []string
	canUse      []string
}

func (r v2Dependency) MayDependOn() []domain.Referable[string] {
	return referStrings(r.mayDependOn)
}

func (r v2Dependency) CanUse() []domain.Referable[string] {
	return referStrings(r.canUse)
}

func (r v2Dependency) AnyProjectDeps() domain.Referable[bool] {
	return domain.NewReferable(false, domain.NewEmptyReference())
}

func (r v2Dependency) AnyVendorDeps() domain.Referable[bool] {
	return domain.NewReferable(false, domain.NewEmptyReference())
}

func (r v2Dependency) DeepScan() domain.Referable[bool] {
	return domain.NewReferable(false, domain.NewEmptyReference())
}

func referStrings(list []string) []domain.Referable[string] {
	out := make([]domain.Referable[string], 0, len(list))
	for _, s := range list {
		out = append(out, domain.NewReferable(s, domain.NewEmptyReference()))
	}
	return out
}
