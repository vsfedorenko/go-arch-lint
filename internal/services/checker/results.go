package checker

import (
	"cmp"
	"slices"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

type (
	results models.CheckResult
)

func newResults() results {
	return results{
		DependencyWarnings: []models.CheckArchWarningDependency{},
		MatchWarnings:      []models.CheckArchWarningMatch{},
	}
}

func (res *results) addNotMatchedWarning(warn models.CheckArchWarningMatch) {
	res.MatchWarnings = append(res.MatchWarnings, warn)
}

func (res *results) addDependencyWarning(warn models.CheckArchWarningDependency) {
	res.DependencyWarnings = append(res.DependencyWarnings, warn)
}

func (res *results) assembleSortedResults() models.CheckResult {
	slices.SortFunc(res.DependencyWarnings, func(a, b models.CheckArchWarningDependency) int {
		return cmp.Compare(a.FileRelativePath, b.FileRelativePath)
	})

	slices.SortFunc(res.MatchWarnings, func(a, b models.CheckArchWarningMatch) int {
		return cmp.Compare(a.FileRelativePath, b.FileRelativePath)
	})

	return models.CheckResult{
		DependencyWarnings: res.DependencyWarnings,
		MatchWarnings:      res.MatchWarnings,
	}
}
