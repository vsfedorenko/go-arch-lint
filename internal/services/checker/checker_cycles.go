package checker

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
)

// Cycles detects dependency cycles between components using the ACTUAL
// import graph (not the declared mayDependOn spec). A cycle is a strongly
// connected component of size > 1. Each hop of a cycle is reported with a
// witness file, so the user can jump straight to the offending import.
//
// Cycles are reported independently of allowed/forbidden edges: even a
// fully "allowed" pair of mutual imports is an architectural smell — the
// components can no longer be understood, tested, or released apart.
type Cycles struct {
	projectFilesResolver projectFilesResolver
}

func NewCycles(
	projectFilesResolver projectFilesResolver,
) *Cycles {
	return &Cycles{
		projectFilesResolver: projectFilesResolver,
	}
}

// cycleWitness records one concrete file+import that creates a graph edge.
type cycleWitness struct {
	file string // absolute path of the offending file
	imp  models.ResolvedImport
}

// componentGraph is the actual component dependency graph: edge a -> b
// exists when any file of component a imports a package of component b.
type componentGraph map[string]map[string]cycleWitness

func (c *Cycles) Check(ctx context.Context, spec arch.Spec) (models.CheckResult, error) {
	projectFiles, err := c.projectFilesResolver.ProjectFiles(ctx, spec)
	if err != nil {
		return models.CheckResult{}, fmt.Errorf("failed to resolve project files: %w", err)
	}

	graph := buildComponentGraph(spec, projectFiles)
	result := models.CheckResult{}

	for _, scc := range findSCCs(graph) {
		if len(scc) < 2 {
			continue
		}

		cycle := orderCycle(scc, graph)

		for i, from := range cycle {
			to := cycle[(i+1)%len(cycle)]

			w, ok := graph[from][to]
			if !ok {
				// Complex SCC where the walk is not a pure ring: the last
				// hop may not exist — only report real edges.
				continue
			}

			result.DependencyWarnings = append(result.DependencyWarnings, models.CheckArchWarningDependency{
				ComponentName:      fmt.Sprintf("%s -> %s (cycle: %s)", from, to, strings.Join(cycle, " -> ")),
				FileRelativePath:   strings.TrimPrefix(w.file, spec.RootDirectory.Value),
				FileAbsolutePath:   w.file,
				ResolvedImportName: w.imp.Name,
				Reference:          w.imp.Reference,
			})
		}
	}

	return result, nil
}

// buildComponentGraph resolves each project file's imports to component
// edges. Package ownership follows the scanner's rules: a package belongs
// to the component whose ResolvedPath ImportPath is the longest matching
// "/"-boundary prefix.
func buildComponentGraph(spec arch.Spec, projectFiles []models.FileHold) componentGraph {
	type owned struct {
		component string
		prefixLen int
	}

	// Longest-prefix ownership: register every component path, keep the
	// longest when prefixes nest ("internal/a" vs "internal/a/b").
	owners := map[string]owned{}
	for _, component := range spec.Components {
		for _, resolvedPath := range component.ResolvedPaths {
			prefix := resolvedPath.Value.ImportPath
			if existing, ok := owners[prefix]; !ok || len(prefix) > existing.prefixLen {
				owners[prefix] = owned{component: component.Name.Value, prefixLen: len(prefix)}
			}
		}
	}

	resolveOwner := func(importPath string) (string, bool) {
		for p := importPath; p != ""; {
			if o, ok := owners[p]; ok {
				return o.component, true
			}

			i := strings.LastIndex(p, "/")
			if i < 0 {
				break
			}
			p = p[:i]
		}

		return "", false
	}

	graph := componentGraph{}

	for _, projectFile := range projectFiles {
		if projectFile.ComponentID == nil {
			continue
		}

		from := *projectFile.ComponentID

		for _, resolvedImport := range projectFile.File.Imports {
			if resolvedImport.ImportType != models.ImportTypeProject {
				continue
			}

			to, ok := resolveOwner(resolvedImport.Name)
			if !ok || to == from {
				continue
			}

			if graph[from] == nil {
				graph[from] = map[string]cycleWitness{}
			}

			// Keep the first witness per hop — deterministic (files come
			// sorted from the resolver) and enough to point at the import.
			if _, exists := graph[from][to]; !exists {
				graph[from][to] = cycleWitness{
					file: projectFile.File.Path,
					imp:  resolvedImport,
				}
			}
		}
	}

	return graph
}

// findSCCs returns all strongly connected components of size > 1.
// Iterative Tarjan — no recursion, so deep graphs cannot blow the stack.
// Deterministic: nodes and successors are visited in sorted order.
func findSCCs(graph componentGraph) [][]string {
	// Collect the node set: targets without outgoing edges still
	// participate in cycles.
	nodeSet := map[string]struct{}{}
	for from, tos := range graph {
		nodeSet[from] = struct{}{}
		for to := range tos {
			nodeSet[to] = struct{}{}
		}
	}

	nodes := make([]string, 0, len(nodeSet))
	for n := range nodeSet {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)

	type frame struct {
		v          string
		successors []string
		next       int
	}

	index := map[string]int{}
	lowlink := map[string]int{}
	onStack := map[string]bool{}
	stack := []string{}
	var sccs [][]string
	next := 0

	for _, root := range nodes {
		if _, seen := index[root]; seen {
			continue
		}

		frames := []frame{{v: root}}

		for len(frames) > 0 {
			f := &frames[len(frames)-1]

			if f.next == 0 {
				index[f.v] = next
				lowlink[f.v] = next
				next++
				stack = append(stack, f.v)
				onStack[f.v] = true

				succ := make([]string, 0, len(graph[f.v]))
				for w := range graph[f.v] {
					succ = append(succ, w)
				}
				sort.Strings(succ)
				f.successors = succ
			}

			if f.next < len(f.successors) {
				w := f.successors[f.next]
				f.next++

				if _, seen := index[w]; !seen {
					frames = append(frames, frame{v: w})
				} else if onStack[w] && index[w] < lowlink[f.v] {
					lowlink[f.v] = index[w]
				}

				continue
			}

			// All successors processed — finish v.
			if lowlink[f.v] == index[f.v] {
				var scc []string
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[w] = false
					scc = append(scc, w)
					if w == f.v {
						break
					}
				}
				if len(scc) > 1 {
					sccs = append(sccs, scc)
				}
			}

			frames = frames[:len(frames)-1]

			if len(frames) > 0 {
				parent := &frames[len(frames)-1]
				if lowlink[f.v] < lowlink[parent.v] {
					lowlink[parent.v] = lowlink[f.v]
				}
			}
		}
	}

	return sccs
}

// orderCycle rotates an SCC into traversal order: starting from the
// smallest node, repeatedly move to the smallest unvisited in-SCC
// successor. For a simple cycle this yields the actual hop sequence; for
// complex SCCs it yields a walk visiting every node, with unvisited
// nodes appended in sorted order.
func orderCycle(scc []string, graph componentGraph) []string {
	sorted := append([]string(nil), scc...)
	sort.Strings(sorted)

	inSCC := make(map[string]struct{}, len(sorted))
	for _, n := range sorted {
		inSCC[n] = struct{}{}
	}

	ordered := make([]string, 0, len(sorted))
	visited := map[string]struct{}{}

	cur := sorted[0]
	for {
		if _, seen := visited[cur]; seen {
			break
		}
		visited[cur] = struct{}{}
		ordered = append(ordered, cur)

		next := ""
		for w := range graph[cur] {
			if _, isIn := inSCC[w]; !isIn {
				continue
			}
			if _, seen := visited[w]; seen {
				continue
			}
			if next == "" || w < next {
				next = w
			}
		}

		if next == "" {
			break
		}
		cur = next
	}

	for _, n := range sorted {
		if _, seen := visited[n]; !seen {
			ordered = append(ordered, n)
		}
	}

	return ordered
}
