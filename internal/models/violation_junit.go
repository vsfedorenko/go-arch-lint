package models

import (
	"encoding/xml"
	"strconv"
	"strings"
)

// JUnitXML is the JUnit-style test-suite report go-arch-lint emits for
// `--format junit`. The schema follows the de-facto standard popularized
// by the JUnit CI ecosystem (pytest / surefire / gitlab JUnit report
// ingestion): one testsuite per invocation, one testcase per violation,
// failures carry the human-readable rule text. CI platforms (GitLab CI,
// Jenkins JUnit plugin, Buildkite, Azure Pipelines) ingest this to render
// violation counts and trend graphs without custom parsers.
type JUnitXML struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Time     string       `xml:"time,attr,omitempty"`
	Suites   []JUnitSuite `xml:"testsuite"`
}

// JUnitSuite is a single go-arch-lint check invocation.
type JUnitSuite struct {
	XMLName  xml.Name    `xml:"testsuite"`
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Cases    []JUnitCase `xml:"testcase"`
}

// JUnitCase is one architecture violation as a failed testcase. The
// classname mirrors the rule class (GA001..GA004) so dashboards can group
// by violation type; name carries file:line for quick navigation.
type JUnitCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Classname string        `xml:"classname,attr"`
	Name      string        `xml:"name,attr"`
	Time      string        `xml:"time,attr,omitempty"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
}

// JUnitFailure carries the violation message and the rule id as type.
type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

// junitRuleIDs maps a violation type to the shared SARIF rule identifiers
// (GA001..GA004) — one numbering space across output formats.
var junitRuleByType = map[string]string{
	violationTypeDependency: SARIFRuleDependency,
	violationTypeMatch:      SARIFRuleMatch,
	violationTypeDeepscan:   SARIFRuleDeepScan,
	violationTypeNaming:     SARIFRuleNaming,
}

// junitToolName is the report/suite name — mirrors the SARIF driver name.
const junitToolName = "go-arch-lint"

// ToJUnitXML converts the check output to a JUnit-style XML report. The
// suites-level counters aggregate the per-suite numbers; violations map
// one-to-one to failed testcases. A clean check produces a report with
// tests=1 failures=0 and a single passed testcase ("arch-check"), so
// dashboards that require at least one testcase keep rendering.
func (out CmdCheckOut) ToJUnitXML() JUnitXML {
	violations := out.ToViolations()

	cases := make([]JUnitCase, 0, len(violations)+1)
	for _, v := range violations {
		ruleID := junitRuleByType[v.Type]
		if ruleID == "" {
			// Unknown types still surface; fall back to the generic
			// rule id so no violation is silently dropped.
			ruleID = SARIFRuleMatch
		}

		cases = append(cases, JUnitCase{
			Classname: junitToolName + "." + ruleID,
			Name:      junitCaseName(v),
			Failure: &JUnitFailure{
				Message: v.Rule,
				Type:    ruleID,
				Content: junitFailureContent(v),
			},
		})
	}

	// Clean project: one green testcase so report consumers that expect
	// at least one case do not render an empty suite.
	if len(cases) == 0 {
		cases = append(cases, JUnitCase{
			Classname: junitToolName,
			Name:      "arch-check",
		})
	}

	suite := JUnitSuite{
		Name:     junitToolName + " check",
		Tests:    len(cases),
		Failures: len(violations),
		Cases:    cases,
	}

	return JUnitXML{
		Name:     junitToolName,
		Tests:    suite.Tests,
		Failures: suite.Failures,
		Suites:   []JUnitSuite{suite},
	}
}

// junitCaseName renders the violation location as "file:line", the
// convention dashboards display in the testcase column.
func junitCaseName(v Violation) string {
	file := strings.TrimPrefix(v.File, "/")
	if v.Line > 0 {
		return file + ":" + strconv.Itoa(v.Line)
	}
	return file
}

// junitFailureContent renders the multi-line failure body: the rule text
// plus the machine fields (component/dependency) that do not fit into
// the single-line message attribute.
func junitFailureContent(v Violation) string {
	content := v.Rule
	if v.Component != "" {
		content += "\ncomponent: " + v.Component
	}
	if v.Dependency != "" {
		content += "\ndependency: " + v.Dependency
	}
	if v.Details != "" {
		content += "\n" + v.Details
	}
	return content
}
