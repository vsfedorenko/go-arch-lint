package decoder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v2 "github.com/vsfedorenko/go-arch-lint/v2/dsl/v2"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

// The V2SpecDocument adapter: dsl/v2.Build -> spec.Document. These tests
// pin the mapping the checker will consume, plus the filesystem
// verification with did-you-mean suggestions.

// Paths become components; /** subtrees keep the glob suffix; Use rules
// become dependency rules with path/vendor targets split correctly.
func TestV2Document_Mapping(t *testing.T) {
	b := v2.Spec(func(s *v2.SpecBuilder) {
		pgx := s.Vendor("pgx", "github.com/jackc/pgx/v5")
		domain := s.Path("shop/domain")
		legacy := s.Path("legacy/**")

		s.Path("shop/core", func() {
			s.Use(domain, legacy, pgx)
		})
	})
	d := NewV2SpecDocument(b)

	comps := d.Components()
	require.Len(t, comps, 3)
	assert.Equal(t, []models.Glob{"shop/domain"}, comps["shop/domain"].Value.RelativePaths())
	assert.Equal(t, []models.Glob{"legacy/**"}, comps["legacy"].Value.RelativePaths(), "subtree keeps /** in the glob")

	vendors := d.Vendors()
	require.Len(t, vendors, 1)
	assert.Equal(t, []models.Glob{"github.com/jackc/pgx/v5"}, vendors["pgx"].Value.ImportPaths())

	deps := d.Dependencies()
	require.Len(t, deps, 1)
	rule := deps["shop/core"].Value
	assert.Equal(t, []string{"shop/domain", "legacy"}, referableStrings(rule.MayDependOn()))
	assert.Equal(t, []string{"pgx"}, referableStrings(rule.CanUse()))

	// The v2 zero-surface: everything the language does not have.
	assert.False(t, d.Options().IsDependOnAnyVendor().Value, "no Use(vendor) rule -> vendor imports are violations")
	assert.Nil(t, d.Tiers())
	assert.Nil(t, d.Naming())
	assert.Nil(t, d.Visibility())
	assert.Nil(t, d.InterfacePlacement())
	assert.Equal(t, "./", d.WorkingDirectory().Value, "no workdir concept; paths are module-relative")
}

func referableStrings(list []domain.Referable[string]) []string {
	out := make([]string, 0, len(list))
	for _, r := range list {
		out = append(out, r.Value)
	}
	return out
}

// FSVerify: a real tree, a declared path that exists, one that does not
// (with a did-you-mean suggestion), an empty /** subtree, and a path-less
// vendor (vendors are not FS-verified — their import paths point outside
// the module).
func TestV2Document_FSVerify(t *testing.T) {
	root := t.TempDir()
	mkdir := func(rel string) {
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755))
	}
	write := func(rel, body string) {
		mkdir(filepath.Dir(rel))
		require.NoError(t, os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(body), 0o600))
	}

	write("shop/domain/model.go", "package domain\n")
	write("shop/adaptive/http/handler.go", "package http\n") // the real directory
	mkdir("legacy/empty")                                    // exists, no Go files

	b := v2.Spec(func(s *v2.SpecBuilder) {
		s.Path("shop/domain")
		s.Path("shop/adaptiv") // typo: the real sibling is shop/adaptive
		s.Path("legacy/**")
	})
	d := NewV2SpecDocument(b)

	notices := d.FSVerify(root)
	// shop/domain exists -> silent. shop/adaptive does not exist (the real
	// child is shop/adaptive/http) -> notice with suggestion from siblings.
	// legacy/** exists but has no Go files -> notice.
	require.Len(t, notices, 2, "domain silent; adaptive missing; legacy empty subtree")

	msgs := []string{notices[0].Notice.Error(), notices[1].Notice.Error()}
	joined := msgs[0] + "\n" + msgs[1]
	assert.Contains(t, joined, `Path("shop/adaptiv")`, "missing path named")
	assert.Contains(t, joined, "did you mean", "suggestion present")
	assert.Contains(t, joined, `Path("legacy/**"): no Go files`, "empty subtree named")
}

// An all-good tree verifies silently.
func TestV2Document_FSVerifyClean(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sub", "b.go"), []byte("package sub\n"), 0o600))

	b := v2.Spec(func(s *v2.SpecBuilder) {
		s.Path(".")
		s.Path("sub")
	})
	d := NewV2SpecDocument(b)
	assert.Empty(t, d.FSVerify(root))
}
