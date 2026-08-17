package models

import "fmt"

// Format controls how check results are rendered to stdout.
type Format = string

const (
	FormatDefault Format = "default" // resolved to FormatText
	FormatText    Format = "text"    // human-readable ASCII (existing templates)
	FormatJSON    Format = "json"    // flat JSON array of violations
	FormatSARIF   Format = "sarif"   // SARIF 2.1.0 log for code-scanning tools
)

// FormatValues lists every accepted --format value.
var FormatValues = []string{FormatText, FormatJSON, FormatSARIF}

// Violation is a single flattened architecture violation, intended for
// machine consumption (CI pipelines, editor integrations, reporting tools).
// It unifies the three internal warning kinds (dependency, match, deepscan)
// behind one stable schema.
type Violation struct {
	Type       string `json:"type"`                 // "dependency" | "match" | "deepscan"
	File       string `json:"file"`                 // project-relative path to the offending file
	Line       int    `json:"line"`                 // 1-based line number (0 when unknown)
	Column     int    `json:"column,omitempty"`     // 1-based column offset (0 when unknown)
	Component  string `json:"component,omitempty"`  // source component that owns the file
	Dependency string `json:"dependency,omitempty"` // target component or import that was used
	Package    string `json:"package,omitempty"`    // resolved import path (dependency violations)
	Rule       string `json:"rule"`                 // human-readable description of the violated rule
	Details    string `json:"details,omitempty"`    // optional extra context (e.g. injection AST)
}

const (
	violationTypeDependency = "dependency"
	violationTypeMatch      = "match"
	violationTypeDeepscan   = "deepscan"
	violationTypeNaming     = "naming"
)

// ToViolations flattens a CmdCheckOut into a sorted slice of Violation.
// The order is stable: dependency, then match, then deepscan warnings,
// each group sorted by file then line so that output is deterministic and
// diff-friendly in CI.
func (out CmdCheckOut) ToViolations() []Violation {
	violations := make([]Violation, 0,
		len(out.ArchWarningsDependency)+len(out.ArchWarningsMatch)+len(out.ArchWarningsDeepScan))

	for _, w := range out.ArchWarningsDependency {
		violations = append(violations, Violation{
			Type:       violationTypeDependency,
			File:       w.FileRelativePath,
			Line:       w.Reference.Line,
			Column:     w.Reference.Column,
			Component:  w.ComponentName,
			Dependency: w.ResolvedImportName,
			Package:    w.ResolvedImportName,
			Rule: fmt.Sprintf("component %q may not depend on %q",
				w.ComponentName, w.ResolvedImportName),
		})
	}

	for _, w := range out.ArchWarningsNaming {
		violations = append(violations, Violation{
			Type:    violationTypeNaming,
			File:    w.FileRelativePath,
			Package: w.PackageName,
			Rule: fmt.Sprintf("package name %q is forbidden (package %s, %d file(s))",
				w.PackageName, w.PackagePath, w.FilesCount),
		})
	}

	for _, w := range out.ArchWarningsMatch {
		violations = append(violations, Violation{
			Type: violationTypeMatch,
			File: w.FileRelativePath,
			Rule: "file is not attached to any component",
		})
	}

	for _, w := range out.ArchWarningsDeepScan {
		violations = append(violations, Violation{
			Type:       violationTypeDeepscan,
			File:       w.Dependency.InjectionPath,
			Line:       w.Dependency.Injection.Line,
			Component:  w.Dependency.ComponentName,
			Dependency: w.Gate.ComponentName,
			Rule: fmt.Sprintf("component %q may not depend on %q via dependency injection",
				w.Dependency.ComponentName, w.Gate.ComponentName),
			Details: w.Dependency.InjectionAST,
		})
	}

	return violations
}
