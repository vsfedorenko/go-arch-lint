package checker

import (
	"sort"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
)

// This file is the SHARED foundation every graph-based checker builds on:
// resolving project files into a component dependency graph with package
// ownership rules. Consumers: cycles, tiers, coupling metrics. Keep it
// free of checker-specific logic.

// graphWitness records one concrete file+import that creates a graph edge.
type graphWitness struct {
	file string // absolute path of the offending file
	imp  models.ResolvedImport
}

// componentGraph is the actual component dependency graph: edge a -> b
// exists when any file of component a imports a package of component b.
type componentGraph map[string]map[string]graphWitness

// buildComponentGraph resolves each project file's imports to component
// edges. Package ownership follows the scanner's rules: a package belongs
// to the component whose ResolvedPath ImportPath is the longest matching
// "/"-boundary prefix.
func buildComponentGraph(spec arch.Spec, projectFiles []models.FileHold) componentGraph {
	resolveOwner := packageOwnerResolver(spec)

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
				graph[from] = map[string]graphWitness{}
			}

			// Keep the first witness per hop — deterministic (files come
			// sorted from the resolver) and enough to point at the import.
			if _, exists := graph[from][to]; !exists {
				graph[from][to] = graphWitness{
					file: projectFile.File.Path,
					imp:  resolvedImport,
				}
			}
		}
	}

	return graph
}

// buildPackageOwnerMap maps every project package directory to its owning
// component (component ID per file hold). Used by checkers that reason
// per-package rather than per-import-edge (interface placement).
func buildPackageOwnerMap(projectFiles []models.FileHold) map[string]string {
	owners := map[string]string{}
	for _, hold := range projectFiles {
		if hold.ComponentID == nil {
			continue
		}
		owners[packagePathOf(hold.File.Path)] = *hold.ComponentID
	}
	return owners
}

// packageOwnerResolver returns a function resolving a project import
// path to its owning component by longest "/"-boundary prefix over the
// spec's component paths (mirrors the scanner's ownership rules).
func packageOwnerResolver(spec arch.Spec) func(importPath string) (string, bool) {
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

	return func(importPath string) (string, bool) {
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
}

// sortedGraphNodes returns all nodes of the graph (sources and targets)
// in sorted order — targets without outgoing edges still participate in
// cycles.
func sortedGraphNodes(graph componentGraph) []string {
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

	return nodes
}
