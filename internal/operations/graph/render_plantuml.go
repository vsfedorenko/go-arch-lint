package graph

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

// renderPlantUML generates PlantUML component diagram text from edges.
// Component dependencies use solid arrows (-->, vendor dependencies use dotted (..>).
func (o *Operation) renderPlantUML(edges []graphEdge, opts models.CmdGraphIn) string {
	components := make(map[string]struct{}, 32)
	for _, e := range edges {
		components[e.from] = struct{}{}
		components[e.to] = struct{}{}
	}

	names := make([]string, 0, len(components))
	for name := range components {
		names = append(names, name)
	}
	slices.Sort(names)

	sortedEdges := make([]graphEdge, len(edges))
	copy(sortedEdges, edges)
	slices.SortFunc(sortedEdges, func(a, b graphEdge) int {
		if a.from != b.from {
			return cmp.Compare(a.from, b.from)
		}
		return cmp.Compare(a.to, b.to)
	})

	var b strings.Builder
	b.WriteString("@startuml\n")

	for _, name := range names {
		fmt.Fprintf(&b, "component [%q]\n", name)
	}

	b.WriteString("\n")

	for _, e := range sortedEdges {
		from, to := directedEdge(e, opts.Type)
		if e.isVendor {
			fmt.Fprintf(&b, "[%s] ..> [%s]\n", from, to)
		} else {
			fmt.Fprintf(&b, "[%s] --> [%s]\n", from, to)
		}
	}

	b.WriteString("@enduml\n")

	return b.String()
}
