package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

func TestToViolations_Empty(t *testing.T) {
	out := CmdCheckOut{}
	violations := out.ToViolations()

	assert.NotNil(t, violations)
	assert.Len(t, violations, 0)
}

func TestToViolations_DependencyWarnings(t *testing.T) {
	out := CmdCheckOut{
		ArchWarningsDependency: []CheckArchWarningDependency{
			{
				ComponentName:      "handler",
				FileRelativePath:   "internal/handler/user.go",
				FileAbsolutePath:   "/abs/internal/handler/user.go",
				ResolvedImportName: "github.com/x/proj/internal/db",
				Reference:          domain.NewReferenceSingleLine("internal/handler/user.go", 12, 3),
			},
		},
	}

	violations := out.ToViolations()

	assert.Len(t, violations, 1)
	v := violations[0]
	assert.Equal(t, "dependency", v.Type)
	assert.Equal(t, "internal/handler/user.go", v.File)
	assert.Equal(t, 12, v.Line)
	assert.Equal(t, 3, v.Column)
	assert.Equal(t, "handler", v.Component)
	assert.Equal(t, "github.com/x/proj/internal/db", v.Dependency)
	assert.Equal(t, "github.com/x/proj/internal/db", v.Package)
	assert.Contains(t, v.Rule, "handler")
	assert.Contains(t, v.Rule, "github.com/x/proj/internal/db")
}

func TestToViolations_MatchWarnings(t *testing.T) {
	out := CmdCheckOut{
		ArchWarningsMatch: []CheckArchWarningMatch{
			{
				FileRelativePath: "internal/orphan/file.go",
				FileAbsolutePath: "/abs/internal/orphan/file.go",
			},
			{
				FileRelativePath: "internal/orphan/other.go",
				FileAbsolutePath: "/abs/internal/orphan/other.go",
			},
		},
	}

	violations := out.ToViolations()

	assert.Len(t, violations, 2)
	for _, v := range violations {
		assert.Equal(t, "match", v.Type)
		assert.Contains(t, v.File, "internal/orphan/")
		assert.NotEmpty(t, v.Rule)
	}
}

func TestToViolations_DeepscanWarnings(t *testing.T) {
	out := CmdCheckOut{
		ArchWarningsDeepScan: []CheckArchWarningDeepscan{
			{
				Gate: DeepscanWarningGate{
					ComponentName: "operations",
					MethodName:    "NewService",
					RelativePath:  "internal/operations/svc.go",
				},
				Dependency: DeepscanWarningDependency{
					ComponentName: "repository",
					Name:          "UserRepo",
					InjectionAST:  "c.provideUserRepo()",
					InjectionPath: "internal/app/container.go",
					Injection:     domain.NewReferenceSingleLine("internal/app/container.go", 42, 5),
				},
			},
		},
	}

	violations := out.ToViolations()

	assert.Len(t, violations, 1)
	v := violations[0]
	assert.Equal(t, "deepscan", v.Type)
	assert.Equal(t, "internal/app/container.go", v.File)
	assert.Equal(t, 42, v.Line)
	assert.Equal(t, "repository", v.Component)
	assert.Equal(t, "operations", v.Dependency)
	assert.Equal(t, "c.provideUserRepo()", v.Details)
	assert.Contains(t, v.Rule, "repository")
	assert.Contains(t, v.Rule, "operations")
}

func TestToViolations_MixedOrdering(t *testing.T) {
	// All three warning types present; verify order is dependency, then match,
	// then deepscan (deterministic for CI diffs).
	out := CmdCheckOut{
		ArchWarningsDependency: []CheckArchWarningDependency{
			{ComponentName: "a", FileRelativePath: "a.go"},
		},
		ArchWarningsMatch: []CheckArchWarningMatch{
			{FileRelativePath: "b.go"},
		},
		ArchWarningsDeepScan: []CheckArchWarningDeepscan{
			{Dependency: DeepscanWarningDependency{InjectionPath: "c.go"}},
		},
	}

	violations := out.ToViolations()

	assert.Len(t, violations, 3)
	assert.Equal(t, "dependency", violations[0].Type)
	assert.Equal(t, "match", violations[1].Type)
	assert.Equal(t, "deepscan", violations[2].Type)
}

func TestFormatValues(t *testing.T) {
	assert.Contains(t, FormatValues, FormatText)
	assert.Contains(t, FormatValues, FormatJSON)
}

func TestViolation_JSONRoundTrip(t *testing.T) {
	v := Violation{
		Type:       "dependency",
		File:       "internal/handler/user.go",
		Line:       12,
		Component:  "handler",
		Dependency: "repository",
		Package:    "github.com/x/proj/internal/repository",
		Rule:       "component \"handler\" may not depend on \"repository\"",
	}

	// column and details are omitempty — verify they don't appear when empty
	data, err := json.Marshal(v)
	assert.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"type":"dependency"`)
	assert.Contains(t, jsonStr, `"file":"internal/handler/user.go"`)
	assert.Contains(t, jsonStr, `"line":12`)
	assert.Contains(t, jsonStr, `"component":"handler"`)
	assert.Contains(t, jsonStr, `"dependency":"repository"`)
	assert.Contains(t, jsonStr, `"package":`)
	assert.Contains(t, jsonStr, `"rule":`)
	// omitempty fields should be absent
	assert.NotContains(t, jsonStr, `"column"`)
	assert.NotContains(t, jsonStr, `"details"`)
}
