package graph

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

// renderD2 generates d2 graph definition text from edges.
// Vendor dependencies get specialized styling (diamond arrowhead, green stroke).
func (o *Operation) renderD2(edges []graphEdge, opts models.CmdGraphIn) string {
	flow := d2Arrow(opts.Type)

	linesBuff := make([]string, 0, len(edges)*2)

	for _, e := range edges {
		if e.isVendor {
			linesBuff = append(linesBuff, fmt.Sprintf(`%s.style.font-size: 12
%s.style.stroke: "#77AA44"
%s %s %s {
  style.stroke: "#77AA44"
  source-arrowhead: {
    shape: diamond
    style.filled: false
  }
}
`, e.to, e.to, e.from, flow, e.to))
		} else {
			linesBuff = append(linesBuff, fmt.Sprintf("%s %s %s\n", e.from, flow, e.to))
		}
	}

	slices.Sort(linesBuff)

	var buff bytes.Buffer
	for _, line := range linesBuff {
		buff.WriteString(strings.ReplaceAll(line, "\t", ""))
	}

	return buff.String()
}

func d2Arrow(graphType models.GraphType) string {
	switch graphType {
	case models.GraphTypeFlow:
		return "->"
	case models.GraphTypeDI:
		return "<-"
	default:
		return "--"
	}
}
