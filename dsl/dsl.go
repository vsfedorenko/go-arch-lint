// Package dsl is the Path-based architecture DSL.
//
// The entire language is four calls:
//
//	Path(path, fn?)        a directory (trailing "**" = the whole subtree).
//	                       A directory containing Go code IS a component.
//	Use(targets...)        the ONLY rule: "this path uses these targets".
//	Vendor(name, import)   an external package; a legal Use target.
//	Spec(fn)               the single entry point.
//
// Defaults: nothing may use anything until a Use says so. The order of
// declarations mirrors the direction of dependencies (referring forward is
// a Go compile error). Malformed specs panic at build time with file:line
// (duplicate paths, Use outside a Path fn, self-use, raw strings as
// targets, misplaced "**"). Filesystem verification of declared paths and
// did-you-mean suggestions arrive with the checker-pipeline integration
// (stage 2 of the v3 roadmap).
//
// This package is experimental: the API may change before it replaces the
// first-generation dsl package.
package dsl

import (
	"fmt"
	"path"
	"runtime"
	"strings"
)

// PathID identifies a declared path (component). Produced by Path.
type PathID struct {
	name string
	spec *SpecBuilder
}

// VendorID identifies a declared vendor. Produced by Vendor.
type VendorID struct {
	name string
	spec *SpecBuilder
}

// Entry is one declared path.
type Entry struct {
	// Full is the slash-joined path from the module root ("**" kept).
	Full string
	// Subtree is true when the path ends with "**".
	Subtree bool
	// File/Line point at the Path() call that declared it.
	File string
	Line int
}

// VendorEntry is one declared vendor.
type VendorEntry struct {
	Name        string
	ImportPaths []string
	File        string
	Line        int
}

// UseEntry is one Use rule: the using path plus its targets.
type UseEntry struct {
	From  string
	Paths []string
	Vends []string
	File  string
	Line  int
}

// SpecBuilder accumulates everything declared inside Spec.
type SpecBuilder struct {
	// paths: full path -> entry. Order preserved in pathOrder.
	paths     map[string]*Entry
	pathOrder []string
	// vendors: name -> entry.
	vendors map[string]*VendorEntry
	// uses: from-path -> rule (one per path; second Use for the same
	// from inside the SAME fn merges — see Use).
	uses     map[string]*UseEntry
	useOrder []string
	// declared tracks every PathID/VendorID that got its declaration
	// executed (assigned), so Use(unusedID) is detectable.
	declared map[string]bool

	// top is the innermost Path fn being executed (nil outside Path fns).
	top      *frame
	excluded []string
}

// Build is the final, immutable spec (fields filled by the pipeline glue).
type Build struct {
	Paths    map[string]Entry
	Order    []string
	Vendors  map[string]VendorEntry
	Uses     map[string]UseEntry
	Excluded []string
}

// callerRef returns file:line of the DSL call site (the direct caller).
func callerRef() (string, int) {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "", 0
	}
	parts := strings.Split(file, "/")
	return parts[len(parts)-1], line
}

// Spec builds the architecture description. fn must contain every Path,
// Use and Vendor call. The returned Build is ready for the checker.
func Spec(fn func(s *SpecBuilder)) *Build {
	s := &SpecBuilder{
		paths:    map[string]*Entry{},
		vendors:  map[string]*VendorEntry{},
		uses:     map[string]*UseEntry{},
		declared: map[string]bool{},
	}
	fn(s)
	return s.finish()
}

// Path declares a directory relative to the module root. With fn it also
// nests children and its own Use rules; without fn it is a leaf.
//
//	path := Path("internal/core")
//	Path("internal", func() { ... children ... })
//	all := Path("legacy/**")  // the whole subtree is one component
//	root := Path(".")         // the module root itself is a component
func (s *SpecBuilder) Path(p string, fn ...func()) PathID {
	file, line := callerRef()

	// A child Path joins the enclosing Path's prefix; at the top level
	// the prefix is empty. Absolute-looking inputs ("/x") stay top-level
	// by convention: leading slashes are stripped before joining.
	rel := strings.TrimPrefix(p, "/")
	var clean string
	if s.top != nil && !strings.HasPrefix(p, "/") {
		clean = path.Join(s.top.from, rel)
	} else {
		clean = strings.TrimPrefix(path.Clean("/"+rel), "/")
	}
	// The module root is a legal path with the canonical key "." (never
	// "" — an empty PathID name is the zero value, and an empty component
	// name renders as a blank "Component  shouldn't depend on" line).
	if clean == "" && p != "." && p != "" && p != "/" {
		panic(fmt.Errorf("%s:%d: Path(%q) — empty path", file, line, p))
	}
	if clean == "" && p == "" {
		panic(fmt.Errorf("%s:%d: Path(\"\") — empty path", file, line))
	}
	if clean == "" {
		clean = "."
	}
	subtree := strings.HasSuffix(clean, "/**")
	if subtree {
		clean = strings.TrimSuffix(clean, "/**")
		if clean == "" {
			panic(fmt.Errorf("%s:%d: Path(\"**\") — a bare ** would swallow the module; declare real paths", file, line))
		}
	}
	if strings.Contains(clean, "**") {
		panic(fmt.Errorf("%s:%d: Path(%q) — ** is only allowed as a trailing \"/**\"", file, line, p))
	}
	if id, dup := s.paths[clean]; dup {
		panic(fmt.Errorf("%s:%d: Path(%q) declared twice — first at %s:%d", file, line, clean, id.File, id.Line))
	}

	e := &Entry{Full: clean, Subtree: subtree, File: file, Line: line}
	s.paths[clean] = e
	s.pathOrder = append(s.pathOrder, clean)
	s.declared[clean] = true

	for _, f := range fn {
		fr := s.pushFrame(clean)
		f()
		s.popFrame(fr)
	}
	return PathID{name: clean, spec: s}
}

// Vendor declares an external dependency as a named Use target.
func (s *SpecBuilder) Vendor(name string, importPaths ...string) VendorID {
	file, line := callerRef()
	if name == "" || len(importPaths) == 0 {
		panic(fmt.Errorf("%s:%d: Vendor(name, imports...) — name and at least one import are required", file, line))
	}
	for _, imp := range importPaths {
		if imp == "" {
			panic(fmt.Errorf("%s:%d: Vendor(%q) — import paths may not be empty", file, line, name))
		}
	}
	if v, dup := s.vendors[name]; dup {
		panic(fmt.Errorf("%s:%d: Vendor(%q) declared twice — first at %s:%d", file, line, name, v.File, v.Line))
	}
	s.vendors[name] = &VendorEntry{Name: name, ImportPaths: importPaths, File: file, Line: line}
	s.declared["vendor:"+name] = true
	return VendorID{name: name, spec: s}
}

// Use declares the ONLY rule: the enclosing path uses the listed targets.
// Must be called inside a Path fn. Path and Vendor targets mix freely.
//
//	Path("internal/core", func() {
//	    Use(domain, pgx)   // core uses domain and pgx
//	})
func (s *SpecBuilder) Use(targets ...any) {
	file, line := callerRef()

	if s.top == nil {
		panic(fmt.Errorf("%s:%d: Use(...) must be called inside Path(path, func(){...})", file, line))
	}
	// top.from is never empty: Path() maps the module root to the
	// canonical key "." (see Path), so no empty-from guard is needed.
	from := s.top.from

	u, exists := s.uses[from]
	if !exists {
		u = &UseEntry{From: from, File: file, Line: line}
		s.uses[from] = u
		s.useOrder = append(s.useOrder, from)
	}

	for _, t := range targets {
		switch v := t.(type) {
		case PathID:
			if v.name == "" || v.spec == nil {
				panic(fmt.Errorf("%s:%d: Use(<empty PathID>) — the variable was never assigned from Path(...)", file, line))
			}
			if !s.declared[v.name] {
				panic(fmt.Errorf("%s:%d: Use(%q) — path is not declared in this spec (declared in another spec?)", file, line, v.name))
			}
			if v.name == from {
				panic(fmt.Errorf("%s:%d: Use(%q) — a path using itself is meaningless", file, line, v.name))
			}
			u.Paths = append(u.Paths, v.name)
		case VendorID:
			if v.name == "" || v.spec == nil {
				panic(fmt.Errorf("%s:%d: Use(<empty VendorID>) — the variable was never assigned from Vendor(...)", file, line))
			}
			if !s.declared["vendor:"+v.name] {
				panic(fmt.Errorf("%s:%d: Use(vendor %q) — vendor is not declared in this spec", file, line, v.name))
			}
			u.Vends = append(u.Vends, v.name)
		default:
			panic(fmt.Errorf("%s:%d: Use(%T) — only PathID and VendorID values are legal targets, declare paths with Path(...) and vendors with Vendor(...)", file, line, v))
		}
	}
}

// frame is one active Path fn on the builder's stack; Use reads the top
// to know its enclosing path. A stack on the builder (not a package
// global) keeps nested/reentrant Specs safe.
type frame struct {
	from string
	prev *frame
}

// pushFrame/popFrame are unexported helpers used by Path.
func (s *SpecBuilder) pushFrame(from string) *frame {
	f := &frame{from: from, prev: s.top}
	s.top = f
	return f
}

func (s *SpecBuilder) popFrame(f *frame) { s.top = f.prev }

// finish freezes the builder into an immutable Build.
func (s *SpecBuilder) finish() *Build {
	b := &Build{
		Paths:    map[string]Entry{},
		Order:    append([]string(nil), s.pathOrder...),
		Vendors:  map[string]VendorEntry{},
		Uses:     map[string]UseEntry{},
		Excluded: append([]string(nil), s.excluded...),
	}
	for k, v := range s.paths {
		b.Paths[k] = *v
	}
	for k, v := range s.vendors {
		b.Vendors[k] = *v
	}
	for k, v := range s.uses {
		b.Uses[k] = *v
	}
	return b
}

// Suggest returns the closest declared name to typo ("" when none is
// close enough). Used by the checker and by tests of diagnostics.
func Suggest(typo string, candidates []string) string {
	best, bestDist := "", -1
	for _, c := range candidates {
		d := levenshtein(typo, c)
		if bestDist == -1 || d < bestDist {
			best, bestDist = c, d
		}
	}
	if bestDist >= 0 && bestDist <= (len(typo)+len(best))/3 {
		return best
	}
	return ""
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			m := prev[j] + 1
			if cur[j-1]+1 < m {
				m = cur[j-1] + 1
			}
			if prev[j-1]+cost < m {
				m = prev[j-1] + cost
			}
			cur[j] = m
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

// Exclude removes directories from the checked tree. Nested modules
// (a directory with its own go.mod), generated code and examples are
// not architecture — declare them excluded instead of listing every
// package. Paths are module-root relative; a trailing "/**" excludes a
// whole subtree.
func (s *SpecBuilder) Exclude(paths ...string) {
	file, line := callerRef()
	if len(paths) == 0 {
		panic(fmt.Errorf("%s:%d: Exclude(paths...) — at least one path is required", file, line))
	}
	for _, p := range paths {
		if p == "" {
			panic(fmt.Errorf("%s:%d: Exclude(%q) — path may not be empty", file, line, p))
		}
		s.excluded = append(s.excluded, p)
	}
}
