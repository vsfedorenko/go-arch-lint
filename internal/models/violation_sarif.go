package models

import "fmt"

// SARIFLog is the minimal SARIF (Static Analysis Results Interchange
// Format, v2.1.0) log go-arch-lint emits for `--format sarif`. Only the
// fields GitHub Code Scanning and other SARIF consumers require are
// populated; the schema URI pins the version for validators.
type SARIFLog struct {
	Version string     `json:"version"` // always "2.1.0"
	Schema  string     `json:"$schema"` // SARIF 2.1.0 schema URI
	Runs    []SARIFRun `json:"runs"`    // one run per invocation
}

// SARIFRun groups the results of a single go-arch-lint invocation.
type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

// SARIFTool identifies the analyzer that produced the results.
type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

// SARIFDriver describes go-arch-lint itself.
type SARIFDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []SARIFRule `json:"rules"`
}

// SARIFRule is one distinct rule class the driver can report.
type SARIFRule struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Default SARIFRuleDefault `json:"default"`
}

// SARIFRuleDefault carries the default severity of a rule.
type SARIFRuleDefault struct {
	Level string `json:"level"`
}

// SARIFResult is one architecture violation as a SARIF result.
type SARIFResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   SARIFMessage    `json:"message"`
	Locations []SARIFLocation `json:"locations"`
}

// SARIFMessage is the human-readable description of a result.
type SARIFMessage struct {
	Text string `json:"text"`
}

// SARIFLocation points at the offending file and line.
type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"`
}

// SARIFPhysicalLocation is the file/region coordinates of a violation.
type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"`
	Region           SARIFRegion           `json:"region"`
}

// SARIFArtifactLocation names the file containing the violation.
type SARIFArtifactLocation struct {
	URI string `json:"uri"`
}

// SARIFRegion is the line/column region of a violation.
type SARIFRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn,omitempty"`
}

// SARIF severity levels (subset applicable to a linter).
const (
	SARIFLevelError = "error"
	SARIFLevelNote  = "note"
)

// SARIF rule identifiers, derived from the violation types.
const (
	SARIFRuleDependency = "GA001"
	SARIFRuleMatch      = "GA002"
	SARIFRuleDeepScan   = "GA003"
	SARIFRuleNaming     = "GA004"
)

// sarifRuleMeta describes one rule class: ID, display name, default level.
type sarifRuleMeta struct {
	id    string
	name  string
	level string
}

// One sarifRuleMeta per rule class — single source of truth for rule IDs,
// display names and default levels.
var (
	sarifMetaDependency = sarifRuleMeta{SARIFRuleDependency, "ArchDependency", SARIFLevelError}
	sarifMetaMatch      = sarifRuleMeta{SARIFRuleMatch, "ArchMatch", SARIFLevelNote}
	sarifMetaDeepScan   = sarifRuleMeta{SARIFRuleDeepScan, "ArchDeepScan", SARIFLevelError}
	sarifMetaNaming     = sarifRuleMeta{SARIFRuleNaming, "ArchNaming", SARIFLevelError}
)

// sarifRuleOrder defines the driver rules array order.
var sarifRuleOrder = []sarifRuleMeta{
	sarifMetaDependency,
	sarifMetaMatch,
	sarifMetaDeepScan,
	sarifMetaNaming,
}

// sarifRuleByType maps a models.Violation type to its SARIF rule metadata.
var sarifRuleByType = map[string]sarifRuleMeta{
	violationTypeDependency: sarifMetaDependency,
	violationTypeMatch:      sarifMetaMatch,
	violationTypeDeepscan:   sarifMetaDeepScan,
	violationTypeNaming:     sarifMetaNaming,
}

// ToSARIF converts the check output to a SARIF 2.1.0 log. driverVersion
// is reported in tool.driver.version (pass the build version, or "dev").
func (out CmdCheckOut) ToSARIF(driverVersion string) SARIFLog {
	rules := make([]SARIFRule, 0, len(sarifRuleOrder))
	for _, meta := range sarifRuleOrder {
		rules = append(rules, SARIFRule{
			ID:      meta.id,
			Name:    meta.name,
			Default: SARIFRuleDefault{Level: meta.level},
		})
	}

	violations := out.ToViolations()
	results := make([]SARIFResult, 0, len(violations))
	for _, v := range violations {
		meta, ok := sarifRuleByType[v.Type]
		if !ok {
			// Unknown types still surface as results; fall back to a
			// generic note so no violation is silently dropped.
			meta = sarifMetaMatch
		}

		results = append(results, SARIFResult{
			RuleID: meta.id,
			Level:  meta.level,
			Message: SARIFMessage{
				Text: sarifResultMessage(v),
			},
			Locations: []SARIFLocation{{
				PhysicalLocation: SARIFPhysicalLocation{
					ArtifactLocation: SARIFArtifactLocation{
						URI: sarifArtifactURI(v.File),
					},
					Region: SARIFRegion{
						StartLine:   sarifStartLine(v.Line),
						StartColumn: v.Column,
					},
				},
			}},
		})
	}

	return SARIFLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []SARIFRun{{
			Tool: SARIFTool{
				Driver: SARIFDriver{
					Name:           "go-arch-lint",
					Version:        driverVersion,
					InformationURI: "https://github.com/vsfedorenko/go-arch-lint",
					Rules:          rules,
				},
			},
			Results: results,
		}},
	}
}

// sarifResultMessage renders the violation as a single human-readable
// text message, appending machine fields (component/dependency) that do
// not map to dedicated SARIF fields.
func sarifResultMessage(v Violation) string {
	msg := v.Rule
	if v.Component != "" {
		msg += fmt.Sprintf(" (component: %s", v.Component)
		if v.Dependency != "" {
			msg += fmt.Sprintf(", dependency: %s", v.Dependency)
		}
		msg += ")"
	}
	if v.Details != "" {
		msg += " — " + v.Details
	}
	return msg
}

// sarifArtifactURI strips the leading slash go-arch-lint uses in
// project-relative paths, producing a clean relative URI.
func sarifArtifactURI(file string) string {
	return RelativeFilePath(file)
}

// RelativeFilePath strips the leading slash go-arch-lint uses in
// project-relative paths. Violation files are rooted ("/internal/...");
// tool integrations (SARIF artifact URIs, GitHub Actions annotation
// properties) expect workspace-relative paths without it.
func RelativeFilePath(file string) string {
	if len(file) > 1 && file[0] == '/' {
		return file[1:]
	}
	return file
}

// sarifStartLine clamps unknown (0) lines to 1: SARIF regions require a
// positive startLine.
func sarifStartLine(line int) int {
	if line < 1 {
		return 1
	}
	return line
}
