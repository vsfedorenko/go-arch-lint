package checker

import (
	"context"
	"fmt"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
)

type CompositeChecker struct {
	checkers []checker
}

func NewCompositeChecker(checkers ...checker) *CompositeChecker {
	return &CompositeChecker{checkers: checkers}
}

func (c *CompositeChecker) Check(ctx context.Context, spec arch.Spec) (models.CheckResult, error) {
	overallResults := models.CheckResult{}

	// Every checker runs: a violation in one category (e.g. imports)
	// must not shadow violations in the others (cycles, tiers, naming,
	// interface placement) — the user fixes what the output shows, so
	// hiding later categories would require extra runs to surface them.
	// (The historical early-break predates the multi-category wave and
	// silently swallowed e.g. naming violations whenever imports failed.)
	for _, checker := range c.checkers {
		results, err := checker.Check(ctx, spec)
		if err != nil {
			return models.CheckResult{}, fmt.Errorf("checker failed '%T': %w", checker, err)
		}

		overallResults.Append(results)
	}

	return overallResults, nil
}
