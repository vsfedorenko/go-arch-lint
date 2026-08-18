package models

import "slices"

// htmlToolName mirrors the SARIF driver name so every machine format
// identifies the same tool.
const htmlToolName = "go-arch-lint"

// HTMLReport is the standalone HTML report go-arch-lint emits for
// `--format html`. The document is self-contained (inline CSS, no scripts,
// no external assets): CI pipelines archive it as an artifact and humans
// open it directly in a browser.
type HTMLReport struct {
	ToolName    string
	ToolVersion string
	ModuleName  string
	Total       int
	ByType      []HTMLCountByType
	Rows        []HTMLRow
	// OmittedCount and SuppressedCount carry the display-cap contract:
	// omitted violations exist but were not rendered (max-warnings cap);
	// suppressed ones were silenced by //go-arch-lint:ignore directives.
	OmittedCount    int
	SuppressedCount int
}

// HTMLCountByType counts violations per rule class for the summary cards.
type HTMLCountByType struct {
	Type  string
	Label string
	Count int
}

// HTMLRow is one violation as a table row.
type HTMLRow struct {
	Type       string
	TypeLabel  string
	File       string
	Line       int
	Column     int
	Component  string
	Dependency string
	Rule       string
	Details    string
}

// htmlTypeLabel renders a human label for a violation type; unknown types
// surface raw so no violation is silently dropped.
func htmlTypeLabel(violationType string) string {
	switch violationType {
	case violationTypeDependency:
		return "Dependency"
	case violationTypeMatch:
		return "Not matched"
	case violationTypeDeepscan:
		return "DeepScan"
	case violationTypeNaming:
		return "Naming"
	default:
		return violationType
	}
}

// ToHTMLReport converts the check output to an HTMLReport ready for
// rendering. driverVersion is reported in the header (pass the build
// version, or "dev"). File paths are normalized to project-relative form
// (leading slash stripped) like the other machine formats.
func (out CmdCheckOut) ToHTMLReport(driverVersion string) HTMLReport {
	violations := out.ToViolations()

	// Stable order mirroring sarifRuleOrder: dependency, match, deepscan,
	// naming — then anything unexpected so unknown types still surface.
	orderedTypes := []string{
		violationTypeDependency, violationTypeMatch,
		violationTypeDeepscan, violationTypeNaming,
	}
	counts := map[string]int{}
	for _, v := range violations {
		counts[v.Type]++
		if !slices.Contains(orderedTypes, v.Type) {
			orderedTypes = append(orderedTypes, v.Type)
		}
	}

	byType := make([]HTMLCountByType, 0, len(orderedTypes))
	for _, t := range orderedTypes {
		byType = append(byType, HTMLCountByType{
			Type:  t,
			Label: htmlTypeLabel(t),
			Count: counts[t],
		})
	}

	rows := make([]HTMLRow, 0, len(violations))
	for _, v := range violations {
		rows = append(rows, HTMLRow{
			Type:       v.Type,
			TypeLabel:  htmlTypeLabel(v.Type),
			File:       RelativeFilePath(v.File),
			Line:       v.Line,
			Column:     v.Column,
			Component:  v.Component,
			Dependency: v.Dependency,
			Rule:       v.Rule,
			Details:    v.Details,
		})
	}

	return HTMLReport{
		ToolName:        htmlToolName,
		ToolVersion:     driverVersion,
		ModuleName:      out.ModuleName,
		Total:           len(violations),
		ByType:          byType,
		Rows:            rows,
		OmittedCount:    out.OmittedCount,
		SuppressedCount: out.SuppressedCount,
	}
}
