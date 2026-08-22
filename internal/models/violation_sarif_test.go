package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

// Fixture component names reused across the SARIF tests (goconst-clean).
const (
	sarifFixtureComponent = "handler"
	sarifFixtureTarget    = "repository"
)

func TestToSARIF_FullViolationSurface(t *testing.T) {
	out := CmdCheckOut{
		ModuleName: "github.com/x/proj",
		ArchWarningsDependency: []CheckArchWarningDependency{
			{
				ComponentName:      sarifFixtureComponent,
				FileRelativePath:   "/internal/handler/user.go",
				ResolvedImportName: "github.com/x/proj/internal/repository",
				Reference:          domain.NewReferenceSingleLine("/internal/handler/user.go", 10, 2),
			},
		},
		ArchWarningsMatch: []CheckArchWarningMatch{
			{FileRelativePath: "/internal/orphan/x.go"},
		},
		ArchWarningsNaming: []CheckArchWarningNaming{
			{
				PackageName:      "utils",
				PackagePath:      "/internal/utils",
				FileRelativePath: "/internal/utils/a.go",
				FilesCount:       3,
			},
		},
	}

	log := out.ToSARIF("v1.2.3")

	// Envelope: version, schema, exactly one run.
	assert.Equal(t, "2.1.0", log.Version)
	assert.Contains(t, log.Schema, "sarif-2.1.0")
	assert.Len(t, log.Runs, 1)

	run := log.Runs[0]

	// Driver identity.
	assert.Equal(t, "go-arch-lint", run.Tool.Driver.Name)
	assert.Equal(t, "v1.2.3", run.Tool.Driver.Version)
	assert.NotEmpty(t, run.Tool.Driver.InformationURI)

	// All four rule classes are declared up front.
	assert.Len(t, run.Tool.Driver.Rules, 4)
	ruleIDs := make([]string, 0, len(run.Tool.Driver.Rules))
	for _, rule := range run.Tool.Driver.Rules {
		ruleIDs = append(ruleIDs, rule.ID)
	}
	assert.ElementsMatch(t, []string{SARIFRuleDependency, SARIFRuleMatch, SARIFRuleDeepScan, SARIFRuleNaming}, ruleIDs)

	// Results: one per violation, in ToViolations order.
	assert.Len(t, run.Results, 3)

	dep := run.Results[0]
	assert.Equal(t, SARIFRuleDependency, dep.RuleID)
	assert.Equal(t, SARIFLevelError, dep.Level)
	assert.Equal(t, "internal/handler/user.go", dep.Locations[0].PhysicalLocation.ArtifactLocation.URI)
	assert.Equal(t, 10, dep.Locations[0].PhysicalLocation.Region.StartLine)
	assert.Equal(t, 2, dep.Locations[0].PhysicalLocation.Region.StartColumn)
	assert.Contains(t, dep.Message.Text, sarifFixtureComponent)
	assert.Contains(t, dep.Message.Text, sarifFixtureTarget)

	// match: note severity, leading slash stripped, line clamped to 1.
	match := run.Results[2]
	assert.Equal(t, SARIFRuleMatch, match.RuleID)
	assert.Equal(t, SARIFLevelNote, match.Level)
	assert.Equal(t, "internal/orphan/x.go", match.Locations[0].PhysicalLocation.ArtifactLocation.URI)
	assert.Equal(t, 1, match.Locations[0].PhysicalLocation.Region.StartLine)

	// naming: rule text carries the banned package name (comes before match
	// in ToViolations order: dependency → naming → match → deepscan).
	naming := run.Results[1]
	assert.Equal(t, SARIFRuleNaming, naming.RuleID)
	assert.Contains(t, naming.Message.Text, "utils")
}

func TestToSARIF_EmptyResults(t *testing.T) {
	log := CmdCheckOut{}.ToSARIF("dev")

	assert.Len(t, log.Runs, 1)
	// Rules are declared even with no results (stable tool metadata).
	assert.Len(t, log.Runs[0].Tool.Driver.Rules, 4)
	// Results must marshal as [] not null.
	assert.Empty(t, log.Runs[0].Results)

	raw, err := json.Marshal(log)
	assert.NoError(t, err)
	assert.NotContains(t, string(raw), "null")
}

func TestToSARIF_DeepScanDetails(t *testing.T) {
	out := CmdCheckOut{
		ArchWarningsDeepScan: []CheckArchWarningDeepscan{
			{
				Dependency: DeepscanWarningDependency{
					ComponentName: "svc",
					InjectionPath: "/internal/service/order.go",
					Injection:     domain.NewReferenceSingleLine("/internal/service/order.go", 42, 1),
					InjectionAST:  "NewOrderRepo(repo Repository)",
				},
				Gate: DeepscanWarningGate{
					ComponentName: sarifFixtureTarget,
				},
			},
		},
	}

	log := out.ToSARIF("dev")
	assert.Len(t, log.Runs[0].Results, 1)

	res := log.Runs[0].Results[0]
	assert.Equal(t, SARIFRuleDeepScan, res.RuleID)
	assert.Equal(t, 42, res.Locations[0].PhysicalLocation.Region.StartLine)
	assert.Contains(t, res.Message.Text, "dependency injection")
	// details are appended to the message so the AST context survives.
	assert.Contains(t, res.Message.Text, "NewOrderRepo(repo Repository)")
}
