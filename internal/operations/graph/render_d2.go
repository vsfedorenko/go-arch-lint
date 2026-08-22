package graph

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
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
`, d2Key(e.to), d2Key(e.to), d2Key(e.from), flow, d2Key(e.to)))
		} else {
			linesBuff = append(linesBuff, fmt.Sprintf("%s %s %s\n", d2Key(e.from), flow, d2Key(e.to)))
		}
	}

	slices.Sort(linesBuff)

	var buff bytes.Buffer
	for _, line := range linesBuff {
		buff.WriteString(strings.ReplaceAll(line, "\t", ""))
	}

	return buff.String()
}

// d2Key quotes a component name for use as a d2 key. Component names are
// directory paths: "." (the module root) is a reserved token in d2 and a
// dotted path segment ("pkg.v1") would nest instead of naming a node, so
// every key is quoted uniformly.
func d2Key(name string) string {
	return fmt.Sprintf("%q", name)
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
