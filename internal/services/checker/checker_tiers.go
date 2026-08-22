package checker

import (
	"context"
	"fmt"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
)

// TierRules asserts that dependencies only flow downward through the
// declared tiers: a component in a higher tier may depend on lower tiers,
// but a lower tier importing upward is a violation. The check runs on the
// ACTUAL import graph (same ownership resolution as the cycles checker),
// independent of mayDependOn permissions.
type TierRules struct{}

func NewTierRules() *TierRules {
	return &TierRules{}
}

func (c *TierRules) Check(ctx context.Context, spec arch.Spec, projectFiles []models.FileHold) (models.CheckResult, error) {
	if len(spec.Tiers) == 0 {
		return models.CheckResult{}, nil
	}

	// component -> tier index (0 = highest)
	tierOf := map[string]int{}
	for i, tier := range spec.Tiers {
		for _, component := range tier.Components {
			tierOf[component] = i
		}
	}

	graph := buildComponentGraph(spec, projectFiles)

	result := models.CheckResult{}

	for from, tos := range graph {
		fromTier, checked := tierOf[from]
		if !checked {
			continue // component not in any tier — unchecked
		}

		for to, w := range tos {
			toTier, checked := tierOf[to]
			if !checked {
				continue
			}

			// Downward (fromTier < toTier, i.e. toward later tiers) and
			// same-tier are allowed; upward edges are violations.
			if fromTier <= toTier {
				continue
			}

			result.DependencyWarnings = append(result.DependencyWarnings, models.CheckArchWarningDependency{
				ComponentName: fmt.Sprintf(
					"%s (tier '%s') -> %s (tier '%s') — dependencies may only flow downward: %s",
					from, spec.Tiers[fromTier].Name,
					to, spec.Tiers[toTier].Name,
					strings.Join(tierNames(spec), " -> "),
				),
				FileRelativePath:   strings.TrimPrefix(w.file, spec.RootDirectory.Value),
				FileAbsolutePath:   w.file,
				ResolvedImportName: w.imp.Name,
				Reference:          w.imp.Reference,
			})
		}
	}

	return result, nil
}

// tierNames returns the declared tier names in order (highest first).
func tierNames(spec arch.Spec) []string {
	names := make([]string, len(spec.Tiers))
	for i, tier := range spec.Tiers {
		names[i] = tier.Name
	}
	return names
}
