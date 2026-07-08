package graph

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

// renderMermaid generates Mermaid flowchart text from edges.
// Component dependencies use solid arrows (-->), vendor dependencies use dotted (-.->).
func (o *Operation) renderMermaid(edges []graphEdge, opts models.CmdGraphIn) string {
	sortedEdges := make([]graphEdge, len(edges))
	copy(sortedEdges, edges)
	slices.SortFunc(sortedEdges, func(a, b graphEdge) int {
		if a.from != b.from {
			return cmp.Compare(a.from, b.from)
		}
		return cmp.Compare(a.to, b.to)
	})

	seen := make(map[string]string, 32)
	var b strings.Builder
	b.WriteString("graph LR\n")

	for _, e := range sortedEdges {
		fromID := mermaidNodeID(e.from, seen)
		toID := mermaidNodeID(e.to, seen)

		src, dst := fromID, toID
		if opts.Type == models.GraphTypeDI {
			src, dst = toID, fromID
		}

		if e.isVendor {
			fmt.Fprintf(&b, "  %s -.-> %s\n", src, dst)
		} else {
			fmt.Fprintf(&b, "  %s --> %s\n", src, dst)
		}
	}

	return b.String()
}

// mermaidNodeID returns a Mermaid-safe node reference, declaring it on first use.
// Example:  handler        → "handler"
//
//	3rd-cobra      → `n0["3rd-cobra"]`
func mermaidNodeID(name string, seen map[string]string) string {
	if id, ok := seen[name]; ok {
		return id
	}

	if isAlnum(name) {
		seen[name] = name
		return name
	}

	id := fmt.Sprintf("n%d", len(seen))
	seen[name] = id
	return fmt.Sprintf(`%s["%s"]`, id, name)
}

func isAlnum(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		isLower := r >= 'a' && r <= 'z'
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		isUnderscore := r == '_'
		if !isLower && !isUpper && !isDigit && !isUnderscore {
			return false
		}
	}

	return true
}
