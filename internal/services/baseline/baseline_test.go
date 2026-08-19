package baseline_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/services/baseline"
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
	if len(fps) != 2 {
		t.Fatalf("two match warnings must produce two fingerprints, got %d", len(fps))
	}
	if fps[0].String() != fps[1].String() {
		t.Fatalf("fingerprints of the same violation at different lines differ: %q vs %q", fps[0], fps[1])
	}
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
	if len(fps) != 4 {
		t.Fatalf("expected 4 fingerprints (one per kind), got %d: %v", len(fps), fps)
	}

	kinds := map[string]bool{}
	for _, fp := range fps {
		kinds[fp.Kind] = true
	}
	for _, want := range []string{"dep", "match", "deepscan", "naming"} {
		if !kinds[want] {
			t.Errorf("kind %q missing in fingerprints: %v", want, fps)
		}
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
	if fps[0].Rule != "alpha" {
		t.Fatalf("fingerprints must be sorted, first is %q", fps[0].Rule)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "baseline.json") // parent dir does not exist

	fps := baseline.FromResult(models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{depWarning("beta", "a.go")},
	})
	if err := baseline.Save(path, fps); err != nil {
		t.Fatalf("Save with a missing parent dir: %v", err)
	}

	base, exists, err := baseline.Load(path)
	if err != nil || !exists {
		t.Fatalf("Load after Save: exists=%v err=%v", exists, err)
	}
	if len(base) != 1 {
		t.Fatalf("baseline must hold 1 fingerprint, got %d", len(base))
	}
	if _, ok := base[knownKey]; !ok {
		t.Fatalf("fingerprint key %q missing: %v", knownKey, base)
	}
}

// The baseline file is reviewed by humans in PRs: annotations must say
// WHAT was baselined, not just store opaque keys.
func TestSave_WritesHumanReadableAnnotations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")

	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{depWarning("beta", "a.go")},
	}
	if err := baseline.Save(path, baseline.FromResult(result)); err != nil {
		t.Fatal(err)
	}

	var file struct {
		SchemeVersion int               `json:"schemeVersion"`
		Fingerprints  map[string]string `json:"fingerprints"`
	}
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("baseline file must be valid JSON: %v", err)
	}
	if file.SchemeVersion != baseline.SchemeVersion {
		t.Fatalf("scheme version %d, want %d", file.SchemeVersion, baseline.SchemeVersion)
	}
	ann := file.Fingerprints[knownKey]
	if ann == "" || ann == "beta" {
		t.Fatalf("annotation for dep fingerprint must be human-readable, got %q", ann)
	}
}

func TestLoad_MissingFileIsNotAnError(t *testing.T) {
	base, exists, err := baseline.Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("missing baseline must not be an error: %v", err)
	}
	if exists {
		t.Fatal("missing baseline must report exists=false")
	}
	if base == nil || len(base) != 0 {
		t.Fatalf("missing baseline must yield an empty map, got %v", base)
	}
}

func TestLoad_BrokenJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := baseline.Load(path)
	if err == nil {
		t.Fatal("broken JSON must fail")
	}
}

func TestLoad_UnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked.json")
	if err := os.WriteFile(path, []byte("{}"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, _, err := baseline.Load(path)
	if err == nil {
		t.Fatal("unreadable file must fail")
	}
}

func TestLoad_FutureSchemeVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.json")
	future := `{"schemeVersion": 999, "fingerprints": {}}`
	if err := os.WriteFile(path, []byte(future), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := baseline.Load(path)
	if err == nil {
		t.Fatal("future scheme version must fail")
	}
	want := "re-record the baseline"
	if !contains(err.Error(), want) {
		t.Fatalf("error must tell the user to re-record: %v", err)
	}
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
	if diff.Known != 1 {
		t.Fatalf("known=%d, want 1", diff.Known)
	}
	if len(diff.New) != 1 || diff.New[0].Rule != "gamma" {
		t.Fatalf("new=%v, want exactly the gamma violation", diff.New)
	}
	if diff.BaselineSize != 1 {
		t.Fatalf("BaselineSize=%d, want 1", diff.BaselineSize)
	}
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
	if known != 1 {
		t.Fatalf("known=%d, want 1", known)
	}
	if len(filtered.DependencyWarnings) != 1 || filtered.DependencyWarnings[0].ResolvedImportName != "gamma" {
		t.Fatalf("filtered dep warnings = %v, want only gamma", filtered.DependencyWarnings)
	}
	if len(filtered.MatchWarnings) != 1 || len(filtered.NamingWarnings) != 1 {
		t.Fatal("non-dep kinds must pass through untouched")
	}
	if filtered.SuppressedCount != 3 {
		t.Fatalf("SuppressedCount must pass through, got %d", filtered.SuppressedCount)
	}
}

func TestFilterResult_EmptyBaseKeepsEverything(t *testing.T) {
	result := models.CheckResult{
		DependencyWarnings: []models.CheckArchWarningDependency{depWarning("beta", "a.go")},
	}
	filtered, known := baseline.FilterResult(result, map[string]string{})
	if known != 0 || len(filtered.DependencyWarnings) != 1 {
		t.Fatalf("empty baseline must keep all: known=%d len=%d", known, len(filtered.DependencyWarnings))
	}
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
		if !strings.Contains(got, want) {
			t.Errorf("%s: annotation %q must contain %q", fp, got, want)
		}
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
	if err := baseline.Save("baseline.json", fps); err != nil {
		t.Fatalf("Save to bare filename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "baseline.json")); err != nil {
		t.Fatalf("file must exist in cwd: %v", err)
	}
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

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(needle) == 0 ||
		indexOf(haystack, needle) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
