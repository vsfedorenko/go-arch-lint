package decoder

import (
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/common"
	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/dsl"
)

func buildTestSpec() *dsl.SpecBuilder {
	b := &dsl.SpecBuilder{
		Vendors:    make(map[string]dsl.VendorEntry),
		Components: make(map[string]dsl.ComponentEntry),
		Deps:       make(map[string]dsl.DepEntry),
	}
	b.Version = common.NewEmptyReferable(1)
	b.Workdir = common.NewEmptyReferable("internal")
	b.Allow.DepOnAnyVendor = common.NewEmptyReferable(false)
	b.Allow.DeepScan = common.NewEmptyReferable(true)
	b.Components["main"] = dsl.ComponentEntry{
		RelativePaths: []string{"app"},
		Reference:     common.NewReferenceSingleLine("arch.go", 5, 0),
	}
	b.Deps["main"] = dsl.DepEntry{
		MayDependOn: []common.Referable[string]{
			{Value: "container", Reference: common.NewReferenceSingleLine("arch.go", 10, 0)},
		},
		Reference: common.NewReferenceSingleLine("arch.go", 9, 0),
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
	assert.Equal(t, []models.Glob{"app"}, paths)
}

func TestGoSpecDocumentDeps(t *testing.T) {
	b := buildTestSpec()
	doc := NewGoSpecDocument(b)
	deps := doc.Dependencies()
	assert.Contains(t, deps, "main")
	rule := deps["main"].Value
	assert.Len(t, rule.MayDependOn(), 1)
	assert.Equal(t, "container", rule.MayDependOn()[0].Value)
}

func TestGoSpecDocumentOptions(t *testing.T) {
	b := buildTestSpec()
	doc := NewGoSpecDocument(b)
	opts := doc.Options()
	assert.Equal(t, false, opts.IsDependOnAnyVendor().Value)
	assert.Equal(t, true, opts.DeepScan().Value)
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
