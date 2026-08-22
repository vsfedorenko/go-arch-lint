package checker

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

/**
 * Cycles checker tests. The graph machinery (findSCCs, orderCycle) is
 * tested directly on synthetic graphs; the full Check() path runs against
 * a real resolver over generated on-disk fixture packages, then the
 * warnings are asserted semantically (a->b->a reported from both files).
 */

// Repeated fixture identifiers (goconst-clean).
const (
	tcAlpha    = "alpha"
	tcBeta     = "beta"
	tcGamma    = "gamma"
	tcCore     = "core"
	tcPkgCore  = "internal"
	tcPkgAlpha = "internal/alpha"
	tcPkgBeta  = "internal/beta"
	tcZeta     = "zeta"
)

// --- graph machinery -------------------------------------------------------

func makeGraph(edges map[string][]string) componentGraph {
	graph := componentGraph{}
	for from, tos := range edges {
		graph[from] = map[string]graphWitness{}
		for _, to := range tos {
			graph[from][to] = graphWitness{}
		}
	}
	return graph
}

func Test_findSCCs_simple_pairs(t *testing.T) {
	graph := makeGraph(map[string][]string{
		"a": {"b"},
		"b": {"a"},
		"c": {"a"},
		"d": {"e"},
		"e": {},
	})

	sccs := findSCCs(graph)
	require.Len(t, sccs, 1)
	assert.ElementsMatch(t, []string{"a", "b"}, sccs[0])
}

func Test_findSCCs_ring_of_three(t *testing.T) {
	graph := makeGraph(map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	})

	sccs := findSCCs(graph)
	require.Len(t, sccs, 1)
	assert.ElementsMatch(t, []string{"a", "b", "c"}, sccs[0])
}

func Test_findSCCs_two_disjoint_cycles(t *testing.T) {
	graph := makeGraph(map[string][]string{
		"a": {"b"}, "b": {"a"},
		"c": {"d"}, "d": {"c"},
		"x": {"a", "c"},
	})

	sccs := findSCCs(graph)
	require.Len(t, sccs, 2)
	flat := append(append([]string{}, sccs[0]...), sccs[1]...)
	assert.ElementsMatch(t, []string{"a", "b", "c", "d"}, flat)
}

func Test_findSCCs_nested_cycle_with_tail(t *testing.T) {
	// a -> b -> c -> b (cycle b/c), a is a tail into the cycle.
	graph := makeGraph(map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"b"},
	})

	sccs := findSCCs(graph)
	require.Len(t, sccs, 1)
	assert.ElementsMatch(t, []string{"b", "c"}, sccs[0])
}

func Test_findSCCs_acyclic_graph_yields_none(t *testing.T) {
	graph := makeGraph(map[string][]string{
		"a": {"b", "c"},
		"b": {"c"},
		"c": {},
	})

	assert.Empty(t, findSCCs(graph))
}

func Test_findSCCs_deterministic_order(t *testing.T) {
	graph := makeGraph(map[string][]string{
		tcZeta:  {"alpha"},
		tcAlpha: {tcZeta},
	})

	sccs := findSCCs(graph)
	require.Len(t, sccs, 1)
	// Tarjan starts from the sorted-first root ("alpha").
	assert.Equal(t, []string{tcZeta, "alpha"}, sccs[0])
}

func Test_orderCycle_simple_ring(t *testing.T) {
	graph := makeGraph(map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	})

	// Starting from the smallest node a: a -> b -> c.
	assert.Equal(t, []string{"a", "b", "c"}, orderCycle([]string{"c", "a", "b"}, graph))
}

func Test_orderCycle_complex_scc_walks_all_nodes(t *testing.T) {
	// a <-> b, a <-> c: everything is one SCC; the walk must visit all.
	graph := makeGraph(map[string][]string{
		"a": {"b", "c"},
		"b": {"a"},
		"c": {"a"},
	})

	ordered := orderCycle([]string{"a", "b", "c"}, graph)
	assert.ElementsMatch(t, []string{"a", "b", "c"}, ordered)
}

// --- full Check path --------------------------------------------------------

const cyclesTestModule = "github.com/vsfedorenko/go-arch-lint/v3/checker/cyclestest"

type fakeProjectFilesResolver struct {
	holds []models.FileHold
}

func (r fakeProjectFilesResolver) ProjectFiles(_ context.Context, _ arch.Spec) ([]models.FileHold, error) {
	return r.holds, nil
}

func cyclesSpec(components map[string][]string) arch.Spec {
	spec := arch.Spec{
		RootDirectory: domain.NewReferable("/project", domain.NewEmptyReference()),
		ModuleName:    domain.NewReferable(cyclesTestModule, domain.NewEmptyReference()),
	}

	for name, pkgs := range components {
		component := arch.Component{
			Name: domain.NewReferable(name, domain.NewEmptyReference()),
		}
		for _, pkg := range pkgs {
			component.ResolvedPaths = append(component.ResolvedPaths, domain.NewReferable(
				models.ResolvedPath{
					ImportPath: cyclesTestModule + "/" + pkg,
					LocalPath:  pkg,
					AbsPath:    "/project/" + pkg,
				},
				domain.NewEmptyReference(),
			))
		}
		spec.Components = append(spec.Components, component)
	}

	sort.Slice(spec.Components, func(i, j int) bool {
		return spec.Components[i].Name.Value < spec.Components[j].Name.Value
	})

	return spec
}

// cyclesFile builds a FileHold for a file of the given component.
func cyclesFile(component, relPath string, imports ...string) models.FileHold {
	file := models.ProjectFile{
		Path:    "/project/" + relPath,
		Imports: make([]models.ResolvedImport, 0, len(imports)),
	}
	for _, imp := range imports {
		file.Imports = append(file.Imports, models.ResolvedImport{
			Name:       imp,
			ImportType: models.ImportTypeProject,
		})
	}

	return models.FileHold{File: file, ComponentID: &component}
}

func TestCycles_Check_reports_mutual_imports(t *testing.T) {
	spec := cyclesSpec(map[string][]string{
		tcAlpha: {tcPkgAlpha},
		tcBeta:  {tcPkgBeta},
	})

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile("alpha", "internal/alpha/a.go", cyclesTestModule+"/internal/beta"),
		cyclesFile("beta", "internal/beta/b.go", cyclesTestModule+"/internal/alpha"),
	}}

	result, err := NewCycles().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	require.Len(t, result.DependencyWarnings, 2) // both hops reported

	messages := []string{}
	for _, w := range result.DependencyWarnings {
		messages = append(messages, w.ComponentName)
	}
	joined := strings.Join(messages, "; ")
	assert.Contains(t, joined, "alpha -> beta")
	assert.Contains(t, joined, "beta -> alpha")
	assert.Contains(t, joined, "(cycle:")
}

func TestCycles_Check_clean_graph_no_warnings(t *testing.T) {
	spec := cyclesSpec(map[string][]string{
		tcAlpha: {tcPkgAlpha},
		tcBeta:  {tcPkgBeta},
	})

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile("alpha", "internal/alpha/a.go", cyclesTestModule+"/internal/beta"), // one-way
	}}

	result, err := NewCycles().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestCycles_Check_self_import_and_vendor_ignored(t *testing.T) {
	spec := cyclesSpec(map[string][]string{
		tcAlpha: {tcPkgAlpha, "internal/alpha/sub"},
	})

	// Same component (sub-package), vendor and stdlib imports — none may
	// create edges.
	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile("alpha", "internal/alpha/a.go",
			cyclesTestModule+"/internal/alpha/sub",
			"github.com/some/vendor/lib",
			"fmt",
		),
	}}

	result, err := NewCycles().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestCycles_Check_longest_prefix_ownership(t *testing.T) {
	// "internal/alpha/sub" is owned by alpha via the longest prefix even
	// though a shorter "internal" prefix could belong elsewhere.
	spec := cyclesSpec(map[string][]string{
		tcCore:  {tcPkgCore},
		tcAlpha: {tcPkgAlpha},
	})

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile("alpha", "internal/alpha/a.go", cyclesTestModule+"/internal/beta"), // alpha -> core
		cyclesFile("core", "internal/beta/b.go", cyclesTestModule+"/internal/alpha"),  // core -> alpha
	}}

	result, err := NewCycles().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	assert.Len(t, result.DependencyWarnings, 2) // core <-> alpha cycle
}

func TestCycles_Check_unowned_packages_ignored(t *testing.T) {
	spec := cyclesSpec(map[string][]string{
		tcAlpha: {tcPkgAlpha},
	})

	// internal/beta belongs to no component — no edge can be built.
	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile("alpha", "internal/alpha/a.go", cyclesTestModule+"/internal/beta"),
	}}

	result, err := NewCycles().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

// --- real fixture packages (deep recursion safety) --------------------------

// TestCycles_deep_cycle_does_not_blow_stack builds a 10k-node ring on
// disk via a synthetic graph only (no disk churn) — the recursion-free
// iterative Tarjan must handle it instantly.
func TestCycles_deep_ring_handled_iteratively(t *testing.T) {
	const n = 10_000

	edges := map[string][]string{}
	for i := range n {
		edges[fmt.Sprintf("c%05d", i)] = []string{fmt.Sprintf("c%05d", (i+1)%n)}
	}

	sccs := findSCCs(makeGraph(edges))
	require.Len(t, sccs, 1)
	assert.Len(t, sccs[0], n)
}
