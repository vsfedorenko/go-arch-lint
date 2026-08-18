package checker

import (
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

// Fixture paths reused across filter tests (goconst).
const (
	tcFileA  = "/p/a.go"
	tcBetaNm = "beta" //nolint:goconst // fixture name (tcBeta taken by cycles fixtures)
)

// fakeSuppressIndex is a scriptable directive index for filter tests.
type fakeSuppressIndex struct {
	files      map[string]bool
	lines      map[string]map[int]map[string]bool
	hasDirList bool
}

func (f *fakeSuppressIndex) IsFileSuppressed(path string) bool {
	return f.files[path]
}

func (f *fakeSuppressIndex) IsLineSuppressed(path string, line int, target string) bool {
	targets, ok := f.lines[path][line]
	if !ok {
		return false
	}
	if targets == nil {
		return true
	}
	return targets[target]
}

func (f *fakeSuppressIndex) HasDirectives() bool {
	return f.hasDirList
}

func depWarning(component, file, importName string, line int) models.CheckArchWarningDependency {
	return models.CheckArchWarningDependency{
		ComponentName:      component,
		FileAbsolutePath:   file,
		ResolvedImportName: importName,
		Reference:          domain.NewReferenceSingleLine(file, line, 0),
	}
}

func TestSuppressFilter_NoDirectivesPassthrough(t *testing.T) {
	filter := NewSuppressFilter(&fakeSuppressIndex{})
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha", tcFileA, "example.com/p/beta", 3),
		},
	}
	filtered, suppressed := filter.Filter(result)
	if suppressed != 0 {
		t.Fatalf("no directives: suppressed must be 0, got %d", suppressed)
	}
	if len(filtered.DependencyWarnings) != 1 {
		t.Fatalf("warning must pass through untouched")
	}
}

func TestSuppressFilter_ImportViolationByLine(t *testing.T) {
	filter := NewSuppressFilter(&fakeSuppressIndex{
		hasDirList: true,
		lines: map[string]map[int]map[string]bool{
			tcFileA: {3: nil}, // any target
		},
	})
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha", tcFileA, "example.com/p/beta", 3),
			depWarning("alpha", tcFileA, "example.com/p/beta", 4),
		},
	}
	filtered, suppressed := filter.Filter(result)
	if suppressed != 1 {
		t.Fatalf("expected 1 suppressed, got %d", suppressed)
	}
	if len(filtered.DependencyWarnings) != 1 || filtered.DependencyWarnings[0].Reference.Line != 4 {
		t.Fatalf("wrong warning kept: %+v", filtered.DependencyWarnings)
	}
}

func TestSuppressFilter_ImportViolationByTargetFilter(t *testing.T) {
	filter := NewSuppressFilter(&fakeSuppressIndex{
		hasDirList: true,
		lines: map[string]map[int]map[string]bool{
			// exact import path filter
			tcFileA: {3: {"example.com/p/beta": true}},
		},
	})
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha", tcFileA, "example.com/p/beta", 3),
			depWarning("alpha", tcFileA, "example.com/p/gamma", 3),
		},
	}
	filtered, suppressed := filter.Filter(result)
	if suppressed != 1 {
		t.Fatalf("target filter must suppress only the matching import, got %d", suppressed)
	}
	if len(filtered.DependencyWarnings) != 1 || filtered.DependencyWarnings[0].ResolvedImportName != "example.com/p/gamma" {
		t.Fatalf("wrong warning kept: %+v", filtered.DependencyWarnings)
	}
}

func TestSuppressFilter_FileDirectiveSuppressesAllCategories(t *testing.T) {
	filter := NewSuppressFilter(&fakeSuppressIndex{
		hasDirList: true,
		files:      map[string]bool{tcFileA: true},
	})
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha", tcFileA, "example.com/p/beta", 3),
		},
		MatchWarnings: []models.CheckArchWarningMatch{
			{FileAbsolutePath: tcFileA},
		},
	}
	filtered, suppressed := filter.Filter(result)
	if suppressed != 2 {
		t.Fatalf("file directive must suppress every warning in the file, got %d", suppressed)
	}
	if len(filtered.DependencyWarnings) != 0 || len(filtered.MatchWarnings) != 0 {
		t.Fatalf("nothing must remain: %+v", filtered)
	}
}

func TestSuppressFilter_CycleRuleTarget(t *testing.T) {
	// Cycles embed the target in ComponentName: "alpha -> beta (cycle: ...)".
	// The ignore argument must match "beta", not the whole rule text.
	filter := NewSuppressFilter(&fakeSuppressIndex{
		hasDirList: true,
		lines: map[string]map[int]map[string]bool{
			tcFileA: {7: {tcBetaNm: true}},
		},
	})
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha -> beta (cycle: alpha -> beta -> alpha)", tcFileA, "example.com/p/beta", 7),
		},
	}
	filtered, suppressed := filter.Filter(result)
	if suppressed != 1 {
		t.Fatalf("cycle warning must be suppressed by its edge target, got %d", suppressed)
	}
	if len(filtered.DependencyWarnings) != 0 {
		t.Fatal("cycle warning must be removed")
	}
}

func TestSuppressFilter_TierRuleTarget(t *testing.T) {
	filter := NewSuppressFilter(&fakeSuppressIndex{
		hasDirList: true,
		lines: map[string]map[int]map[string]bool{
			tcFileA: {9: {tcBetaNm: true}},
		},
	})
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha (tier 'domain') -> beta (tier 'app') — upward dependency", tcFileA, "example.com/p/beta", 9),
		},
	}
	_, suppressed := filter.Filter(result)
	if suppressed != 1 {
		t.Fatalf("tier warning must be suppressed by its edge target, got %d", suppressed)
	}
}

func TestSuppressFilter_VisibilityRuleTarget(t *testing.T) {
	filter := NewSuppressFilter(&fakeSuppressIndex{
		hasDirList: true,
		lines: map[string]map[int]map[string]bool{
			tcFileA: {11: {tcBetaNm: true}},
		},
	})
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("alpha may not consume 'beta' (restricted API: visible to [gamma])", tcFileA, "example.com/p/beta", 11),
		},
	}
	_, suppressed := filter.Filter(result)
	if suppressed != 1 {
		t.Fatalf("visibility warning must be suppressed by quoted target, got %d", suppressed)
	}
}

func TestSuppressFilter_MatchWarningOnlyFileDirective(t *testing.T) {
	// Match warnings carry no line: a line directive must NOT suppress
	// them, only the file-level one does.
	filter := NewSuppressFilter(&fakeSuppressIndex{
		hasDirList: true,
		lines: map[string]map[int]map[string]bool{
			tcFileA: {1: nil},
		},
	})
	result := models.CheckResult{
		MatchWarnings: []models.CheckArchWarningMatch{
			{FileAbsolutePath: tcFileA},
		},
	}
	filtered, suppressed := filter.Filter(result)
	if suppressed != 0 {
		t.Fatal("line directive must not suppress a match warning")
	}
	if len(filtered.MatchWarnings) != 1 {
		t.Fatal("match warning must survive")
	}
}

func TestSuppressFilter_NamingWarningOnlyFileDirective(t *testing.T) {
	filter := NewSuppressFilter(&fakeSuppressIndex{
		hasDirList: true,
		lines: map[string]map[int]map[string]bool{
			tcFileA: {1: nil},
		},
	})
	result := models.CheckResult{
		NamingWarnings: []models.CheckArchWarningNaming{
			{PackageName: "helpers", FileAbsolutePath: tcFileA},
		},
	}
	filtered, suppressed := filter.Filter(result)
	if suppressed != 0 {
		t.Fatal("line directive must not suppress a naming warning")
	}
	if len(filtered.NamingWarnings) != 1 {
		t.Fatal("naming warning must survive")
	}
}

func TestSuppressFilter_DeepscanWarningByInjectionSite(t *testing.T) {
	filter := NewSuppressFilter(&fakeSuppressIndex{
		hasDirList: true,
		lines: map[string]map[int]map[string]bool{
			"/p/container.go": {15: {"repository": true}},
		},
	})
	result := models.CheckResult{
		DeepscanWarnings: []models.CheckArchWarningDeepscan{{
			Gate: models.DeepscanWarningGate{ComponentName: "repository", MethodName: "NewRepo"},
			Dependency: models.DeepscanWarningDependency{
				Injection: domain.NewReferenceSingleLine("/p/container.go", 15, 0),
			},
		}},
	}
	filtered, suppressed := filter.Filter(result)
	if suppressed != 1 {
		t.Fatalf("deepscan warning must be suppressed at the injection site, got %d", suppressed)
	}
	if len(filtered.DeepscanWarnings) != 0 {
		t.Fatal("deepscan warning must be removed")
	}
}

func TestRuleTarget_PlainImportNotRewritten(t *testing.T) {
	// A plain component name (no rule markers) must NOT be treated as a
	// rule target — the import path is used instead.
	if _, ok := ruleTarget("alpha"); ok {
		t.Fatal("plain component name must not extract a rule target")
	}
	if name, ok := ruleTarget("alpha -> " + tcBetaNm + " (cycle: alpha -> " + tcBetaNm + " -> alpha)"); !ok || name != tcBetaNm {
		t.Fatalf("cycle rule target = %q, %v; want %s, true", name, ok, tcBetaNm)
	}
	if name, ok := ruleTarget("alpha may not consume '" + tcBetaNm + "' (restricted API)"); !ok || name != tcBetaNm {
		t.Fatalf("visibility rule target = %q, %v; want %s, true", name, ok, tcBetaNm)
	}
	if name, ok := ruleTarget("interface 'View' must live with its consumer '" + tcBetaNm + "' (declared in component 'gamma')"); !ok || name != tcBetaNm {
		t.Fatalf("placement rule target = %q, %v; want %s, true", name, ok, tcBetaNm)
	}
}
