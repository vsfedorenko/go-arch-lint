package checker

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

/**
 * TierRules checker tests. Reuses the cycles-test helpers: cyclesSpec
 * builds a spec with components owning packages, fakeProjectFilesResolver
 * feeds fixed file holds, cyclesFile builds holds with ComponentID.
 */

const (
	tcTierDomain = "domain"
	tcTierApp    = "app"
	tcTierInfra  = "infra"
)

func tiersSpec(tiers []arch.Tier) arch.Spec {
	spec := cyclesSpec(map[string][]string{
		tcAlpha: {tcPkgAlpha},
		tcBeta:  {tcPkgBeta},
		tcCore:  {tcPkgCore},
	})
	spec.Tiers = tiers
	return spec
}

func tierRef() domain.Reference {
	return domain.NewEmptyReference()
}

func TestTierRules_upward_dependency_is_violation(t *testing.T) {
	// Tiers: domain (alpha) -> app (beta). beta importing alpha is upward.
	spec := tiersSpec([]arch.Tier{
		{Name: tcTierDomain, Components: []string{tcAlpha}, Reference: tierRef()},
		{Name: tcTierApp, Components: []string{tcBeta}, Reference: tierRef()},
	})

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcBeta, "internal/beta/b.go", cyclesTestModule+"/"+tcPkgAlpha), // app -> domain: UP
	}}

	result, err := NewTierRules(resolver).Check(context.Background(), spec)
	require.NoError(t, err)
	require.Len(t, result.DependencyWarnings, 1)

	w := result.DependencyWarnings[0]
	assert.Contains(t, w.ComponentName, "beta (tier 'app')")
	assert.Contains(t, w.ComponentName, "alpha (tier 'domain')")
	assert.Contains(t, w.ComponentName, "downward")
	assert.Equal(t, w.ResolvedImportName, cyclesTestModule+"/"+tcPkgAlpha)
}

func TestTierRules_downward_and_same_tier_allowed(t *testing.T) {
	spec := tiersSpec([]arch.Tier{
		{Name: tcTierDomain, Components: []string{tcAlpha}, Reference: tierRef()},
		{Name: tcTierApp, Components: []string{tcBeta}, Reference: tierRef()},
	})

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcAlpha, "internal/alpha/a.go", cyclesTestModule+"/"+tcPkgBeta), // domain -> app: DOWN
	}}

	result, err := NewTierRules(resolver).Check(context.Background(), spec)
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestTierRules_same_tier_allowed(t *testing.T) {
	spec := tiersSpec([]arch.Tier{
		{Name: tcTierDomain, Components: []string{tcAlpha, tcBeta}, Reference: tierRef()},
		{Name: tcTierInfra, Components: []string{tcCore}, Reference: tierRef()},
	})

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcAlpha, "internal/alpha/a.go", cyclesTestModule+"/"+tcPkgBeta), // same tier
	}}

	result, err := NewTierRules(resolver).Check(context.Background(), spec)
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestTierRules_unchecked_components_ignored(t *testing.T) {
	// core is in no tier — its edges are not tier-checked.
	spec := tiersSpec([]arch.Tier{
		{Name: tcTierDomain, Components: []string{tcAlpha}, Reference: tierRef()},
	})

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcCore, "internal/beta/b.go", cyclesTestModule+"/"+tcPkgAlpha), // core -> alpha
	}}

	result, err := NewTierRules(resolver).Check(context.Background(), spec)
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestTierRules_no_tiers_configured_noop(t *testing.T) {
	spec := tiersSpec(nil)

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcBeta, "internal/beta/b.go", cyclesTestModule+"/"+tcPkgAlpha),
	}}

	result, err := NewTierRules(resolver).Check(context.Background(), spec)
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestTierRules_message_lists_full_tier_chain(t *testing.T) {
	spec := tiersSpec([]arch.Tier{
		{Name: tcTierDomain, Components: []string{tcAlpha}, Reference: tierRef()},
		{Name: tcTierApp, Components: []string{tcBeta}, Reference: tierRef()},
		{Name: tcTierInfra, Components: []string{tcCore}, Reference: tierRef()},
	})

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcCore, "internal/beta/b.go", cyclesTestModule+"/"+tcPkgAlpha), // infra -> domain
	}}

	result, err := NewTierRules(resolver).Check(context.Background(), spec)
	require.NoError(t, err)
	require.Len(t, result.DependencyWarnings, 1)
	assert.True(t, strings.Contains(result.DependencyWarnings[0].ComponentName, "domain -> app -> infra"))
}
