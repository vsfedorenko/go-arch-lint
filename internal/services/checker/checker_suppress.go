package checker

import (
	"strings"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
)

// SuppressIndex is the port the composite uses to apply source-level
// //go-arch-lint:ignore directives to aggregated check results.
type SuppressIndex interface {
	IsFileSuppressed(path string) bool
	IsLineSuppressed(path string, line int, dependencyTarget string) bool
	HasDirectives() bool
}

// SuppressFilter drops warnings matched by suppression directives and
// counts what it removed, so suppressed debt stays visible in output.
type SuppressFilter struct {
	index SuppressIndex
}

// NewSuppressFilter applies the given directive index.
func NewSuppressFilter(index SuppressIndex) *SuppressFilter {
	return &SuppressFilter{index: index}
}

// Filter removes suppressed warnings from result and returns the
// filtered result plus the count of suppressed violations.
func (f *SuppressFilter) Filter(result models.CheckResult) (models.CheckResult, int) {
	if !f.index.HasDirectives() {
		return result, 0
	}

	suppressed := 0

	keptDeps := make([]models.CheckArchWarningDependency, 0, len(result.DependencyWarnings))
	for _, w := range result.DependencyWarnings {
		if f.suppressedDependency(w) {
			suppressed++
			continue
		}

		keptDeps = append(keptDeps, w)
	}

	keptMatch := make([]models.CheckArchWarningMatch, 0, len(result.MatchWarnings))
	for _, w := range result.MatchWarnings {
		// Match warnings have no line: only file-level directives apply.
		if f.index.IsFileSuppressed(w.FileAbsolutePath) {
			suppressed++
			continue
		}

		keptMatch = append(keptMatch, w)
	}

	keptNaming := make([]models.CheckArchWarningNaming, 0, len(result.NamingWarnings))
	for _, w := range result.NamingWarnings {
		// Naming warnings are per-package with a "first file" witness:
		// only file-level directives apply.
		if f.index.IsFileSuppressed(w.FileAbsolutePath) {
			suppressed++
			continue
		}

		keptNaming = append(keptNaming, w)
	}

	keptDeepscan := make([]models.CheckArchWarningDeepscan, 0, len(result.DeepscanWarnings))
	for _, w := range result.DeepscanWarnings {
		if f.suppressedDeepscan(w) {
			suppressed++
			continue
		}

		keptDeepscan = append(keptDeepscan, w)
	}

	return models.CheckResult{
		DependencyWarnings: keptDeps,
		MatchWarnings:      keptMatch,
		DeepscanWarnings:   keptDeepscan,
		NamingWarnings:     keptNaming,
	}, suppressed
}

// suppressedDependency matches a dependency warning against directives
// by its witness position (absolute path + line) and the dependency
// target (import path or rule-embedded component name).
func (f *SuppressFilter) suppressedDependency(w models.CheckArchWarningDependency) bool {
	if f.index.IsFileSuppressed(w.FileAbsolutePath) {
		return true
	}

	return f.index.IsLineSuppressed(w.FileAbsolutePath, w.Reference.Line, dependencyTarget(w))
}

// suppressedDeepscan matches an injection warning: the witness is the
// injection site (file + line); the target is the gate component name.
func (f *SuppressFilter) suppressedDeepscan(w models.CheckArchWarningDeepscan) bool {
	if f.index.IsFileSuppressed(w.Dependency.Injection.File) {
		return true
	}

	return f.index.IsLineSuppressed(
		w.Dependency.Injection.File,
		w.Dependency.Injection.Line,
		w.Gate.ComponentName,
	)
}

// dependencyTarget returns the name a //go-arch-lint:ignore argument
// must match for this warning. Plain import violations use the import
// path; graph-derived rules (cycles, tiers, visibility, interface
// placement) embed the target component in the rule text and match by
// component name.
func dependencyTarget(w models.CheckArchWarningDependency) string {
	if name, ok := ruleTarget(w.ComponentName); ok {
		return name
	}

	return w.ResolvedImportName
}

// ruleTarget extracts the dependency target from rule texts that embed
// it in the human-readable component-name field.
func ruleTarget(componentName string) (string, bool) {
	// Cycles: "alpha -> beta (cycle: alpha -> beta -> alpha)"
	if name, ok := edgeTarget(componentName, " -> ", " (cycle"); ok {
		return name, true
	}

	// Tiers: "alpha (tier 'domain') -> beta (tier 'app') — ..."
	if name, ok := edgeTarget(componentName, ") -> ", " (tier"); ok {
		return name, true
	}

	// Visibility: "alpha may not consume 'beta' (restricted API: ...)"
	if name, ok := quotedAfter(componentName, " may not consume '"); ok {
		return name, true
	}

	// Interface placement: "interface 'X' must live with its consumer
	// 'beta' (declared in component 'gamma')"
	if name, ok := quotedAfter(componentName, "consumer '"); ok {
		return name, true
	}

	return "", false
}

// edgeTarget extracts the target of an "A ...-> B" rule string: the
// segment after arrow up to end marker.
func edgeTarget(rule, arrow, endMarker string) (string, bool) {
	arrowAt := strings.Index(rule, arrow)
	if arrowAt < 0 {
		return "", false
	}

	rest := rule[arrowAt+len(arrow):]

	endAt := strings.Index(rest, endMarker)
	if endAt < 0 {
		return "", false
	}

	return rest[:endAt], true
}

// quotedAfter returns the single-quoted value following marker.
func quotedAfter(text, marker string) (string, bool) {
	start := strings.Index(text, marker)
	if start < 0 {
		return "", false
	}

	rest := text[start+len(marker):]

	end := strings.Index(rest, "'")
	if end < 0 {
		return "", false
	}

	return rest[:end], true
}
