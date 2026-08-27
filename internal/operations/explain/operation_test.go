package explain

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

// operation_test.go pins the explain operation at the unit layer: std
// classification and per-component verdicts, project-import ownership,
// usage-site collection with the display cap, and error propagation from
// every port.

// fixtures ---------------------------------------------------------------

const (
	stdFixtureImport = "fmt"
	coreComponent    = "core"
	appComponent     = "app"
)

func explainFile(path, component string, imports ...models.ResolvedImport) models.FileHold {
	if component == "" {
		return models.FileHold{File: models.ProjectFile{Path: path, Imports: imports}}
	}
	return models.FileHold{
		File:        models.ProjectFile{Path: path, Imports: imports},
		ComponentID: &component,
	}
}

func explainPath(local, importPath string) domain.Referable[models.ResolvedPath] {
	return domain.NewReferable(models.ResolvedPath{LocalPath: local, ImportPath: importPath}, domain.NewEmptyReference())
}

// explainSpec declares two components: core (owning internal/core) and
// app (allowed to import it via AllowedProjectImports).
func explainSpec() arch.Spec {
	spec := arch.Spec{
		RootDirectory: domain.NewReferable("/project", domain.NewEmptyReference()),
		ModuleName:    domain.NewReferable("example.com/m", domain.NewEmptyReference()),
	}

	core := arch.Component{
		Name: domain.NewReferable("core", domain.NewEmptyReference()),
		ResolvedPaths: []domain.Referable[models.ResolvedPath]{
			explainPath("internal/core", "example.com/m/internal/core"),
		},
		// The assembler puts a component's own paths into its
		// AllowedProjectImports — a component may always import itself.
		AllowedProjectImports: []domain.Referable[models.ResolvedPath]{
			explainPath("internal/core", "example.com/m/internal/core"),
		},
	}
	app := arch.Component{
		Name: domain.NewReferable("app", domain.NewEmptyReference()),
		AllowedProjectImports: []domain.Referable[models.ResolvedPath]{
			explainPath("internal/core", "example.com/m/internal/core"),
		},
	}
	spec.Components = append(spec.Components, core, app)

	return spec
}

func stdPackages() map[string]struct{} {
	return map[string]struct{}{
		stdFixtureImport: {},
		"strings":        {},
	}
}

type explainWired struct {
	op    *Operation
	info  *mockprojectInfoAssembler
	specA *mockspecAssembler
	files *mockprojectFilesResolver
	spec  arch.Spec
	holds []models.FileHold
}

func wireExplain(t *testing.T, healthy ...bool) *explainWired {
	t.Helper()
	w := &explainWired{
		info:  newMockprojectInfoAssembler(t),
		specA: newMockspecAssembler(t),
		files: newMockprojectFilesResolver(t),
		spec:  explainSpec(),
		holds: []models.FileHold{
			explainFile("/project/cmd/main.go", "app",
				models.ResolvedImport{Name: "example.com/m/internal/core", ImportType: models.ImportTypeProject}),
		},
	}
	w.op = NewOperation(w.specA, w.info, w.files, stdPackages())

	if len(healthy) == 0 || healthy[0] {
		w.info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).Return(domain.Project{}, nil)
		w.specA.EXPECT().Assemble(mock.Anything).RunAndReturn(func(domain.Project) (arch.Spec, error) {
			return w.spec, nil
		})
		w.files.EXPECT().ProjectFiles(mock.Anything, mock.Anything).RunAndReturn(func(context.Context, arch.Spec) ([]models.FileHold, error) {
			return w.holds, nil
		})
	}

	return w
}

// tests -------------------------------------------------------------------

// A std import is always allowed for every component, classified "std",
// and finds its actual usage sites sorted by file then line.
func TestBehave_StdImport(t *testing.T) {
	w := wireExplain(t)
	w.holds = []models.FileHold{
		explainFile("/project/b/second.go", "app",
			models.ResolvedImport{Name: stdFixtureImport, Reference: domain.Reference{Line: 7}}),
		explainFile("/project/a/first.go", "core",
			models.ResolvedImport{Name: stdFixtureImport, Reference: domain.Reference{Line: 3}}),
	}

	out, err := w.op.Behave(context.Background(), models.CmdExplainIn{ImportPath: stdFixtureImport})
	require.NoError(t, err)

	want := models.CmdExplainOut{
		ModuleName: "example.com/m",
		ImportPath: stdFixtureImport,
		ImportType: "std",
		Verdicts: []models.CmdExplainVerdict{
			{Component: coreComponent, Allowed: true, Rule: "std library imports are always allowed"},
			{Component: appComponent, Allowed: true, Rule: "std library imports are always allowed"},
		},
		Usages: []models.CmdExplainUsage{
			{File: "/a/first.go", Line: 3, Component: coreComponent},
			{File: "/b/second.go", Line: 7, Component: appComponent},
		},
	}
	assert.Equal(t, want, out)
}

// A project import: the owning component is named, the component that
// Uses the owner is allowed, and the usage site carries the importing
// component with the root prefix trimmed.
func TestBehave_ProjectImport(t *testing.T) {
	w := wireExplain(t)

	out, err := w.op.Behave(context.Background(), models.CmdExplainIn{ImportPath: "example.com/m/internal/core"})
	require.NoError(t, err)

	assert.Equal(t, "project", out.ImportType)
	assert.Equal(t, "core", out.OwnerComponent)

	require.Len(t, out.Verdicts, 2)
	assert.Equal(t, models.CmdExplainVerdict{
		Component: "core",
		Allowed:   true,
		Rule:      `path "internal/core" is part of this component or of a component it Uses`,
	}, out.Verdicts[0])
	assert.Equal(t, models.CmdExplainVerdict{
		Component: "app",
		Allowed:   true,
		Rule:      `path "internal/core" is part of this component or of a component it Uses`,
	}, out.Verdicts[1])

	require.Len(t, out.Usages, 1)
	assert.Equal(t, models.CmdExplainUsage{File: "/cmd/main.go", Component: "app"}, out.Usages[0])
}

// A vendor import with no matching vendor rule is denied for every
// component, with a concrete fix naming the Vendor(...) declaration.
func TestBehave_VendorImportDenied(t *testing.T) {
	w := wireExplain(t)

	out, err := w.op.Behave(context.Background(), models.CmdExplainIn{ImportPath: "github.com/external/dep"})
	require.NoError(t, err)

	require.Len(t, out.Verdicts, 2)
	for _, verdict := range out.Verdicts {
		assert.False(t, verdict.Allowed, "component %s", verdict.Component)
		assert.Contains(t, verdict.Rule, "no matching vendor rule")
		assert.Contains(t, verdict.Fix, `vendor := Vendor("<name>", "github.com/external/dep")`)
	}
}

// The usage list is capped at maxUsagesShown; the rest is counted in
// OmittedUsages (display cap, semantics unchanged).
func TestBehave_UsageCap(t *testing.T) {
	w := wireExplain(t)
	holds := make([]models.FileHold, 0, maxUsagesShown+5)
	for i := range maxUsagesShown + 5 {
		holds = append(holds, explainFile("/project/f/file.go", "app",
			models.ResolvedImport{Name: stdFixtureImport, Reference: domain.Reference{Line: i + 1}}))
	}
	w.holds = holds

	out, err := w.op.Behave(context.Background(), models.CmdExplainIn{ImportPath: stdFixtureImport})
	require.NoError(t, err)

	require.Len(t, out.Usages, maxUsagesShown)
	assert.Equal(t, 5, out.OmittedUsages)
}

// A workspace member import classifies as project even though it is not
// a subpackage of the root module.
func TestBehave_WorkspaceProjectImport(t *testing.T) {
	w := wireExplain(t)
	w.spec.WorkspaceModules = []domain.WorkspaceModule{{Path: "example.com/sibling"}}
	w.spec.Components[0].ResolvedPaths = []domain.Referable[models.ResolvedPath]{
		explainPath("sib", "example.com/sibling"),
	}

	out, err := w.op.Behave(context.Background(), models.CmdExplainIn{ImportPath: "example.com/sibling"})
	require.NoError(t, err)

	assert.Equal(t, "project", out.ImportType)
	assert.Equal(t, "core", out.OwnerComponent)
}

// An invalid spec (document notices) is a config error, not a verdict.
func TestBehave_InvalidSpecIsConfigError(t *testing.T) {
	w := wireExplain(t, false)
	w.info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).Return(domain.Project{}, nil)
	w.specA.EXPECT().Assemble(mock.Anything).RunAndReturn(func(domain.Project) (arch.Spec, error) {
		spec := w.spec
		spec.Integrity.DocumentNotices = []arch.Notice{{Notice: errors.New("broken vendor glob")}}
		return spec, nil
	})

	_, err := w.op.Behave(context.Background(), models.CmdExplainIn{ImportPath: stdFixtureImport})
	require.Error(t, err)
	assert.ErrorContains(t, err, "arch spec is invalid")
}

// Every port failure propagates with its cause wrapped.
func TestBehave_PortErrorsWrap(t *testing.T) {
	healthyInfo := func(w *explainWired) {
		w.info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).
			Return(domain.Project{}, nil)
	}
	cases := []struct {
		name  string
		setup func(w *explainWired)
		want  string
	}{
		{"project info", func(w *explainWired) {
			w.info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).
				Return(domain.Project{}, errors.New("no go.mod"))
		}, "failed to assemble project info"},
		{"spec assembler", func(w *explainWired) {
			healthyInfo(w)
			w.specA.EXPECT().Assemble(mock.Anything).
				Return(arch.Spec{}, errors.New("bad glob"))
		}, "failed to assemble spec"},
		{"files resolver", func(w *explainWired) {
			healthyInfo(w)
			w.specA.EXPECT().Assemble(mock.Anything).RunAndReturn(func(domain.Project) (arch.Spec, error) {
				return w.spec, nil
			})
			w.files.EXPECT().ProjectFiles(mock.Anything, mock.Anything).
				Return(nil, errors.New("walk failed"))
		}, "failed to resolve project files"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := wireExplain(t, false)
			tc.setup(w)

			_, err := w.op.Behave(context.Background(), models.CmdExplainIn{ImportPath: stdFixtureImport})
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.want)
		})
	}
}
