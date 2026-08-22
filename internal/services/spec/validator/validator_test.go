// Rewrites validator_test.go onto fakeDoc (v3: the v1 dsl builder is gone).
// Mechanical mapping: builder() -> docFixture(); b.Components[x]=dsl.ComponentEntry{...}
// -> d.components[x] = domain.NewReferable(fakeComponent{...}, ref(n)); b.Deps[x] ->
// d.deps[x]; b.Vendors[x] -> d.vendors[x]; b.Allow.* -> d fields.
// document(b) / validate unchanged. Version test: v2 doc always version 1 -> keep
// the loop but only {1,false} is constructible; drop out-of-range cases (validator
// still guards them via fakeDoc version field).
package validator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/services/spec"
)

// docFixture returns a minimal valid document: one component "app" (relative
// path "app" resolved against the testdata tree), any-project deps.
// Fixture names reused across tests (goconst).
const (
	tcCompApp   = "app"
	tcCompLayer = "layer"
	tcVendor    = "gorm.io"
	tcVendorImp = "gorm.io/gorm"
)

const tcNoSuchDir = "no-such-dir"

func ref(line int) domain.Reference {
	return domain.NewReferenceSingleLine("fixture_test.go", line, 1)
}

func fixtureRoot(t *testing.T) string {
	t.Helper()
	return "./testdata"
}

func docFixture() *fakeDoc {
	appComp := fakeComponent{paths: []string{tcCompApp}}
	appDep := fakeDep{anyProject: true}
	return &fakeDoc{
		version: 1,
		workdir: ".",
		components: spec.Components{
			tcCompApp: referableComponent(appComp, ref(6)),
		},
		deps: spec.Dependencies{
			tcCompApp: referableDep(appDep, ref(9)),
		},
		vendors:  spec.Vendors{},
		deepScan: true,
	}
}

func validate(t *testing.T, doc spec.Document) []arch.Notice {
	t.Helper()
	v := NewValidator(fakePathResolver{})
	return v.Validate(doc, fixtureRoot(t))
}

func noticeTexts(notices []arch.Notice) []string {
	texts := make([]string, 0, len(notices))
	for _, n := range notices {
		texts = append(texts, n.Notice.Error())
	}
	return texts
}

func findNotice(notices []arch.Notice, substr string) *arch.Notice {
	for i := range notices {
		if strings.Contains(notices[i].Notice.Error(), substr) {
			return &notices[i]
		}
	}
	return nil
}

func assertNoticeRef(t *testing.T, n *arch.Notice, line int) {
	t.Helper()
	assert.True(t, n.Ref.Valid, "notice reference must be valid")
	assert.Equal(t, line, n.Ref.Line, "notice must reference the declaring line")
}

func TestValidatorCleanSpec(t *testing.T) {
	notices := validate(t, docFixture())
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
		d := docFixture()
		d.version = tc.version
		notices := validate(t, d)
		found := findNotice(notices, "version")
		if tc.wantErr {
			require.NotNil(t, found, "version %d: expected a notice, got %v", tc.version, noticeTexts(notices))
			assert.Contains(t, found.Notice.Error(),
				fmt.Sprintf("version '%d' is not supported, supported: [1-1]", tc.version))
		} else {
			assert.Nil(t, found, "version %d: unexpected notice %v", tc.version, noticeTexts(notices))
		}
	}
}

func TestValidatorNoComponents(t *testing.T) {
	d := docFixture()
	d.components = spec.Components{}
	d.deps = spec.Dependencies{}
	notices := validate(t, d)
	found := findNotice(notices, "at least one component")
	require.NotNil(t, found)
}

func TestValidatorUnknownGlobPath(t *testing.T) {
	d := docFixture()
	d.components[tcCompApp] = referableComponent(fakeComponent{paths: []string{tcNoSuchDir}}, ref(77))
	notices := validate(t, d)
	found := findNotice(notices, "not found directories")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
	assert.Contains(t, found.Notice.Error(), tcNoSuchDir)
	assertNoticeRef(t, found, 77)
}

func TestValidatorGlobPathIgnoreNotFound(t *testing.T) {
	d := docFixture()
	d.ignoreNF = true
	d.components["ghost"] = referableComponent(fakeComponent{paths: []string{tcNoSuchDir}}, ref(78))
	notices := validate(t, d)
	assert.Nil(t, findNotice(notices, "not found directories"),
		"ignoreNotFoundComponents=true must suppress the notice, got %v", noticeTexts(notices))
}

func TestValidatorUnknownComponentInDeps(t *testing.T) {
	d := docFixture()
	d.deps[tcCompApp] = referableDep(fakeDep{mayDependOn: []string{"ghost"}}, ref(54))
	notices := validate(t, d)
	found := findNotice(notices, "unknown component 'ghost'")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
}

func TestValidatorDuplicateComponentInDeps(t *testing.T) {
	d := docFixture()
	d.components[tcCompLayer] = referableComponent(fakeComponent{paths: []string{tcCompLayer}}, ref(60))
	d.deps[tcCompApp] = referableDep(fakeDep{mayDependOn: []string{tcCompLayer, tcCompLayer}}, ref(65))
	notices := validate(t, d)
	found := findNotice(notices, "component 'layer' duplicated")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
}

func TestValidatorDepsRuleConflicts(t *testing.T) {
	t.Run("any project + explicit mayDependOn", func(t *testing.T) {
		d := docFixture()
		d.components[tcCompLayer] = referableComponent(fakeComponent{paths: []string{tcCompLayer}}, ref(60))
		d.deps[tcCompApp] = referableDep(fakeDep{anyProject: true, mayDependOn: []string{tcCompLayer}}, ref(61))
		notices := validate(t, d)
		found := findNotice(notices, "anyProjectDeps")
		require.NotNil(t, found, "got %v", noticeTexts(notices))
	})

	t.Run("any vendor + explicit canUse", func(t *testing.T) {
		d := docFixture()
		d.vendors[tcVendor] = referableVendor(fakeVendor{imports: []string{tcVendorImp}}, ref(80))
		d.deps[tcCompApp] = referableDep(fakeDep{anyVendor: true, canUse: []string{tcVendor}}, ref(81))
		notices := validate(t, d)
		found := findNotice(notices, "anyVendorDeps")
		require.NotNil(t, found, "got %v", noticeTexts(notices))
	})

	t.Run("any vendor flag with no vendor rules is fine", func(t *testing.T) {
		d := docFixture()
		d.deps[tcCompApp] = referableDep(fakeDep{anyProject: true, anyVendor: true}, ref(82))
		notices := validate(t, d)
		assert.Empty(t, noticeTexts(notices), "unexpected notices: %v", noticeTexts(notices))
	})
}

func TestValidatorCommonComponents(t *testing.T) {
	d := docFixture()
	d.components[tcCompLayer] = referableComponent(fakeComponent{paths: []string{tcCompLayer}}, ref(90))
	d.commonCom = []domain.Referable[string]{domain.NewReferable(tcCompLayer, ref(91))}
	notices := validate(t, d)
	assert.Nil(t, findNotice(notices, "unknown component"),
		"common component must resolve, got %v", noticeTexts(notices))

	d.commonCom = []domain.Referable[string]{domain.NewReferable("ghost", ref(92))}
	notices = validate(t, d)
	found := findNotice(notices, "unknown component 'ghost'")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
}

func TestValidatorCommonVendors(t *testing.T) {
	d := docFixture()
	d.vendors[tcVendor] = referableVendor(fakeVendor{imports: []string{tcVendorImp}}, ref(110))
	d.commonVen = []domain.Referable[string]{domain.NewReferable(tcVendor, ref(111))}
	notices := validate(t, d)
	assert.Nil(t, findNotice(notices, "unknown vendor"),
		"common vendor must resolve, got %v", noticeTexts(notices))

	d.commonVen = []domain.Referable[string]{domain.NewReferable("ghost.io", ref(112))}
	notices = validate(t, d)
	found := findNotice(notices, "unknown vendor 'ghost.io'")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
}

func TestValidatorDuplicateVendorInDeps(t *testing.T) {
	d := docFixture()
	d.vendors[tcVendor] = referableVendor(fakeVendor{imports: []string{tcVendorImp}}, ref(120))
	d.deps[tcCompApp] = referableDep(fakeDep{
		anyProject: true,
		canUse:     []string{tcVendor, tcVendor},
	}, ref(121))
	notices := validate(t, d)
	found := findNotice(notices, "vendor 'gorm.io' duplicated")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
}

func TestValidatorUnknownVendorInDeps(t *testing.T) {
	d := docFixture()
	d.deps[tcCompApp] = referableDep(fakeDep{canUse: []string{"ghost.io"}}, ref(130))
	notices := validate(t, d)
	found := findNotice(notices, "unknown vendor 'ghost.io'")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
}

func TestValidatorExcludeFiles(t *testing.T) {
	d := docFixture()
	d.exclFiles = []domain.Referable[string]{domain.NewReferable("(", ref(140))}
	notices := validate(t, d)
	found := findNotice(notices, "invalid regexp")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
}

func TestValidatorWorkdir(t *testing.T) {
	d := docFixture()
	d.workdir = tcNoSuchDir
	notices := validate(t, d)
	found := findNotice(notices, "workdir")
	require.NotNil(t, found, "got %v", noticeTexts(notices))
}

func TestFixtureTreePresent(t *testing.T) {
	notices := validate(t, docFixture())
	assert.Empty(t, notices, "fixture tree must satisfy the validator: %v", noticeTexts(notices))
}
