package mapping

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

// operation_test.go pins the mapping operation at the unit layer: the
// happy-path wiring (spec → files → grouped+list views), error propagation
// from every port, and the pure grouping/sorting/dedup rules of the
// private helpers — including the "[not attached]" bucket for files no
// component claims.

// fixtures ---------------------------------------------------------------

func mapFile(path, component string) models.FileHold {
	id := component
	if component == "" {
		return models.FileHold{File: models.ProjectFile{Path: path}}
	}
	return models.FileHold{
		File:        models.ProjectFile{Path: path},
		ComponentID: &id,
	}
}

func mapSpec(components ...string) arch.Spec {
	spec := arch.Spec{
		RootDirectory: domain.NewReferable("/project", domain.NewEmptyReference()),
		ModuleName:    domain.NewReferable("example.com/m", domain.NewEmptyReference()),
	}
	for _, name := range components {
		spec.Components = append(spec.Components, arch.Component{
			Name: domain.NewReferable(name, domain.NewEmptyReference()),
		})
	}
	return spec
}

type mapWired struct {
	op    *Operation
	info  *mockprojectInfoAssembler
	specA *mockspecAssembler
	files *mockprojectFilesResolver
	spec  arch.Spec
	holds []models.FileHold
}

func wireMapping(t *testing.T, healthy ...bool) *mapWired {
	t.Helper()
	w := &mapWired{
		info:  newMockprojectInfoAssembler(t),
		specA: newMockspecAssembler(t),
		files: newMockprojectFilesResolver(t),
		spec:  mapSpec("alpha", "beta"),
		holds: []models.FileHold{mapFile("b.go", "beta"), mapFile("a.go", "alpha")},
	}
	w.op = NewOperation(w.specA, w.files, w.info)

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

// Happy path: grouped view follows SPEC component order (empty components
// included), file lists are sorted, the flat view is deduped and sorted.
func TestBehave_GroupsAndLists(t *testing.T) {
	w := wireMapping(t)

	out, err := w.op.Behave(context.Background(), models.CmdMappingIn{})
	require.NoError(t, err)
	assert.Equal(t, "example.com/m", out.ModuleName)

	require.Len(t, out.MappingGrouped, 2)
	assert.Equal(t, "alpha", out.MappingGrouped[0].ComponentName)
	assert.Equal(t, []string{"a.go"}, out.MappingGrouped[0].FileNames)
	assert.Equal(t, "beta", out.MappingGrouped[1].ComponentName)

	require.Len(t, out.MappingList, 2)
	assert.Equal(t, "a.go", out.MappingList[0].FileName, "flat view sorted by file name")
	assert.Equal(t, "b.go", out.MappingList[1].FileName)
}

// A spec component with no files still appears in the grouped mapping —
// an empty component is information, not an omission.
func TestBehave_EmptyComponentStillListed(t *testing.T) {
	w := wireMapping(t)
	w.holds = []models.FileHold{mapFile("a.go", "alpha")}

	out, err := w.op.Behave(context.Background(), models.CmdMappingIn{})
	require.NoError(t, err)
	require.Len(t, out.MappingGrouped, 2)
	assert.Equal(t, "beta", out.MappingGrouped[1].ComponentName)
	assert.Empty(t, out.MappingGrouped[1].FileNames, "component without files: empty list, still present")
}

// Files no component claims land in the trailing "[not attached]" group.
func TestBehave_NotAttachedBucket(t *testing.T) {
	w := wireMapping(t)
	w.holds = append(w.holds, mapFile("orphan.go", ""))

	out, err := w.op.Behave(context.Background(), models.CmdMappingIn{})
	require.NoError(t, err)
	// The grouped view is sorted alphabetically and "[" sorts before
	// letters — the not-attached bucket leads, not trails.
	require.Len(t, out.MappingGrouped, 3)
	first := out.MappingGrouped[0]
	assert.Equal(t, "[not attached]", first.ComponentName)
	assert.Equal(t, []string{"orphan.go"}, first.FileNames)

	require.Len(t, out.MappingList, 3)
	assert.Equal(t, "[not attached]", out.MappingList[2].ComponentName, "flat view sorts by file name; orphan.go is last")
}

// Coupling computed on the import graph is attached to its component.
func TestBehave_CouplingAttached(t *testing.T) {
	w := wireMapping(t)

	out, err := w.op.Behave(context.Background(), models.CmdMappingIn{})
	require.NoError(t, err)
	require.Len(t, out.MappingGrouped, 2)

	// wireMapping files have no imports: coupling attaches with zeros.
	require.NotNil(t, out.MappingGrouped[0].Coupling, "coupling must attach even when zero")
	assert.Equal(t, "alpha", out.MappingGrouped[0].Coupling.Name)
}

// Every port failure propagates with its cause wrapped.
func TestBehave_PortErrorsWrap(t *testing.T) {
	healthyInfo := func(w *mapWired) {
		w.info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).
			Return(domain.Project{}, nil)
	}
	cases := []struct {
		name  string
		setup func(w *mapWired)
		want  string
	}{
		{"project info", func(w *mapWired) {
			w.info.EXPECT().ProjectInfo(mock.Anything, mock.Anything).
				Return(domain.Project{}, errors.New("no go.mod"))
		}, "failed to assemble project info"},
		{"spec assembler", func(w *mapWired) {
			healthyInfo(w)
			w.specA.EXPECT().Assemble(mock.Anything).
				Return(arch.Spec{}, errors.New("bad glob"))
		}, "failed to assemble spec"},
		{"files resolver", func(w *mapWired) {
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
			// No healthy wiring: each case installs exactly one failing
			// expectation, upstream ports stay unconfigured (their call
			// would panic loudly if the order regressed).
			w := wireMapping(t, false)
			tc.setup(w)

			_, err := w.op.Behave(context.Background(), models.CmdMappingIn{})
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

// attachCoupling is a pure join: entries without metrics keep nil coupling.
func TestAttachCoupling(t *testing.T) {
	grouped := []models.CmdMappingOutGrouped{
		{ComponentName: "a"},
		{ComponentName: "b"},
	}
	coupling := []models.ComponentCoupling{
		{Name: "b", OutboundDeps: 3, InboundDeps: 1, Stability: 0.75},
	}

	attachCoupling(grouped, coupling)

	assert.Nil(t, grouped[0].Coupling, "component without metrics: nil coupling")
	require.NotNil(t, grouped[1].Coupling)
	assert.Equal(t, 3, grouped[1].Coupling.OutboundDeps)
	assert.InDelta(t, 0.75, grouped[1].Coupling.Stability, 1e-9)
}
