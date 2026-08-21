// Package v2 is the second-generation architecture DSL.
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
// a Go compile error). Paths are verified against the real filesystem at
// spec-build time — a typo panics with file:line and a suggestion.
//
// This package is experimental: the API may change before it replaces the
// first-generation dsl package.
package v2

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
	Name       string
	ImportPath string
	File       string
	Line       int
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
	top *frame
}

// Build is the final, immutable spec (fields filled by the pipeline glue).
type Build struct {
	Paths   map[string]Entry
	Order   []string
	Vendors map[string]VendorEntry
	Uses    map[string]UseEntry
}

// callerRef returns file:line of the DSL call site (skip=1 → the caller).
func callerRef(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip + 1)
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
func (s *SpecBuilder) Path(p string, fn ...func()) PathID {
	file, line := callerRef(1)

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
	if clean == "" || clean == "." {
		panic(fmt.Errorf("%s:%d: Path(\"\") — empty path", file, line))
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
func (s *SpecBuilder) Vendor(name, importPath string) VendorID {
	file, line := callerRef(1)
	if name == "" || importPath == "" {
		panic(fmt.Errorf("%s:%d: Vendor(name, importPath) — neither argument may be empty", file, line))
	}
	if v, dup := s.vendors[name]; dup {
		panic(fmt.Errorf("%s:%d: Vendor(%q) declared twice — first at %s:%d", file, line, name, v.File, v.Line))
	}
	s.vendors[name] = &VendorEntry{Name: name, ImportPath: importPath, File: file, Line: line}
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
	file, line := callerRef(1)

	if s.top == nil {
		panic(fmt.Errorf("%s:%d: Use(...) must be called inside Path(path, func(){...})", file, line))
	}
	from := s.top.from
	if from == "" {
		panic(fmt.Errorf("%s:%d: Use(...) must be called inside Path(path, func(){...})", file, line))
	}

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
		Paths:   map[string]Entry{},
		Order:   append([]string(nil), s.pathOrder...),
		Vendors: map[string]VendorEntry{},
		Uses:    map[string]UseEntry{},
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
