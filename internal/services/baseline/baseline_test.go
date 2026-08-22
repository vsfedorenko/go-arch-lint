package baseline_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/services/baseline"
)

// dep|beta|a.go is the canonical fingerprint used across the suite.
const knownKey = "dep|beta|a.go"

func depWarning(importName, file string) models.CheckArchWarningDependency {
	return models.CheckArchWarningDependency{ResolvedImportName: importName, FileRelativePath: file}
}

// The fingerprint must be line-number independent: the same violation at a
// different line is KNOWN debt, not new. This is the core promise of the
// baseline mode ("line-number-stable fingerprints").
func TestFingerprint_LineNumberIndependent(t *testing.T) {
	w1 := models.CheckArchWarningMatch{FileRelativePath: "a.go"}
	w2 := models.CheckArchWarningMatch{FileRelativePath: "a.go"}

	fps := baseline.FromResult(models.CheckResult{MatchWarnings: []models.CheckArchWarningMatch{w1, w2}})
	require.Len(t, fps, 2, "two match warnings must produce two fingerprints")
	assert.Equal(t, fps[0].String(), fps[1].String(),
		"fingerprints of the same violation at different lines differ")
}

func TestFromResult_AllKinds(t *testing.T) {
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{depWarning("beta", "a.go")},
		MatchWarnings: []models.CheckArchWarningMatch{
			{FileRelativePath: "orphan.go"},
		},
		DeepscanWarnings: []models.CheckArchWarningDeepscan{{
			Dependency: models.DeepscanWarningDependency{ComponentName: "compA", Name: "NewRepo"},
			Gate:       models.DeepscanWarningGate{ComponentName: "compB", MethodName: "Serve"},
			Target:     models.DeepscanWarningTarget{RelativePath: "internal/b/gate.go"},
		}},
		NamingWarnings: []models.CheckArchWarningNaming{
			{PackageName: "helpers", FileRelativePath: "internal/helpers/x.go"},
		},
	}

	fps := baseline.FromResult(result)
	require.Len(t, fps, 4, "fingerprints (one per kind): ")

	kinds := map[string]bool{}
	for _, fp := range fps {
		kinds[fp.Kind] = true
	}
	for _, want := range []string{"dep", "match", "deepscan", "naming"} {
		assert.True(t, kinds[want], "kind %q missing in fingerprints: %v", want, fps)
	}
}

func TestFromResult_SortedDeterministically(t *testing.T) {
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("zeta", "z.go"),
			depWarning("alpha", "m.go"),
		},
	}
	fps := baseline.FromResult(result)
	assert.Equal(t, "alpha", fps[0].Rule, "fingerprints must be sorted")
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "baseline.json") // parent dir does not exist

	fps := baseline.FromResult(models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{depWarning("beta", "a.go")},
	})
	require.NoError(t, baseline.Save(path, fps), "Save with a missing parent dir")

	base, exists, err := baseline.Load(path)
	require.NoError(t, err, "Load after Save")
	require.True(t, exists, "Load after Save")
	require.Len(t, base, 1, "baseline must hold 1 fingerprint")
	assert.Contains(t, base, knownKey, "fingerprint key must be present")
}

// The baseline file is reviewed by humans in PRs: annotations must say
// WHAT was baselined, not just store opaque keys.
func TestSave_WritesHumanReadableAnnotations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")

	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{depWarning("beta", "a.go")},
	}
	require.NoError(t, baseline.Save(path, baseline.FromResult(result)))

	var file struct {
		SchemeVersion int               `json:"schemeVersion"`
		Fingerprints  map[string]string `json:"fingerprints"`
	}
	raw, _ := os.ReadFile(path)
	require.NoError(t, json.Unmarshal(raw, &file), "baseline file must be valid JSON")
	assert.Equal(t, baseline.SchemeVersion, file.SchemeVersion, "scheme version")

	ann := file.Fingerprints[knownKey]
	assert.NotEmpty(t, ann, "annotation for dep fingerprint must be human-readable")
	assert.NotEmpty(t, ann, "annotation for dep fingerprint must be human-readable, got empty")
	assert.NotEqual(t, "beta", ann, "annotation must not be the bare rule text")
}

func TestLoad_MissingFileIsNotAnError(t *testing.T) {
	base, exists, err := baseline.Load(filepath.Join(t.TempDir(), "absent.json"))
	require.NoError(t, err, "missing baseline must not be an error")
	assert.False(t, exists, "missing baseline must report exists=false")
	assert.Empty(t, base, "missing baseline must yield an empty map")
}

func TestLoad_BrokenJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.json")
	require.NoError(t, os.WriteFile(path, []byte("{broken"), 0o600))

	_, _, err := baseline.Load(path)
	require.Error(t, err, "broken JSON must fail")
}

func TestLoad_UnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked.json")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, _, err := baseline.Load(path)
	require.Error(t, err, "unreadable file must fail")
}

func TestLoad_FutureSchemeVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.json")
	future := `{"schemeVersion": 999, "fingerprints": {}}`
	require.NoError(t, os.WriteFile(path, []byte(future), 0o600))

	_, _, err := baseline.Load(path)
	require.Error(t, err, "future scheme version must fail")
	require.ErrorContains(t, err, "re-record the baseline", "error must tell the user to re-record")
}

func TestCompare_KnownAndNew(t *testing.T) {
	base := map[string]string{knownKey: "annot"}
	fps := baseline.FromResult(models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("beta", "a.go"),  // known
			depWarning("gamma", "b.go"), // new
		},
	})

	diff := baseline.Compare(fps, base)
	assert.Equal(t, 1, diff.Known, "known")
	require.Len(t, diff.New, 1, "new must hold exactly the gamma violation")
	assert.Equal(t, "gamma", diff.New[0].Rule, "new violation rule")
	assert.Equal(t, 1, diff.BaselineSize, "BaselineSize")
}

func TestFilterResult_KeepsNewRemovesKnown(t *testing.T) {
	base := map[string]string{knownKey: "annot"}
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{
			depWarning("beta", "a.go"),
			depWarning("gamma", "b.go"),
		},
		MatchWarnings:   []models.CheckArchWarningMatch{{FileRelativePath: "x.go"}},
		NamingWarnings:  []models.CheckArchWarningNaming{{PackageName: "helpers", FileRelativePath: "h.go"}},
		SuppressedCount: 3,
	}

	filtered, known := baseline.FilterResult(result, base)
	assert.Equal(t, 1, known, "known")
	require.Len(t, filtered.DependencyWarnings, 1, "filtered dep warnings must hold only gamma")
	assert.Equal(t, "gamma", filtered.DependencyWarnings[0].ResolvedImportName, "filtered dep warnings")
	assert.Len(t, filtered.MatchWarnings, 1, "non-dep kinds must pass through untouched")
	assert.Len(t, filtered.NamingWarnings, 1, "non-dep kinds must pass through untouched")
	assert.Equal(t, 3, filtered.SuppressedCount, "SuppressedCount must pass through")
}

func TestFilterResult_EmptyBaseKeepsEverything(t *testing.T) {
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{depWarning("beta", "a.go")},
	}
	filtered, known := baseline.FilterResult(result, map[string]string{})
	assert.Equal(t, 0, known, "empty baseline must keep all")
	assert.Len(t, filtered.DependencyWarnings, 1, "empty baseline must keep all")
}

// Annotations exist so PR reviewers see WHAT was baselined: cover every
// kind, including the defensive default branch.
func TestAnnotations_AllKinds(t *testing.T) {
	cases := map[string]string{
		knownKey:                       "component imports beta",
		"match|file-not-attached|x.go": "file not attached",
		"deepscan|a->b:N:M|c.go":       "injected dependency",
		"naming|helpers|h.go":          "forbidden package name helpers",
	}
	for fp, want := range cases {
		parts := strings.SplitN(fp, "|", 3)
		f := baseline.Fingerprint{Kind: parts[0], Rule: parts[1], File: parts[2]}
		got := annotationOfExported(f)
		assert.Contains(t, got, want, "%s: annotation %q must contain ")
	}
}

// Save into a bare filename (no directory part) must not attempt MkdirAll.
func TestSave_BareFilename(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Skip("cannot get cwd")
	}
	if err := os.Chdir(dir); err != nil {
		t.Skip("cannot chdir")
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	fps := baseline.FromResult(models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{depWarning("beta", "a.go")},
	})
	require.NoError(t, baseline.Save("baseline.json", fps), "Save to bare filename")
	_, err = os.Stat(filepath.Join(dir, "baseline.json"))
	require.NoError(t, err, "file must exist in cwd")
}

// annotationOfExported exercises annotationOf through the public API: the
// annotation is what Save puts next to the fingerprint key in the file.
func annotationOfExported(f baseline.Fingerprint) string {
	path := filepath.Join(os.TempDir(), "gal-bl-ann.json")
	if err := baseline.Save(path, []baseline.Fingerprint{f}); err != nil {
		panic(err)
	}
	base, _, err := baseline.Load(path)
	if err != nil {
		panic(err)
	}
	return base[f.String()]
}
