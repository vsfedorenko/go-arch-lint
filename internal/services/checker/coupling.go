package checker

import (
	"cmp"
	"sort"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
)

// ComputeCoupling measures per-component coupling over the actual
// component graph: fan-out (Ce), fan-in (Ca) and the Robert C. Martin
// stability ratio I = Ce / (Ca + Ce) in [0, 1] (0 — maximally stable,
// 1 — maximally unstable; 0 for isolated components). Pure function of
// spec + resolved files — used by the mapping operation to surface
// metrics, and testable in isolation.
func ComputeCoupling(spec arch.Spec, projectFiles []models.FileHold) []models.ComponentCoupling {
	graph := buildComponentGraph(spec, projectFiles)

	inbound := map[string]int{}
	outbound := map[string]int{}

	for from, tos := range graph {
		outbound[from] = len(tos)
		for to := range tos {
			inbound[to]++
		}
	}

	result := make([]models.ComponentCoupling, 0, len(spec.Components))
	for _, component := range spec.Components {
		name := component.Name.Value
		ca := inbound[name]
		ce := outbound[name]

		var stability float64
		if ca+ce > 0 {
			stability = float64(ce) / float64(ca+ce)
		}

		result = append(result, models.ComponentCoupling{
			Name:         name,
			OutboundDeps: ce,
			InboundDeps:  ca,
			Stability:    stability,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if c := cmp.Compare(result[i].Name, result[j].Name); c != 0 {
			return c < 0
		}
		return false
	})

	return result
}
