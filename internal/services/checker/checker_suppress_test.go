package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

// Fixture paths reused across filter tests (goconst).
const (
	tcFileA  = "/p/a.go"
	tcBetaNm = "beta" //nolint:goconst // fixture name (tcBeta taken by cycles fixtures)
)

func depWarning(component, file, importName string, line int) models.CheckArchWarningDependency {
	return models.CheckArchWarningDependency{
		ComponentName:      component,
		FileAbsolutePath:   file,
		ResolvedImportName: importName,
		Reference:          domain.NewReferenceSingleLine(file, line, 0),
	}
}

func TestSuppressFilter_NoDirectivesPassthrough(t *testing.T) {
	index := NewMockSuppressIndex(t)
	index.EXPECT().HasDirectives().Return(false)

	filter := NewSuppressFilter(index)
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha", tcFileA, "example.com/p/beta", 3),
		},
	}
	filtered, suppressed := filter.Filter(result)
	assert.Equal(t, 0, suppressed, "no directives: suppressed must be 0")
	assert.Len(t, filtered.DependencyWarnings, 1, "warning must pass through untouched")
}

func TestSuppressFilter_ImportViolationByLine(t *testing.T) {
	index := NewMockSuppressIndex(t)
	index.EXPECT().HasDirectives().Return(true)
	index.EXPECT().IsFileSuppressed(tcFileA).Return(false).Times(2)
	index.EXPECT().IsLineSuppressed(tcFileA, 3, "example.com/p/beta").Return(true)
	index.EXPECT().IsLineSuppressed(tcFileA, 4, "example.com/p/beta").Return(false)

	filter := NewSuppressFilter(index)
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha", tcFileA, "example.com/p/beta", 3),
			depWarning("alpha", tcFileA, "example.com/p/beta", 4),
		},
	}
	filtered, suppressed := filter.Filter(result)
	assert.Equal(t, 1, suppressed, "expected 1 suppressed")
	require.Len(t, filtered.DependencyWarnings, 1)
	assert.Equal(t, 4, filtered.DependencyWarnings[0].Reference.Line, "wrong warning kept")
}

func TestSuppressFilter_ImportViolationByTargetFilter(t *testing.T) {
	index := NewMockSuppressIndex(t)
	index.EXPECT().HasDirectives().Return(true)
	// exact import path filter
	index.EXPECT().IsFileSuppressed(tcFileA).Return(false).Times(2)
	index.EXPECT().IsLineSuppressed(tcFileA, 3, "example.com/p/beta").Return(true)
	index.EXPECT().IsLineSuppressed(tcFileA, 3, "example.com/p/gamma").Return(false)

	filter := NewSuppressFilter(index)
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha", tcFileA, "example.com/p/beta", 3),
			depWarning("alpha", tcFileA, "example.com/p/gamma", 3),
		},
	}
	filtered, suppressed := filter.Filter(result)
	assert.Equal(t, 1, suppressed, "target filter must suppress only the matching import")
	require.Len(t, filtered.DependencyWarnings, 1)
	assert.Equal(t, "example.com/p/gamma", filtered.DependencyWarnings[0].ResolvedImportName, "wrong warning kept")
}

func TestSuppressFilter_FileDirectiveSuppressesAllCategories(t *testing.T) {
	index := NewMockSuppressIndex(t)
	index.EXPECT().HasDirectives().Return(true)
	index.EXPECT().IsFileSuppressed(tcFileA).Return(true).Times(2)

	filter := NewSuppressFilter(index)
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha", tcFileA, "example.com/p/beta", 3),
		},
		MatchWarnings: []models.CheckArchWarningMatch{
			{FileAbsolutePath: tcFileA},
		},
	}
	filtered, suppressed := filter.Filter(result)
	assert.Equal(t, 2, suppressed, "file directive must suppress every warning in the file")
	assert.Empty(t, filtered.DependencyWarnings, "nothing must remain")
	assert.Empty(t, filtered.MatchWarnings, "nothing must remain")
}

func TestSuppressFilter_CycleRuleTarget(t *testing.T) {
	// Cycles embed the target in ComponentName: "alpha -> beta (cycle: ...)".
	// The ignore argument must match "beta", not the whole rule text.
	index := NewMockSuppressIndex(t)
	index.EXPECT().HasDirectives().Return(true)
	index.EXPECT().IsFileSuppressed(tcFileA).Return(false)
	index.EXPECT().IsLineSuppressed(tcFileA, 7, tcBetaNm).Return(true)

	filter := NewSuppressFilter(index)
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha -> beta (cycle: alpha -> beta -> alpha)", tcFileA, "example.com/p/beta", 7),
		},
	}
	filtered, suppressed := filter.Filter(result)
	assert.Equal(t, 1, suppressed, "cycle warning must be suppressed by its edge target")
	assert.Empty(t, filtered.DependencyWarnings, "cycle warning must be removed")
}

func TestSuppressFilter_TierRuleTarget(t *testing.T) {
	index := NewMockSuppressIndex(t)
	index.EXPECT().HasDirectives().Return(true)
	index.EXPECT().IsFileSuppressed(tcFileA).Return(false)
	index.EXPECT().IsLineSuppressed(tcFileA, 9, tcBetaNm).Return(true)

	filter := NewSuppressFilter(index)
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha (tier 'domain') -> beta (tier 'app') — upward dependency", tcFileA, "example.com/p/beta", 9),
		},
	}
	_, suppressed := filter.Filter(result)
	assert.Equal(t, 1, suppressed, "tier warning must be suppressed by its edge target")
}

func TestSuppressFilter_VisibilityRuleTarget(t *testing.T) {
	index := NewMockSuppressIndex(t)
	index.EXPECT().HasDirectives().Return(true)
	index.EXPECT().IsFileSuppressed(tcFileA).Return(false)
	index.EXPECT().IsLineSuppressed(tcFileA, 11, tcBetaNm).Return(true)

	filter := NewSuppressFilter(index)
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha may not consume 'beta' (restricted API: visible to [gamma])", tcFileA, "example.com/p/beta", 11),
		},
	}
	_, suppressed := filter.Filter(result)
	assert.Equal(t, 1, suppressed, "visibility warning must be suppressed by quoted target")
}

func TestSuppressFilter_MatchWarningOnlyFileDirective(t *testing.T) {
	// Match warnings carry no line: a line directive must NOT suppress
	// them, only the file-level one does.
	index := NewMockSuppressIndex(t)
	index.EXPECT().HasDirectives().Return(true)
	index.EXPECT().IsFileSuppressed(tcFileA).Return(false)

	filter := NewSuppressFilter(index)
	result := models.CheckResult{
		MatchWarnings: []models.CheckArchWarningMatch{
			{FileAbsolutePath: tcFileA},
		},
	}
	filtered, suppressed := filter.Filter(result)
	assert.Equal(t, 0, suppressed, "line directive must not suppress a match warning")
	assert.Len(t, filtered.MatchWarnings, 1, "match warning must survive")
}

func TestSuppressFilter_NamingWarningOnlyFileDirective(t *testing.T) {
	index := NewMockSuppressIndex(t)
	index.EXPECT().HasDirectives().Return(true)
	index.EXPECT().IsFileSuppressed(tcFileA).Return(false)

	filter := NewSuppressFilter(index)
	result := models.CheckResult{
		NamingWarnings: []models.CheckArchWarningNaming{
			{PackageName: "helpers", FileAbsolutePath: tcFileA},
		},
	}
	filtered, suppressed := filter.Filter(result)
	assert.Equal(t, 0, suppressed, "line directive must not suppress a naming warning")
	assert.Len(t, filtered.NamingWarnings, 1, "naming warning must survive")
}

func TestSuppressFilter_DeepscanWarningByInjectionSite(t *testing.T) {
	index := NewMockSuppressIndex(t)
	index.EXPECT().HasDirectives().Return(true)
	index.EXPECT().IsFileSuppressed("/p/container.go").Return(false)
	index.EXPECT().IsLineSuppressed("/p/container.go", 15, "repository").Return(true)

	filter := NewSuppressFilter(index)
	result := models.CheckResult{
		DeepscanWarnings: []models.CheckArchWarningDeepscan{{
			Gate: models.DeepscanWarningGate{ComponentName: "repository", MethodName: "NewRepo"},
			Dependency: models.DeepscanWarningDependency{
				Injection: domain.NewReferenceSingleLine("/p/container.go", 15, 0),
			},
		}},
	}
	filtered, suppressed := filter.Filter(result)
	assert.Equal(t, 1, suppressed, "deepscan warning must be suppressed at the injection site")
	assert.Empty(t, filtered.DeepscanWarnings, "deepscan warning must be removed")
}

func TestRuleTarget_PlainImportNotRewritten(t *testing.T) {
	// A plain component name (no rule markers) must NOT be treated as a
	// rule target — the import path is used instead.
	_, ok := ruleTarget("alpha")
	assert.False(t, ok, "plain component name must not extract a rule target")

	name, ok := ruleTarget("alpha -> " + tcBetaNm + " (cycle: alpha -> " + tcBetaNm + " -> alpha)")
	assert.True(t, ok, "cycle rule target must be extracted")
	assert.Equal(t, tcBetaNm, name, "cycle rule target")

	name, ok = ruleTarget("alpha may not consume '" + tcBetaNm + "' (restricted API)")
	assert.True(t, ok, "visibility rule target must be extracted")
	assert.Equal(t, tcBetaNm, name, "visibility rule target")

	name, ok = ruleTarget("interface 'View' must live with its consumer '" + tcBetaNm + "' (declared in component 'gamma')")
	assert.True(t, ok, "placement rule target must be extracted")
	assert.Equal(t, tcBetaNm, name, "placement rule target")
}
