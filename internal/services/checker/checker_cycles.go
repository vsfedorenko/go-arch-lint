package checker

import (
	"context"
	"fmt"
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
//
// Graph construction lives in component_graph.go; the SCC machinery
// lives in scc.go. This file is the checker only.
type Cycles struct{}

func NewCycles() *Cycles {
	return &Cycles{}
}

func (c *Cycles) Check(ctx context.Context, spec arch.Spec, projectFiles []models.FileHold) (models.CheckResult, error) {
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
