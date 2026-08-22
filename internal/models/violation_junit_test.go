package models

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

// Fixture literals reused across the JUnit tests (goconst-clean; shared
// with the SARIF suite where the names coincide).
const (
	junitFixtureModule     = "github.com/x/proj"
	junitFixtureComponent  = "handler"
	junitFixtureTarget     = "repository"
	junitFixtureDepImport  = junitFixtureModule + "/internal/repository"
	junitFixtureDepFile    = "/internal/handler/user.go"
	junitFixtureOrphanFile = "/internal/orphan/x.go"
	junitFixtureNamingPkg  = "utils"
	junitFixtureNamingPath = "/internal/utils"
	junitFixtureNamingFile = "/internal/utils/a.go"
)

func TestToJUnitXML_FullViolationSurface(t *testing.T) {
	out := CmdCheckOut{
		ModuleName: junitFixtureModule,
		ArchWarningsDependency: []CheckArchWarningDependency{
			{
				ComponentName:      junitFixtureComponent,
				FileRelativePath:   junitFixtureDepFile,
				ResolvedImportName: junitFixtureDepImport,
				Reference:          domain.NewReferenceSingleLine("/internal/handler/user.go", 10, 2),
			},
		},
		ArchWarningsMatch: []CheckArchWarningMatch{
			{FileRelativePath: junitFixtureOrphanFile},
		},
		ArchWarningsNaming: []CheckArchWarningNaming{
			{
				PackageName:      junitFixtureNamingPkg,
				PackagePath:      junitFixtureNamingPath,
				FileRelativePath: junitFixtureNamingFile,
				FilesCount:       3,
			},
		},
	}

	report := out.ToJUnitXML()

	// Envelope: one suite, counters match the violation count.
	assert.Len(t, report.Suites, 1)
	suite := report.Suites[0]
	assert.Equal(t, 3, report.Tests)
	assert.Equal(t, 3, report.Failures)
	assert.Equal(t, suite.Tests, report.Tests)
	assert.Equal(t, suite.Failures, report.Failures)

	// One failed testcase per violation, in ToViolations order
	// (dependency → naming → match).
	assert.Len(t, suite.Cases, 3)

	dep := suite.Cases[0]
	assert.Equal(t, "go-arch-lint."+SARIFRuleDependency, dep.Classname)
	assert.Equal(t, "internal/handler/user.go:10", dep.Name)
	assert.NotNil(t, dep.Failure)
	assert.Equal(t, SARIFRuleDependency, dep.Failure.Type)
	assert.Contains(t, dep.Failure.Message, junitFixtureComponent)
	assert.Contains(t, dep.Failure.Message, junitFixtureTarget)
	// machine fields survive in the failure body
	assert.Contains(t, dep.Failure.Content, "component: "+junitFixtureComponent)
	assert.Contains(t, dep.Failure.Content, "dependency: "+junitFixtureDepImport)

	naming := suite.Cases[1]
	assert.Equal(t, "go-arch-lint."+SARIFRuleNaming, naming.Classname)
	assert.Contains(t, naming.Failure.Message, junitFixtureNamingPkg)

	// match: leading slash stripped, no line → bare file name.
	match := suite.Cases[2]
	assert.Equal(t, "go-arch-lint."+SARIFRuleMatch, match.Classname)
	assert.Equal(t, "internal/orphan/x.go", match.Name)
}

func TestToJUnitXML_EmptyResults(t *testing.T) {
	report := CmdCheckOut{}.ToJUnitXML()

	assert.Len(t, report.Suites, 1)
	assert.Equal(t, 0, report.Failures)
	// clean project: one green testcase so dashboards keep rendering
	assert.Len(t, report.Suites[0].Cases, 1)
	assert.Equal(t, "arch-check", report.Suites[0].Cases[0].Name)
	assert.Nil(t, report.Suites[0].Cases[0].Failure)

	raw, err := xml.Marshal(report)
	assert.NoError(t, err)
	assert.NotContains(t, string(raw), "null")
}

func TestToJUnitXML_DeepScanDetails(t *testing.T) {
	out := CmdCheckOut{
		ArchWarningsDeepScan: []CheckArchWarningDeepscan{
			{
				Dependency: DeepscanWarningDependency{
					ComponentName: "svc",
					InjectionPath: "/internal/service/order.go",
					Injection:     domain.NewReferenceSingleLine("/internal/service/order.go", 42, 1),
					InjectionAST:  "NewOrderRepo(repo Repository)",
				},
				Gate: DeepscanWarningGate{ComponentName: junitFixtureTarget},
			},
		},
	}

	report := out.ToJUnitXML()
	assert.Len(t, report.Suites[0].Cases, 1)

	res := report.Suites[0].Cases[0]
	assert.Equal(t, "go-arch-lint."+SARIFRuleDeepScan, res.Classname)
	assert.Equal(t, "internal/service/order.go:42", res.Name)
	assert.Contains(t, res.Failure.Message, "dependency injection")
	// AST context survives in the failure body
	assert.Contains(t, res.Failure.Content, "NewOrderRepo(repo Repository)")
}

func TestToJUnitXML_XMLEscaping(t *testing.T) {
	out := CmdCheckOut{
		ArchWarningsDependency: []CheckArchWarningDependency{
			{
				ComponentName:      "a<b>&c",
				FileRelativePath:   "/internal/x/y.go",
				ResolvedImportName: "github.com/x/proj/internal/z",
				Reference:          domain.NewReferenceSingleLine("/internal/x/y.go", 1, 1),
			},
		},
	}

	raw, err := xml.Marshal(out.ToJUnitXML())
	assert.NoError(t, err)
	doc := string(raw)

	// angle brackets and ampersands must be escaped in attributes and text
	assert.Contains(t, doc, "a&lt;b&gt;&amp;c")
	assert.NotContains(t, doc, "a<b>")
}
