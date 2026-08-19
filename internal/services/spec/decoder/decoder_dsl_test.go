package decoder

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
)

// buildSpecThroughDSL assembles a SpecBuilder by running the real DSL
// functions inside Spec() — the same path a user's arch.go takes. This
// exercises the builders → decoder bridge, not just hand-filled structs.
// Component names used across the suite (goconst: 3+ occurrences).
const (
	compApp       = "app"
	compContainer = "container"
)

func buildSpecThroughDSL(t *testing.T) *dsl.SpecBuilder {
	t.Helper()
	def := dsl.Spec(func() {
		dsl.Version(1)
		dsl.Workdir("internal")
		dsl.Allow(func() {
			dsl.DepOnAnyVendor(false)
			dsl.DeepScan(true)
			dsl.IgnoreNotFoundComponents(true)
		})
		dsl.ExcludeFiles(`^.*_test\.go$`, `^mock/.*$`)
		dsl.Vendor("cobra", "github.com/spf13/cobra")
		dsl.CommonVendors("cobra")
		dsl.Component(compApp, "app")
		dsl.Component(compContainer, "app/container")
		dsl.CommonComponents("models")
		dsl.Tiers("high", "low")
		dsl.Tier("high", compApp)
		dsl.Tier("low", compContainer)
		dsl.Deps(compApp, func() {
			dsl.MayDependOn(compContainer)
			dsl.CanUse("cobra")
		})
		dsl.Interfaces(func() {
			dsl.MustLiveWithConsumer()
		})
		dsl.Naming(func() {
			dsl.ForbiddenPackages("utils", "helpers")
		})
		dsl.Visibility(func() {
			dsl.VisibleTo(compApp, compContainer)
		})
	})
	return def.Builder()
}

func TestDecode_FullSpec(t *testing.T) {
	builder := buildSpecThroughDSL(t)
	doc, notices, err := NewGoDecoder(builder).Decode("ignored")
	assert.NoError(t, err)
	assert.Empty(t, notices)

	assert.Equal(t, 1, doc.Version().Value)
	assert.Equal(t, "internal", doc.WorkingDirectory().Value)

	opts := doc.Options()
	assert.False(t, opts.IsDependOnAnyVendor().Value)
	assert.True(t, opts.DeepScan().Value)
	assert.True(t, opts.IgnoreNotFoundComponents().Value)

	assert.Len(t, doc.ExcludedFilesRegExp(), 2)
	assert.Len(t, doc.Vendors(), 1)
	assert.True(t, containsReferable(doc.CommonVendors(), "cobra"), "common vendors must include cobra")

	comps := doc.Components()
	assert.Contains(t, comps, compApp)
	paths := comps["app"].Value.RelativePaths()
	if assert.Len(t, paths, 1) {
		assert.Equal(t, compApp, string(paths[0]))
	}

	assert.True(t, containsReferable(doc.CommonComponents(), "models"), "common components must include models")

	tiers := doc.Tiers()
	if assert.Len(t, tiers, 2) {
		assert.Equal(t, "high", tiers[0].Name)
		assert.Equal(t, []string{compApp}, tiers[0].Components)
		assert.Equal(t, "low", tiers[1].Name)
		assert.Equal(t, []string{compContainer}, tiers[1].Components)
	}

	deps := doc.Dependencies()
	if assert.Contains(t, deps, compApp) {
		rule := deps["app"].Value
		assert.Len(t, rule.MayDependOn(), 1)
		assert.Equal(t, compContainer, rule.MayDependOn()[0].Value)
		assert.Equal(t, "cobra", rule.CanUse()[0].Value)
	}

	ip := doc.InterfacePlacement()
	if assert.NotNil(t, ip) {
		assert.True(t, ip.MustLiveWithConsumer())
	}

	naming := doc.Naming()
	if assert.NotNil(t, naming) {
		forbidden := naming.ForbiddenPackages()
		if assert.Len(t, forbidden, 2) {
			assert.Equal(t, "utils", forbidden[0].Value)
		}
	}

	vis := doc.Visibility()
	if assert.NotNil(t, vis) {
		rules := vis.Rules()
		if assert.Len(t, rules, 1) {
			assert.Equal(t, compApp, rules[0].Component)
			assert.Equal(t, []string{compContainer}, rules[0].Allowed)
		}
	}
}

// Default values: DeepScan defaults to true (since v3), DeepScan inside a
// Deps block defaults to false, IgnoreNotFoundComponents defaults to false.
func TestDecode_Defaults(t *testing.T) {
	b := &dsl.SpecBuilder{
		Vendors:    map[string]dsl.VendorEntry{},
		Components: map[string]dsl.ComponentEntry{},
		Deps:       map[string]dsl.DepEntry{},
	}
	doc := NewGoSpecDocument(b)

	assert.True(t, doc.Options().DeepScan().Value, "global DeepScan must default to true")
	assert.False(t, doc.Options().IgnoreNotFoundComponents().Value)

	b.Deps["x"] = dsl.DepEntry{}
	deps := doc.Dependencies()
	rule := deps["x"].Value
	assert.False(t, rule.DeepScan().Value, "per-component DeepScan must default to false")
}

func TestDecode_NilOptionalBlocks(t *testing.T) {
	b := &dsl.SpecBuilder{
		Vendors:    map[string]dsl.VendorEntry{},
		Components: map[string]dsl.ComponentEntry{},
		Deps:       map[string]dsl.DepEntry{},
	}
	doc := NewGoSpecDocument(b)
	assert.Nil(t, doc.InterfacePlacement())
	assert.Nil(t, doc.Naming())
	assert.Nil(t, doc.Visibility())
}

func TestDecode_TiersAreCopies(t *testing.T) {
	builder := buildSpecThroughDSL(t)
	doc := NewGoSpecDocument(builder)

	tiers := doc.Tiers()
	if !assert.Len(t, tiers, 2) {
		return
	}
	// Mutating the decoded copy must not corrupt the builder's data
	// (decoder promises defensive copies).
	tiers[0].Components[0] = "mutated"
	fresh := doc.Tiers()
	assert.Equal(t, compApp, fresh[0].Components[0], "Tiers() must return defensive copies")
}

func containsReferable(refs []domain.Referable[string], want string) bool {
	for _, r := range refs {
		if r.Value == want {
			return true
		}
	}
	return false
}
