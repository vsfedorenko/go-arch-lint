package checker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

/**
 * Visibility checker tests: the same fake-resolver fixture pattern as the
 * cycles suite — graph edges expressed as FileHold imports.
 *
 * Components: alpha, beta, gamma (module .../vistest).
 */

const visTestModule = "github.com/vsfedorenko/go-arch-lint/v2/checker/vistest"

func visSpec(rules []arch.VisibilityRule, components ...string) arch.Spec {
	spec := arch.Spec{
		RootDirectory: domain.NewReferable("/project", domain.NewEmptyReference()),
		ModuleName:    domain.NewReferable(visTestModule, domain.NewEmptyReference()),
	}

	for _, name := range components {
		component := arch.Component{
			Name: domain.NewReferable(name, domain.NewEmptyReference()),
		}
		pkg := "internal/" + name
		component.ResolvedPaths = append(component.ResolvedPaths, domain.NewReferable(
			models.ResolvedPath{
				ImportPath: visTestModule + "/" + pkg,
				LocalPath:  pkg,
				AbsPath:    "/project/" + pkg,
			},
			domain.NewEmptyReference(),
		))
		spec.Components = append(spec.Components, component)
	}

	if rules != nil {
		spec.Visibility = &arch.Visibility{Rules: rules}
	}

	return spec
}

func TestVisibility_no_rule_noop(t *testing.T) {
	spec := visSpec(nil, tcAlpha, tcBeta)

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcBeta, "internal/beta/b.go", visTestModule+"/internal/alpha"),
	}}

	result, err := NewVisibility().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestVisibility_allowed_consumer_ok(t *testing.T) {
	spec := visSpec([]arch.VisibilityRule{
		{Component: tcAlpha, Allowed: []string{tcBeta}},
	}, tcAlpha, tcBeta)

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcBeta, "internal/beta/b.go", visTestModule+"/internal/alpha"),
	}}

	result, err := NewVisibility().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestVisibility_unlisted_consumer_violation(t *testing.T) {
	spec := visSpec([]arch.VisibilityRule{
		{Component: tcAlpha, Allowed: []string{tcBeta}}, // gamma not listed
	}, tcAlpha, tcBeta, tcGamma)

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcBeta, "internal/beta/b.go", visTestModule+"/internal/alpha"),
		cyclesFile(tcGamma, "internal/gamma/g.go", visTestModule+"/internal/alpha"),
	}}

	result, err := NewVisibility().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	require.Len(t, result.DependencyWarnings, 1)

	w := result.DependencyWarnings[0]
	assert.Contains(t, w.ComponentName, tcGamma)
	assert.Contains(t, w.ComponentName, tcAlpha)
	assert.Contains(t, w.ComponentName, "restricted API")
	assert.Contains(t, w.FileRelativePath, "gamma")
}

func TestVisibility_fully_internal_component(t *testing.T) {
	// No Allowed list at all: nobody may import alpha.
	spec := visSpec([]arch.VisibilityRule{
		{Component: tcAlpha},
	}, tcAlpha, tcBeta)

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcBeta, "internal/beta/b.go", visTestModule+"/internal/alpha"),
	}}

	result, err := NewVisibility().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	require.Len(t, result.DependencyWarnings, 1)
	assert.Contains(t, result.DependencyWarnings[0].ComponentName, tcBeta)
}

func TestVisibility_self_import_ignored(t *testing.T) {
	// The component itself is always implicitly allowed.
	spec := visSpec([]arch.VisibilityRule{
		{Component: tcAlpha},
	}, tcAlpha)

	// buildComponentGraph skips to == from edges, so a self-import hold
	// produces no edge and no violation.
	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcAlpha, "internal/alpha/a.go", visTestModule+"/internal/alpha"),
	}}

	result, err := NewVisibility().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestVisibility_multiple_rules_accumulate(t *testing.T) {
	// Two rules for the same component: allow lists are unioned.
	spec := visSpec([]arch.VisibilityRule{
		{Component: tcAlpha, Allowed: []string{tcBeta}},
		{Component: tcAlpha, Allowed: []string{tcGamma}},
	}, tcAlpha, tcBeta, tcGamma)

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcBeta, "internal/beta/b.go", visTestModule+"/internal/alpha"),
		cyclesFile(tcGamma, "internal/gamma/g.go", visTestModule+"/internal/alpha"),
	}}

	result, err := NewVisibility().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	assert.Empty(t, result.DependencyWarnings)
}

func TestVisibility_multiple_witnesses_one_violation(t *testing.T) {
	// gamma imports alpha from two files — still one violation (edge-level
	// reporting, not per-file). The graph keeps the FIRST witness per edge
	// in resolver order; the production resolver yields files sorted, so
	// the witness is deterministic there. The fake preserves slice order.
	spec := visSpec([]arch.VisibilityRule{
		{Component: tcAlpha},
	}, tcAlpha, tcGamma)

	resolver := fakeProjectFilesResolver{holds: []models.FileHold{
		cyclesFile(tcGamma, "internal/gamma/z.go", visTestModule+"/internal/alpha"),
		cyclesFile(tcGamma, "internal/gamma/a.go", visTestModule+"/internal/alpha"),
	}}

	result, err := NewVisibility().Check(context.Background(), spec, resolver.holds)
	require.NoError(t, err)
	require.Len(t, result.DependencyWarnings, 1)
	assert.Contains(t, result.DependencyWarnings[0].FileRelativePath, "z.go")
}
