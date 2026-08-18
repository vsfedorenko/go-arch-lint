// Package baseline implements the incremental adoption mode: compare the
// current check result against a recorded baseline of known violations so
// CI fails only on NEW violations ("don't fix everything, don't add new").
//
// A baseline is a JSON document listing violation fingerprints. A
// fingerprint identifies a violation independent of incidental details
// (code line numbers shift as the file grows); the scheme is versioned so
// future formats can evolve without breaking existing files.
package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

// SchemeVersion is the fingerprint scheme of the baseline file format.
// It is bumped whenever fingerprint composition changes in a way that
// makes old baselines incomparable with newly computed fingerprints.
const SchemeVersion = 1

// fileScheme mirrors the JSON layout persisted on disk.
type fileScheme struct {
	SchemeVersion int               `json:"schemeVersion"`
	Fingerprints  map[string]string `json:"fingerprints"`
}

// Fingerprint is a stable identity of a single violation: kind + rule
// payload + file. Line and column are deliberately excluded — they drift
// on every edit above the violation and would make baselines brittle.
type Fingerprint struct {
	Kind string // "dep" | "match" | "deepscan" | "naming"
	Rule string // violation-specific identity (import path, package name, gate/method pair)
	File string // path relative to the project root
}

// String renders the fingerprint in the canonical "kind|rule|file" form
// used both as the baseline map key and in user-facing messages.
func (f Fingerprint) String() string {
	return f.Kind + "|" + f.Rule + "|" + f.File
}

// Per-category fingerprint constructors: a single place defines what
// identifies a violation, shared by FromResult (recording) and
// FilterResult (comparing).
func depFingerprint(w models.CheckArchWarningDependency) Fingerprint {
	return Fingerprint{Kind: "dep", Rule: w.ResolvedImportName, File: w.FileRelativePath}
}

func matchFingerprint(w models.CheckArchWarningMatch) Fingerprint {
	return Fingerprint{Kind: "match", Rule: "file-not-attached", File: w.FileRelativePath}
}

func deepscanFingerprint(w models.CheckArchWarningDeepscan) Fingerprint {
	return Fingerprint{
		Kind: "deepscan",
		Rule: w.Dependency.ComponentName + "->" + w.Gate.ComponentName +
			":" + w.Dependency.Name + ":" + w.Gate.MethodName,
		File: w.Target.RelativePath,
	}
}

func namingFingerprint(w models.CheckArchWarningNaming) Fingerprint {
	return Fingerprint{Kind: "naming", Rule: w.PackageName, File: w.FileRelativePath}
}

// FromResult extracts the fingerprints of every violation in a check
// result. The result must be the FULL checker output (before the
// --max-warnings display cap): the cap is a rendering concern and must
// not influence which violations get baselined.
func FromResult(result models.CheckResult) []Fingerprint {
	fps := make([]Fingerprint, 0,
		len(result.DependencyWarnings)+len(result.MatchWarnings)+
			len(result.DeepscanWarnings)+len(result.NamingWarnings))

	for _, w := range result.DependencyWarnings {
		fps = append(fps, depFingerprint(w))
	}
	for _, w := range result.MatchWarnings {
		fps = append(fps, matchFingerprint(w))
	}
	for _, w := range result.DeepscanWarnings {
		fps = append(fps, deepscanFingerprint(w))
	}
	for _, w := range result.NamingWarnings {
		fps = append(fps, namingFingerprint(w))
	}

	sort.Slice(fps, func(i, j int) bool { return fps[i].String() < fps[j].String() })
	return fps
}

// Load reads a baseline file from disk. A missing file with ok=false is
// the "no baseline recorded yet" state — the caller decides whether that
// is an error (check mode) or fine (record mode captures everything).
func Load(path string) (map[string]string, bool, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // user-supplied baseline path
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, false, nil
		}
		return nil, false, fmt.Errorf("failed to read baseline file %s: %w", path, err)
	}

	var file fileScheme
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, false, fmt.Errorf("failed to parse baseline file %s: %w", path, err)
	}

	if file.SchemeVersion != SchemeVersion {
		return nil, false, fmt.Errorf(
			"baseline file %s has scheme version %d, this build understands %d — re-record the baseline",
			path, file.SchemeVersion, SchemeVersion,
		)
	}

	return file.Fingerprints, true, nil
}

// Save persists fingerprints to path, creating parent directories as
// needed (the conventional .go-arch-lint/baseline.json lives in a dir
// that may not exist yet on first record).
func Save(path string, fps []Fingerprint) error {
	file := fileScheme{
		SchemeVersion: SchemeVersion,
		Fingerprints:  map[string]string{},
	}
	for _, fp := range fps {
		file.Fingerprints[fp.String()] = annotationOf(fp)
	}

	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode baseline: %w", err)
	}
	raw = append(raw, '\n')

	if dir := parentDir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create baseline dir %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(path, raw, 0o644); err != nil { //nolint:gosec // baseline artifact is non-secret
		return fmt.Errorf("failed to write baseline file %s: %w", path, err)
	}
	return nil
}

// annotationOf returns a human-readable description stored next to each
// fingerprint; it makes the file reviewable in PRs (reviewers see WHAT
// was baselined, not an opaque hash list).
func annotationOf(fp Fingerprint) string {
	switch fp.Kind {
	case "dep":
		return "component imports " + fp.Rule
	case "match":
		return "file not attached to any component"
	case "deepscan":
		return "injected dependency " + fp.Rule
	case "naming":
		return "forbidden package name " + fp.Rule
	default:
		return fp.Rule
	}
}

// Diff holds the outcome of comparing a check result against a baseline.
type Diff struct {
	// Known is the number of current violations present in the baseline
	// (pre-existing debt — tolerated).
	Known int

	// New holds violations absent from the baseline — the ones that make
	// the check fail.
	New []Fingerprint

	// BaselineSize is the number of fingerprints in the loaded baseline.
	BaselineSize int
}

// Compare matches fingerprints against a baseline map. Fingerprints
// present in both are "known" debt; the rest are new violations.
func Compare(fps []Fingerprint, base map[string]string) Diff {
	diff := Diff{New: []Fingerprint{}, BaselineSize: len(base)}
	for _, fp := range fps {
		if _, ok := base[fp.String()]; ok {
			diff.Known++
		} else {
			diff.New = append(diff.New, fp)
		}
	}
	return diff
}

// FilterResult removes baseline-known violations from a check result so
// downstream rendering and the exit code reflect only NEW violations.
// Returns the filtered result and the number of tolerated (known)
// violations. SuppressedCount passes through unchanged — suppression
// and baselining are independent mechanisms.
func FilterResult(result models.CheckResult, base map[string]string) (models.CheckResult, int) {
	known := 0

	filtered := models.CheckResult{
		DependencyWarnings: make([]models.CheckArchWarningDependency, 0, len(result.DependencyWarnings)),
		MatchWarnings:      make([]models.CheckArchWarningMatch, 0, len(result.MatchWarnings)),
		DeepscanWarnings:   make([]models.CheckArchWarningDeepscan, 0, len(result.DeepscanWarnings)),
		NamingWarnings:     make([]models.CheckArchWarningNaming, 0, len(result.NamingWarnings)),
		SuppressedCount:    result.SuppressedCount,
	}

	isKnown := func(fp Fingerprint) bool {
		if _, ok := base[fp.String()]; ok {
			known++
			return true
		}
		return false
	}

	for _, w := range result.DependencyWarnings {
		if !isKnown(depFingerprint(w)) {
			filtered.DependencyWarnings = append(filtered.DependencyWarnings, w)
		}
	}
	for _, w := range result.MatchWarnings {
		if !isKnown(matchFingerprint(w)) {
			filtered.MatchWarnings = append(filtered.MatchWarnings, w)
		}
	}
	for _, w := range result.DeepscanWarnings {
		if !isKnown(deepscanFingerprint(w)) {
			filtered.DeepscanWarnings = append(filtered.DeepscanWarnings, w)
		}
	}
	for _, w := range result.NamingWarnings {
		if !isKnown(namingFingerprint(w)) {
			filtered.NamingWarnings = append(filtered.NamingWarnings, w)
		}
	}

	return filtered, known
}

// parentDir returns the directory part of path ("" for bare names).
func parentDir(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[:i]
	}
	return ""
}
