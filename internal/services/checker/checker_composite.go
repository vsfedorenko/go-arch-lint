package checker

import (
	"context"
	"fmt"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
)

// suppressIndexFactory builds the directive index from the resolved
// project file list. Overridable for tests; the container injects the
// suppress.NewIndexFromFiles adapter.
type suppressIndexFactory func(projectFiles []models.FileHold) (SuppressIndex, error)

// CompositeChecker coordinates all checkers: it resolves the project
// file list once, runs every checker over it, and applies source-level
// suppression directives to the aggregated result.
type CompositeChecker struct {
	projectFilesResolver projectFilesResolver
	checkers             []checker
	suppressIndexFactory suppressIndexFactory
}

func NewCompositeChecker(projectFilesResolver projectFilesResolver, checkers ...checker) *CompositeChecker {
	return &CompositeChecker{
		projectFilesResolver: projectFilesResolver,
		checkers:             checkers,
	}
}

// WithSuppressIndex wires the //go-arch-lint:ignore directive index
// factory. Without it no filtering happens (feature off).
func (c *CompositeChecker) WithSuppressIndex(factory suppressIndexFactory) *CompositeChecker {
	c.suppressIndexFactory = factory
	return c
}

func (c *CompositeChecker) Check(ctx context.Context, spec arch.Spec) (models.CheckResult, error) {
	projectFiles, err := c.projectFilesResolver.ProjectFiles(ctx, spec)
	if err != nil {
		return models.CheckResult{}, fmt.Errorf("failed to resolve project files: %w", err)
	}

	overallResults := models.CheckResult{}

	// Every checker runs: a violation in one category (e.g. imports)
	// must not shadow violations in the others (cycles, tiers, naming,
	// interface placement) — the user fixes what the output shows, so
	// hiding later categories would require extra runs to surface them.
	// (The historical early-break predates the multi-category wave and
	// silently swallowed e.g. naming violations whenever imports failed.)
	for _, checker := range c.checkers {
		results, err := checker.Check(ctx, spec, projectFiles)
		if err != nil {
			return models.CheckResult{}, fmt.Errorf("checker failed '%T': %w", checker, err)
		}

		overallResults.Append(results)
	}

	// Apply //go-arch-lint:ignore directives AFTER aggregation so the
	// filtering covers every warning kind and every output format
	// (text, JSON, SARIF, JUnit) uniformly.
	if c.suppressIndexFactory != nil {
		index, err := c.suppressIndexFactory(projectFiles)
		if err != nil {
			return models.CheckResult{}, fmt.Errorf("failed to build suppress index: %w", err)
		}

		filtered, suppressed := (&SuppressFilter{index: index}).Filter(overallResults)
		overallResults = filtered
		overallResults.SuppressedCount = suppressed
	}

	return overallResults, nil
}
