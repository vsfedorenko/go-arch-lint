package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/dsl"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/internal/models/domain"
	"github.com/vsfedorenko/go-arch-lint/internal/services/spec"
	"github.com/vsfedorenko/go-arch-lint/internal/services/spec/decoder"
)

// The validator suite exercises the real Document implementation
// (decoder.GoSpecDocument over a dsl.SpecBuilder) — the same path production
// code takes — so notices are proven against actual references, options
// defaults, and map layouts.

// Fixture names reused across tests (goconst).
const (
	tcCompApp   = "app"
	tcCompLayer = "layer"
	tcVendor    = "gorm.io"
	tcVendorImp = "gorm.io/gorm"
)

// fakePathResolver resolves glob paths against the real testdata tree.
type fakePathResolver struct{}

func (fakePathResolver) Resolve(absPath string) ([]string, error) {
	return filepath.Glob(absPath)
}

// fixtureRoot is the directory validators check globs against. "app",
// "layer", and "domain" exist under it (guarded by TestFixtureTreePresent).
func fixtureRoot(t *testing.T) string {
	t.Helper()
	require.DirExists(t, filepath.Join("testdata", "app"))
	return "testdata"
}

// ref builds a reference with a stable test file/line.
func ref(line int) domain.Reference {
	return domain.NewReferenceSingleLine("arch.go", line, 0)
}

// builder assembles a minimal valid spec: version 1, workdir ".",
// one component "app" mapped to testdata/app.
func builder(t *testing.T) *dsl.SpecBuilder {
	t.Helper()
	b := &dsl.SpecBuilder{
		Vendors:    map[string]dsl.VendorEntry{},
		Components: map[string]dsl.ComponentEntry{},
		Deps:       map[string]dsl.DepEntry{},
	}
	b.Version = domain.NewReferable(1, ref(1))
	b.Workdir = domain.NewReferable(".", ref(2))
	b.Allow.DepOnAnyVendor = domain.NewReferable(false, ref(3))
	b.Allow.DeepScan = domain.NewReferable(true, ref(4))
	b.Allow.IgnoreNotFoundComponents = domain.NewReferable(false, ref(5))
	b.Components["app"] = dsl.ComponentEntry{
		RelativePaths: []string{tcCompApp},
		Reference:     ref(6),
	}
	b.Deps["app"] = dsl.DepEntry{
		MayDependOn:    []domain.Referable[string]{},
		CanUse:         []domain.Referable[string]{},
		AnyProjectDeps: domain.NewReferable(true, ref(7)),
		AnyVendorDeps:  domain.NewReferable(false, ref(8)),
		Reference:      ref(9),
	}
	return b
}

// document wraps a builder into the production Document implementation.
func document(b *dsl.SpecBuilder) spec.Document {
	return decoder.NewGoSpecDocument(b)
}

// validate runs the full validator chain (the same composition as
// NewValidator.Validate) against the document over the testdata fixture.
func validate(t *testing.T, doc spec.Document) []arch.Notice {
	t.Helper()
	v := NewValidator(fakePathResolver{})
	return v.Validate(doc, fixtureRoot(t))
}

// noticeTexts extracts just the messages for readable failure output.
func noticeTexts(notices []arch.Notice) []string {
	texts := make([]string, 0, len(notices))
	for _, n := range notices {
		texts = append(texts, n.Notice.Error())
	}
	return texts
}

// findNotice returns the first notice whose message contains the substring.
func findNotice(notices []arch.Notice, substr string) *arch.Notice {
	for i := range notices {
		if strings.Contains(notices[i].Notice.Error(), substr) {
			return &notices[i]
		}
	}
	return nil
}

// assertNoticeRef verifies a notice carries a valid reference to the
// expected source line — the "source-referenced" half of the roadmap item.
func assertNoticeRef(t *testing.T, n *arch.Notice, line int) {
	t.Helper()
	assert.True(t, n.Ref.Valid, "notice reference must be valid")
	assert.Equal(t, line, n.Ref.Line, "notice must reference the declaring line")
}

func TestValidatorCleanSpec(t *testing.T) {
	notices := validate(t, document(builder(t)))
	assert.Empty(t, notices, "a minimal valid spec must produce no notices: %v", noticeTexts(notices))
}

func TestValidatorVersion(t *testing.T) {
	for _, tc := range []struct {
		version int
		wantErr bool
	}{
		{1, false},
		{0, true},
		{2, true},
		{-1, true},
	} {
		b := builder(t)
		b.Version = domain.NewReferable(tc.version, ref(42))
		notices := validate(t, document(b))
		found := findNotice(notices, "version")
		if tc.wantErr {
			require.NotNil(t, found, "version %d: expected a notice, got %v", tc.version, noticeTexts(notices))
			assert.Contains(t, found.Notice.Error(),
				fmt.Sprintf("version '%d' is not supported, supported: [1-1]", tc.version))
			assertNoticeRef(t, found, 42)
		} else {
			assert.Nil(t, found, "version %d: unexpected notice %v", tc.version, noticeTexts(notices))
		}
	}
}

func TestValidatorNoComponents(t *testing.T) {
	b := builder(t)
	b.Components = map[string]dsl.ComponentEntry{}
	b.Deps = map[string]dsl.DepEntry{}
	notices := validate(t, document(b))
	found := findNotice(notices, "at least one component")
	require.NotNil(t, found)
	assertNoticeRef(t, found, 1)
}

func TestValidatorUnknownGlobPath(t *testing.T) {
	b := builder(t)
	b.Components["app"] = dsl.ComponentEntry{
		RelativePaths: []string{"no-such-dir"},
		Reference:     ref(77),
	}
	notices := validate(t, document(b))
	found := findNotice(notices, "not found directories")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
	assert.Contains(t, found.Notice.Error(), "no-such-dir")
	assertNoticeRef(t, found, 77)
}

func TestValidatorGlobPathIgnoreNotFound(t *testing.T) {
	b := builder(t)
	b.Allow.IgnoreNotFoundComponents = domain.NewReferable(true, ref(5))
	b.Components["ghost"] = dsl.ComponentEntry{
		RelativePaths: []string{"no-such-dir"},
		Reference:     ref(78),
	}
	notices := validate(t, document(b))
	assert.Nil(t, findNotice(notices, "not found directories"),
		"ignoreNotFoundComponents=true must suppress the notice, got %v", noticeTexts(notices))
}

func TestValidatorUnknownComponentInDeps(t *testing.T) {
	b := builder(t)
	b.Deps["app"] = dsl.DepEntry{
		MayDependOn:    []domain.Referable[string]{{Value: "ghost", Reference: ref(51)}},
		AnyProjectDeps: domain.NewReferable(false, ref(52)),
		AnyVendorDeps:  domain.NewReferable(false, ref(53)),
		Reference:      ref(54),
	}
	notices := validate(t, document(b))
	found := findNotice(notices, "unknown component 'ghost'")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
	assertNoticeRef(t, found, 51)
}

func TestValidatorDuplicateComponentInDeps(t *testing.T) {
	b := builder(t)
	b.Components[tcCompLayer] = dsl.ComponentEntry{RelativePaths: []string{tcCompLayer}, Reference: ref(60)}
	b.Deps["app"] = dsl.DepEntry{
		MayDependOn: []domain.Referable[string]{
			{Value: tcCompLayer, Reference: ref(61)},
			{Value: tcCompLayer, Reference: ref(62)},
		},
		AnyProjectDeps: domain.NewReferable(false, ref(63)),
		AnyVendorDeps:  domain.NewReferable(false, ref(64)),
		Reference:      ref(65),
	}
	notices := validate(t, document(b))
	found := findNotice(notices, "component 'layer' duplicated")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
	assertNoticeRef(t, found, 62)
}

func TestValidatorDepsRuleConflicts(t *testing.T) {
	t.Run("anyProjectDeps with non-empty mayDependOn", func(t *testing.T) {
		b := builder(t)
		b.Deps["app"] = dsl.DepEntry{
			MayDependOn:    []domain.Referable[string]{{Value: "app", Reference: ref(71)}},
			AnyProjectDeps: domain.NewReferable(true, ref(72)),
			AnyVendorDeps:  domain.NewReferable(false, ref(73)),
			Reference:      ref(74),
		}
		notices := validate(t, document(b))
		found := findNotice(notices, "'anyProjectDeps=true' used with not empty 'MayDependOn'")
		require.NotNil(t, found, "got %v", noticeTexts(notices))
		assertNoticeRef(t, found, 72)
	})

	t.Run("anyVendorDeps with non-empty canUse", func(t *testing.T) {
		b := builder(t)
		b.Vendors[tcVendor] = dsl.VendorEntry{ImportPaths: []string{tcVendorImp}, Reference: ref(80)}
		b.Deps["app"] = dsl.DepEntry{
			CanUse:         []domain.Referable[string]{{Value: tcVendor, Reference: ref(81)}},
			AnyProjectDeps: domain.NewReferable(false, ref(82)),
			AnyVendorDeps:  domain.NewReferable(true, ref(83)),
			Reference:      ref(84),
		}
		notices := validate(t, document(b))
		found := findNotice(notices, "'anyVendorDeps=true' used with not empty 'CanUse'")
		require.NotNil(t, found, "got %v", noticeTexts(notices))
	})

	t.Run("empty rule without any flag", func(t *testing.T) {
		b := builder(t)
		b.Deps["app"] = dsl.DepEntry{
			MayDependOn:    []domain.Referable[string]{},
			CanUse:         []domain.Referable[string]{},
			AnyProjectDeps: domain.NewReferable(false, ref(91)),
			AnyVendorDeps:  domain.NewReferable(false, ref(92)),
			Reference:      ref(93),
		}
		notices := validate(t, document(b))
		found := findNotice(notices, "should have ref in 'mayDependOn'/'canUse'")
		require.NotNil(t, found, "got %v", noticeTexts(notices))
		assertNoticeRef(t, found, 93)
	})
}

func TestValidatorCommonComponents(t *testing.T) {
	b := builder(t)
	b.CommonComponents = []domain.Referable[string]{{Value: "ghost", Reference: ref(101)}}
	notices := validate(t, document(b))
	found := findNotice(notices, "unknown component 'ghost'")
	require.NotNil(t, found)
	assertNoticeRef(t, found, 101)
}

func TestValidatorCommonVendors(t *testing.T) {
	b := builder(t)
	b.Vendors[tcVendor] = dsl.VendorEntry{ImportPaths: []string{tcVendorImp}, Reference: ref(110)}
	b.CommonVendors = []domain.Referable[string]{{Value: "unknown-vendor", Reference: ref(111)}}
	notices := validate(t, document(b))
	found := findNotice(notices, "unknown vendor 'unknown-vendor'")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
	assertNoticeRef(t, found, 111)
}

func TestValidatorDuplicateVendorInDeps(t *testing.T) {
	b := builder(t)
	b.Vendors[tcVendor] = dsl.VendorEntry{ImportPaths: []string{tcVendorImp}, Reference: ref(120)}
	b.Deps["app"] = dsl.DepEntry{
		CanUse: []domain.Referable[string]{
			{Value: tcVendor, Reference: ref(121)},
			{Value: tcVendor, Reference: ref(122)},
		},
		AnyProjectDeps: domain.NewReferable(false, ref(123)),
		AnyVendorDeps:  domain.NewReferable(false, ref(124)),
		Reference:      ref(125),
	}
	notices := validate(t, document(b))
	found := findNotice(notices, "vendor 'gorm.io' duplicated")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
	assertNoticeRef(t, found, 122)
}

func TestValidatorUnknownVendorInDeps(t *testing.T) {
	b := builder(t)
	b.Deps["app"] = dsl.DepEntry{
		CanUse:         []domain.Referable[string]{{Value: "ghost-vendor", Reference: ref(131)}},
		AnyProjectDeps: domain.NewReferable(false, ref(132)),
		AnyVendorDeps:  domain.NewReferable(false, ref(133)),
		Reference:      ref(134),
	}
	notices := validate(t, document(b))
	found := findNotice(notices, "unknown vendor 'ghost-vendor'")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
	assertNoticeRef(t, found, 131)
}

func TestValidatorExcludeFiles(t *testing.T) {
	b := builder(t)
	b.ExcludeFiles = []domain.Referable[string]{
		{Value: `^.*_test\.go$`, Reference: ref(140)},
		{Value: `^[unclosed$`, Reference: ref(141)},
	}
	notices := validate(t, document(b))
	found := findNotice(notices, "invalid regexp")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
	assert.Contains(t, found.Notice.Error(), `[unclosed$`)
	assertNoticeRef(t, found, 141)
}

func TestValidatorWorkdir(t *testing.T) {
	t.Run("missing workdir", func(t *testing.T) {
		b := builder(t)
		b.Workdir = domain.NewReferable("no-such-dir", ref(150))
		notices := validate(t, document(b))
		found := findNotice(notices, "invalid workdir")
		require.NotNil(t, found, "got %v", noticeTexts(notices))
		assert.Contains(t, found.Notice.Error(), "no-such-dir")
		assertNoticeRef(t, found, 150)
	})

	t.Run("valid workdir", func(t *testing.T) {
		b := builder(t)
		b.Workdir = domain.NewReferable(".", ref(151))
		assert.Empty(t, validate(t, document(b)))
	})
}

// The fixture tree must keep the directories the globs resolve against —
// if testdata is lost the glob tests silently degrade to "not found".
func TestFixtureTreePresent(t *testing.T) {
	for _, dir := range []string{"app", "layer", "domain"} {
		info, err := os.Stat(filepath.Join("testdata", dir))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	}
}
