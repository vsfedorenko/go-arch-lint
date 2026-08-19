package decoder

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/v2/dsl"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

func buildTestSpec() *dsl.SpecBuilder {
	b := &dsl.SpecBuilder{
		Vendors:    make(map[string]dsl.VendorEntry),
		Components: make(map[string]dsl.ComponentEntry),
		Deps:       make(map[string]dsl.DepEntry),
	}
	b.Version = domain.NewEmptyReferable(1)
	b.Workdir = domain.NewEmptyReferable("internal")
	b.Allow.DepOnAnyVendor = domain.NewEmptyReferable(false)
	b.Allow.DeepScan = domain.NewEmptyReferable(true)
	b.Components["main"] = dsl.ComponentEntry{
		RelativePaths: []string{compApp},
		Reference:     domain.NewReferenceSingleLine("arch.go", 5, 0),
	}
	b.Deps["main"] = dsl.DepEntry{
		MayDependOn: []domain.Referable[string]{
			{Value: compContainer, Reference: domain.NewReferenceSingleLine("arch.go", 10, 0)},
		},
		Reference: domain.NewReferenceSingleLine("arch.go", 9, 0),
	}
	return b
}

func TestGoSpecDocumentVersion(t *testing.T) {
	b := buildTestSpec()
	doc := NewGoSpecDocument(b)
	assert.Equal(t, 1, doc.Version().Value)
}

func TestGoSpecDocumentWorkdir(t *testing.T) {
	b := buildTestSpec()
	doc := NewGoSpecDocument(b)
	assert.Equal(t, "internal", doc.WorkingDirectory().Value)
}

func TestGoSpecDocumentComponents(t *testing.T) {
	b := buildTestSpec()
	doc := NewGoSpecDocument(b)
	comps := doc.Components()
	assert.Contains(t, comps, "main")
	paths := comps["main"].Value.RelativePaths()
	assert.Equal(t, []models.Glob{models.Glob(compApp)}, paths)
}

func TestGoSpecDocumentDeps(t *testing.T) {
	b := buildTestSpec()
	doc := NewGoSpecDocument(b)
	deps := doc.Dependencies()
	assert.Contains(t, deps, "main")
	rule := deps["main"].Value
	assert.Len(t, rule.MayDependOn(), 1)
	assert.Equal(t, compContainer, rule.MayDependOn()[0].Value)
}

func TestGoSpecDocumentOptions(t *testing.T) {
	b := buildTestSpec()
	doc := NewGoSpecDocument(b)
	opts := doc.Options()
	assert.False(t, opts.IsDependOnAnyVendor().Value)
	assert.True(t, opts.DeepScan().Value)
}

func TestGoSpecDocumentEmptyBuilder(t *testing.T) {
	b := &dsl.SpecBuilder{
		Vendors:    make(map[string]dsl.VendorEntry),
		Components: make(map[string]dsl.ComponentEntry),
		Deps:       make(map[string]dsl.DepEntry),
	}
	doc := NewGoSpecDocument(b)
	assert.NotNil(t, doc)
	assert.Empty(t, doc.Components())
	assert.Empty(t, doc.Vendors())
	assert.Empty(t, doc.Dependencies())
}
