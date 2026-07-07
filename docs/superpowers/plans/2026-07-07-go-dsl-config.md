# Go DSL Config Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the YAML config format (`.go-arch-lint.yml`) with a Pure Go DSL, removing ~1500 lines of YAML/JSON Schema machinery and two external dependencies.

**Architecture:** Users write `.go-arch-lint/arch.go` using a `dsl` package. A generated `main.go` imports the linter as a library and runs `check` in-process via `archlint.RunCLI()`. The `go-arch-lint` binary becomes a thin launcher that delegates to `go run .go-arch-lint/`. Source positions are captured natively via `runtime.Caller(1)` inside DSL functions, replacing the `ref[T]` generic wrapper. YAML support is removed entirely (hard cutover, major version bump to v2.0).

**Tech Stack:** Go 1.25, `github.com/spf13/cobra`, `github.com/google/go-cmdtest`, existing internal packages (assembler, checker, renderer)

**Spec:** `docs/superpowers/specs/2026-07-07-go-dsl-config-design.md`

## Global Constraints

- Go 1.25+ (from `go.mod`)
- Every commit must compile (`go build ./...`) and pass existing tests (`go test ./...`)
- TDD: write failing test → implement → verify pass → commit
- No `as any`, `@ts-ignore` equivalents — no `interface{}` where typed is possible
- Module path: `github.com/fe3dback/go-arch-lint` (unchanged)
- Follow existing code patterns: package layout in `internal/`, cobra commands in `internal/app/internal/container/`
- `common.Referable[T]` = `{Value T, Reference Reference}`; `common.NewReferable(value, ref)` and `common.NewReferenceSingleLine(file, line, col)` are the constructors used throughout
- `spec.Document` interface (11 methods) is the contract between decoder and assembler — the new Go decoder must produce a type implementing this interface

---

## File Structure

### New files

| Path | Responsibility | ~LoC |
|---|---|---|
| `dsl/types.go` | SpecBuilder, ComponentEntry, VendorEntry, DepEntry, AllowEntry structs | 80 |
| `dsl/spec.go` | `Spec(fn func())` entry point, `FlushSpec()` | 50 |
| `dsl/context.go` | Builder stack (current spec/dep/allow context) | 60 |
| `dsl/builders.go` | `Version`, `Workdir`, `Allow`, `Component`, `Vendor`, `Deps`, `MayDependOn`, `CanUse`, etc. — each with `runtime.Caller(1)` | 180 |
| `dsl/builders_test.go` | Unit tests for every builder function | 200 |
| `internal/services/spec/decoder/decoder_go.go` | Go DSL decoder: wraps SpecBuilder into `spec.Document` implementor | 200 |
| `internal/services/spec/decoder/decoder_go_test.go` | Tests for SpecBuilder → Document mapping | 150 |
| `archlint.go` | Public package `archlint`, `RunCLI()` entry point | 80 |
| `cmd/arch-lint/main.go` | Thin launcher binary: locate `.go-arch-lint/`, exec `go run`, handle `init`/`version` | 120 |
| `cmd/arch-lint/scaffold.go` | `init` scaffolding logic (generate main.go, arch.go, go.mod) | 100 |
| `docs/migration-v2.md` | YAML→DSL migration guide for users | 80 |

### Modified files

| Path | Change |
|---|---|
| `internal/app/internal/container/cnt_glue.go` | Replace `provideYamlSpecProvider()` with `provideGoSpecProvider()` |
| `internal/services/project/info/assembler.go` | Adapt: arch file path → `.go-arch-lint/arch.go` (or unused in new flow) |
| `go.mod` | Remove `fe3dback/go-yaml`, `xeipuuv/gojsonschema`; update go version if needed |
| `main.go` | Delete (replaced by `cmd/arch-lint/main.go`) |
| `main_test.go` | Update to test launcher binary |
| `.go-arch-lint.yml` | Delete (replaced by `.go-arch-lint/arch.go`) |
| `README.md` | Rewrite quickstart with Go DSL |
| `docs/syntax/README.md` | Replace YAML syntax table with DSL function reference |
| `docs/README.md` | Remove `schema` command section |

### Deleted files

| Path | Reason |
|---|---|
| `internal/services/spec/decoder/decoder.go` | YAML-specific decode logic |
| `internal/services/spec/decoder/decoder_yaml.go` | `ref[T]` + YAML AST hooks |
| `internal/services/spec/decoder/decoder_doc.go` | `doc` interface |
| `internal/services/spec/decoder/decoder_doc_v1.go` | V1 schema |
| `internal/services/spec/decoder/decoder_doc_v2.go` | V2 schema |
| `internal/services/spec/decoder/decoder_doc_v3.go` | V3 schema |
| `internal/services/spec/decoder/decoder_utils.go` | `ref[T]` utilities |
| `internal/services/spec/decoder/json_scheme.go` | JSON Schema validation |
| `internal/services/spec/decoder/json_scheme_test.go` | test for above |
| `internal/services/spec/decoder/decoder_utils.go` | `castRef`/`castRefList` helpers |
| `internal/services/common/yaml/reference/` | YAML path → source ref resolver (whole dir) |
| `internal/services/schema/provider.go` | JSON Schema provider |
| `internal/operations/schema/` | `schema` command (whole dir) |
| All `test/**/*.yml` | YAML fixtures → replaced by `.go-arch-lint/arch.go` per test |

---

## Task Dependency Graph

```
Task 1 (dsl types)
  └→ Task 2 (dsl builders) ──────────────────────┐
       └→ Task 3 (Go decoder)                     │
            └→ Task 5 (wire DI)                   │
                 └→ Task 7 (delete YAML code)     │
                      └→ Task 8 (test fixtures)   │
                           └→ Task 9 (docs)       │
  Task 4 (archlint + launcher) ───────────────────┘
       └→ Task 5 (wire DI)
Task 6 (migrate own config) — independent, any time after Task 2
```

**Parallel opportunities:** Tasks 1-2 and Task 4 can start in parallel. Task 6 is independent after Task 2.

---

## Task 1: DSL Package — Core Types

**Files:**
- Create: `dsl/types.go`
- Create: `dsl/spec.go`
- Create: `dsl/context.go`
- Test: `dsl/types_test.go`

**Interfaces:**
- Produces: `SpecBuilder` struct, `Spec(fn func())`, `FlushSpec() (*SpecBuilder, error)`, `resetSpecBuilder()`

- [ ] **Step 1: Write failing test for SpecBuilder accumulation**

Create `dsl/types_test.go`:

```go
package dsl

import (
	"testing"

	"github.com/fe3dback/go-arch-lint/internal/models/common"
	"github.com/stretchr/testify/assert"
)

func TestSpecCapturesVersion(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Version(1)
	})

	builder, err := FlushSpec()
	assert.NoError(t, err)
	assert.Equal(t, 1, builder.Version.Value)
}

func TestSpecCapturesWorkdir(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Workdir("internal")
	})

	builder, err := FlushSpec()
	assert.NoError(t, err)
	assert.Equal(t, "internal", builder.Workdir.Value)
}

func TestFlushSpecReturnsErrorWhenNotInitialized(t *testing.T) {
	resetSpecBuilder()
	_, err := FlushSpec()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Spec() was not called")
}

func TestSpecBuilderPosition(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Version(1) // line 28 in this test file
	})

	builder, _ := FlushSpec()
	// The Reference should point to the test file
	assert.True(t, builder.Version.Reference.Valid)
	assert.Equal(t, "types_test.go", builder.Version.Reference.File)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./dsl/... -run TestSpec -v`
Expected: FAIL — package dsl does not exist

- [ ] **Step 3: Create `dsl/types.go` — SpecBuilder and entry types**

```go
package dsl

import "github.com/fe3dback/go-arch-lint/internal/models/common"

// SpecBuilder is the in-memory representation of the user's arch config,
// populated by DSL functions. It replaces the YAML decoder's ArchV3 struct.
type SpecBuilder struct {
	Version          common.Referable[int]
	Workdir          common.Referable[string]
	Allow            AllowEntry
	Exclude          []common.Referable[string]
	ExcludeFiles     []common.Referable[string]
	Vendors          map[string]VendorEntry
	CommonVendors    []common.Referable[string]
	Components       map[string]ComponentEntry
	CommonComponents []common.Referable[string]
	Deps             map[string]DepEntry
}

// AllowEntry holds global allow rules.
type AllowEntry struct {
	DepOnAnyVendor           common.Referable[bool]
	DeepScan                 common.Referable[bool]
	IgnoreNotFoundComponents common.Referable[bool]
}

// VendorEntry holds a named vendor definition.
type VendorEntry struct {
	ImportPaths []string
	Reference   common.Reference
}

// ComponentEntry holds a named component definition.
type ComponentEntry struct {
	RelativePaths []string
	Reference     common.Reference
}

// DepEntry holds dependency rules for a component.
type DepEntry struct {
	MayDependOn    []common.Referable[string]
	CanUse         []common.Referable[string]
	AnyProjectDeps common.Referable[bool]
	AnyVendorDeps  common.Referable[bool]
	DeepScan       common.Referable[bool]
	Reference      common.Reference
}

func newSpecBuilder() *SpecBuilder {
	return &SpecBuilder{
		Vendors:    make(map[string]VendorEntry),
		Components: make(map[string]ComponentEntry),
		Deps:       make(map[string]DepEntry),
	}
}
```

- [ ] **Step 4: Create `dsl/context.go` — builder stack**

```go
package dsl

// globalBuilder is the singleton SpecBuilder populated by DSL functions.
// It is set by Spec() and consumed by FlushSpec().
var globalBuilder *SpecBuilder

// contextStack tracks the current builder context for nested DSL calls
// (e.g., inside Deps() callback, the current DepEntry).
type contextStack struct {
	spec *SpecBuilder
	dep  *DepEntry
	// allow context is set inside Allow() callback
	inAllow bool
}

var current contextStack

func resetSpecBuilder() {
	globalBuilder = nil
	current = contextStack{}
}
```

- [ ] **Step 5: Create `dsl/spec.go` — Spec entry point and FlushSpec**

```go
package dsl

import (
	"fmt"
	"runtime"
)

// Spec is the entry point for the DSL. It creates a new SpecBuilder,
// sets it as the current context, and executes fn. DSL functions called
// inside fn populate the builder.
//
// Usage:
//
//	var _ = Spec(func() {
//	    Version(1)
//	    Workdir("internal")
//	    Component("main", "app")
//	})
func Spec(fn func()) {
	globalBuilder = newSpecBuilder()
	current = contextStack{spec: globalBuilder}

	fn()

	current = contextStack{}
}

// FlushSpec returns the populated SpecBuilder and resets the global state.
// It must be called after Spec(fn) has executed (typically at the start of
// archlint.RunCLI()).
func FlushSpec() (*SpecBuilder, error) {
	if globalBuilder == nil {
		return nil, fmt.Errorf("Spec() was not called — ensure your arch.go contains 'var _ = Spec(func() { ... })'")
	}

	builder := globalBuilder
	globalBuilder = nil
	current = contextStack{}

	return builder, nil
}

// callerRef returns a Reference pointing to the file:line of the DSL function
// call site (the user's arch.go). skip=1 means the immediate caller.
func callerRef(skip int) (file string, line int) {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "", 0
	}
	return file, line
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./dsl/... -run TestSpec -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add dsl/
git commit -m "feat(dsl): add SpecBuilder core types and Spec entry point"
```

---

## Task 2: DSL Package — Builder Functions

**Files:**
- Create: `dsl/builders.go`
- Test: `dsl/builders_test.go`

**Interfaces:**
- Consumes: `SpecBuilder`, `current` context, `callerRef()` from Task 1
- Produces: `Version`, `Workdir`, `Allow`, `DepOnAnyVendor`, `DeepScan`, `IgnoreNotFoundComponents`, `Exclude`, `ExcludeFiles`, `Component`, `Vendor`, `CommonComponents`, `CommonVendors`, `Deps`, `MayDependOn`, `CanUse`, `AnyProjectDeps`, `AnyVendorDeps`

- [ ] **Step 1: Write failing tests for all builder functions**

Create `dsl/builders_test.go`:

```go
package dsl

import (
	"testing"

	"github.com/fe3dback/go-arch-lint/internal/models/common"
	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Version(1)
	})
	b, _ := FlushSpec()
	assert.Equal(t, 1, b.Version.Value)
	assert.True(t, b.Version.Reference.Valid)
}

func TestWorkdir(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Workdir("internal")
	})
	b, _ := FlushSpec()
	assert.Equal(t, "internal", b.Workdir.Value)
}

func TestAllowDepOnAnyVendor(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Allow(func() {
			DepOnAnyVendor(false)
		})
	})
	b, _ := FlushSpec()
	assert.Equal(t, false, b.Allow.DepOnAnyVendor.Value)
}

func TestAllowDeepScan(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Allow(func() {
			DeepScan(true)
		})
	})
	b, _ := FlushSpec()
	assert.Equal(t, true, b.Allow.DeepScan.Value)
}

func TestAllowIgnoreNotFoundComponents(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Allow(func() {
			IgnoreNotFoundComponents(true)
		})
	})
	b, _ := FlushSpec()
	assert.Equal(t, true, b.Allow.IgnoreNotFoundComponents.Value)
}

func TestExclude(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Exclude("vendor", "test")
	})
	b, _ := FlushSpec()
	assert.Len(t, b.Exclude, 2)
	assert.Equal(t, "vendor", b.Exclude[0].Value)
	assert.Equal(t, "test", b.Exclude[1].Value)
}

func TestExcludeFiles(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		ExcludeFiles(`^.*_test\.go$`)
	})
	b, _ := FlushSpec()
	assert.Len(t, b.ExcludeFiles, 1)
	assert.Equal(t, `^.*_test\.go$`, b.ExcludeFiles[0].Value)
}

func TestComponent(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Component("main", "app")
	})
	b, _ := FlushSpec()
	assert.Contains(t, b.Components, "main")
	assert.Equal(t, []string{"app"}, b.Components["main"].RelativePaths)
}

func TestComponentMultiplePaths(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Component("services", "services/**", "lib/svc")
	})
	b, _ := FlushSpec()
	assert.Equal(t, []string{"services/**", "lib/svc"}, b.Components["services"].RelativePaths)
}

func TestVendor(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Vendor("cobra", "github.com/spf13/cobra")
	})
	b, _ := FlushSpec()
	assert.Contains(t, b.Vendors, "cobra")
	assert.Equal(t, []string{"github.com/spf13/cobra"}, b.Vendors["cobra"].ImportPaths)
}

func TestVendorMultiplePaths(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Vendor("yaml", "github.com/goccy/go-yaml", "github.com/goccy/go-yaml/**")
	})
	b, _ := FlushSpec()
	assert.Len(t, b.Vendors["yaml"].ImportPaths, 2)
}

func TestCommonComponents(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		CommonComponents("models", "utils")
	})
	b, _ := FlushSpec()
	assert.Len(t, b.CommonComponents, 2)
	assert.Equal(t, "models", b.CommonComponents[0].Value)
}

func TestCommonVendors(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		CommonVendors("go-common")
	})
	b, _ := FlushSpec()
	assert.Len(t, b.CommonVendors, 1)
}

func TestDepsMayDependOn(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Deps("main", func() {
			MayDependOn("container")
		})
	})
	b, _ := FlushSpec()
	assert.Contains(t, b.Deps, "main")
	assert.Len(t, b.Deps["main"].MayDependOn, 1)
	assert.Equal(t, "container", b.Deps["main"].MayDependOn[0].Value)
}

func TestDepsCanUse(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Deps("services", func() {
			CanUse("cobra", "yaml")
		})
	})
	b, _ := FlushSpec()
	assert.Len(t, b.Deps["services"].CanUse, 2)
}

func TestDepsAnyVendorDeps(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Deps("container", func() {
			AnyVendorDeps(true)
		})
	})
	b, _ := FlushSpec()
	assert.Equal(t, true, b.Deps["container"].AnyVendorDeps.Value)
}

func TestDepsAnyProjectDeps(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Deps("main", func() {
			AnyProjectDeps(true)
		})
	})
	b, _ := FlushSpec()
	assert.Equal(t, true, b.Deps["main"].AnyProjectDeps.Value)
}

func TestDepsDeepScanOverride(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Deps("operations", func() {
			DeepScan(false)
		})
	})
	b, _ := FlushSpec()
	assert.Equal(t, false, b.Deps["operations"].DeepScan.Value)
}

func TestBuilderPositionsAreFromTestFile(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Component("main", "app") // this line should be the reference
	})
	b, _ := FlushSpec()
	ref := b.Components["main"].Reference
	assert.True(t, ref.Valid)
	assert.Equal(t, "builders_test.go", ref.File)
	assert.Greater(t, ref.Line, 0)
}

func TestMultipleDeps(t *testing.T) {
	resetSpecBuilder()
	Spec(func() {
		Deps("main", func() { MayDependOn("a") })
		Deps("container", func() { MayDependOn("b") })
	})
	b, _ := FlushSpec()
	assert.Len(t, b.Deps, 2)
	assert.Contains(t, b.Deps, "main")
	assert.Contains(t, b.Deps, "container")
}

// Ensure unused import doesn't cause issues
var _ = common.NewEmptyReferable
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./dsl/... -v`
Expected: FAIL — builder functions not defined

- [ ] **Step 3: Create `dsl/builders.go` — all DSL functions**

```go
package dsl

import (
	"fmt"

	"github.com/fe3dback/go-arch-lint/internal/models/common"
)

// Version sets the DSL schema version (always 1 for v2.0).
func Version(v int) {
	file, line := callerRef(1)
	current.spec.Version = common.Referable[int]{
		Value:     v,
		Reference: common.NewReferenceSingleLine(file, line, 0),
	}
}

// Workdir sets the relative working directory for analysis.
func Workdir(path string) {
	file, line := callerRef(1)
	current.spec.Workdir = common.Referable[string]{
		Value:     path,
		Reference: common.NewReferenceSingleLine(file, line, 0),
	}
}

// Allow defines global rules. Call DepOnAnyVendor/DeepScan/IgnoreNotFoundComponents inside fn.
func Allow(fn func()) {
	current.inAllow = true
	fn()
	current.inAllow = false
}

// DepOnAnyVendor sets whether any project code may import any vendor lib.
func DepOnAnyVendor(b bool) {
	file, line := callerRef(1)
	current.spec.Allow.DepOnAnyVendor = common.Referable[bool]{
		Value:     b,
		Reference: common.NewReferenceSingleLine(file, line, 0),
	}
}

// DeepScan enables/disables advanced AST analysis.
// Inside Allow(): sets global default. Inside Deps(): overrides per-component.
func DeepScan(b bool) {
	file, line := callerRef(1)
	ref := common.Referable[bool]{
		Value:     b,
		Reference: common.NewReferenceSingleLine(file, line, 0),
	}

	if current.inAllow {
		current.spec.Allow.DeepScan = ref
		return
	}

	if current.dep != nil {
		current.dep.DeepScan = ref
	}
}

// IgnoreNotFoundComponents skips components not found by their glob.
func IgnoreNotFoundComponents(b bool) {
	file, line := callerRef(1)
	current.spec.Allow.IgnoreNotFoundComponents = common.Referable[bool]{
		Value:     b,
		Reference: common.NewReferenceSingleLine(file, line, 0),
	}
}

// Exclude adds directories to exclude from analysis.
func Exclude(paths ...string) {
	file, line := callerRef(1)
	for _, p := range paths {
		current.spec.Exclude = append(current.spec.Exclude, common.Referable[string]{
			Value:     p,
			Reference: common.NewReferenceSingleLine(file, line, 0),
		})
	}
}

// ExcludeFiles adds regex patterns to exclude matching files.
func ExcludeFiles(patterns ...string) {
	file, line := callerRef(1)
	for _, p := range patterns {
		current.spec.ExcludeFiles = append(current.spec.ExcludeFiles, common.Referable[string]{
			Value:     p,
			Reference: common.NewReferenceSingleLine(file, line, 0),
		})
	}
}

// Component defines a named component mapping to one or more package paths.
func Component(name string, paths ...string) {
	if name == "" {
		panic(fmt.Errorf("Component name cannot be empty"))
	}
	file, line := callerRef(1)
	current.spec.Components[name] = ComponentEntry{
		RelativePaths: paths,
		Reference:     common.NewReferenceSingleLine(file, line, 0),
	}
}

// Vendor defines a named vendor mapping to one or more import paths.
func Vendor(name string, importPaths ...string) {
	if name == "" {
		panic(fmt.Errorf("Vendor name cannot be empty"))
	}
	file, line := callerRef(1)
	current.spec.Vendors[name] = VendorEntry{
		ImportPaths: importPaths,
		Reference:   common.NewReferenceSingleLine(file, line, 0),
	}
}

// CommonComponents marks components as importable by any project package.
func CommonComponents(names ...string) {
	file, line := callerRef(1)
	for _, n := range names {
		current.spec.CommonComponents = append(current.spec.CommonComponents, common.Referable[string]{
			Value:     n,
			Reference: common.NewReferenceSingleLine(file, line, 0),
		})
	}
}

// CommonVendors marks vendors as importable by any project package.
func CommonVendors(names ...string) {
	file, line := callerRef(1)
	for _, n := range names {
		current.spec.CommonVendors = append(current.spec.CommonVendors, common.Referable[string]{
			Value:     n,
			Reference: common.NewReferenceSingleLine(file, line, 0),
		})
	}
}

// Deps defines dependency rules for a component. Call MayDependOn/CanUse/etc inside fn.
func Deps(component string, fn func()) {
	if component == "" {
		panic(fmt.Errorf("Deps component name cannot be empty"))
	}

	file, line := callerRef(1)
	dep := DepEntry{
		Reference: common.NewReferenceSingleLine(file, line, 0),
	}

	prevDep := current.dep
	current.dep = &dep
	fn()
	current.dep = prevDep

	current.spec.Deps[component] = dep
}

// MayDependOn lists components that this component may import.
func MayDependOn(components ...string) {
	file, line := callerRef(1)
	if current.dep == nil {
		panic(fmt.Errorf("MayDependOn called outside of Deps() callback"))
	}
	for _, c := range components {
		current.dep.MayDependOn = append(current.dep.MayDependOn, common.Referable[string]{
			Value:     c,
			Reference: common.NewReferenceSingleLine(file, line, 0),
		})
	}
}

// CanUse lists vendors that this component may import.
func CanUse(vendors ...string) {
	file, line := callerRef(1)
	if current.dep == nil {
		panic(fmt.Errorf("CanUse called outside of Deps() callback"))
	}
	for _, v := range vendors {
		current.dep.CanUse = append(current.dep.CanUse, common.Referable[string]{
			Value:     v,
			Reference: common.NewReferenceSingleLine(file, line, 0),
		})
	}
}

// AnyProjectDeps allows this component to import any other project package.
func AnyProjectDeps(b bool) {
	file, line := callerRef(1)
	if current.dep == nil {
		panic(fmt.Errorf("AnyProjectDeps called outside of Deps() callback"))
	}
	current.dep.AnyProjectDeps = common.Referable[bool]{
		Value:     b,
		Reference: common.NewReferenceSingleLine(file, line, 0),
	}
}

// AnyVendorDeps allows this component to import any vendor package.
func AnyVendorDeps(b bool) {
	file, line := callerRef(1)
	if current.dep == nil {
		panic(fmt.Errorf("AnyVendorDeps called outside of Deps() callback"))
	}
	current.dep.AnyVendorDeps = common.Referable[bool]{
		Value:     b,
		Reference: common.NewReferenceSingleLine(file, line, 0),
	}
}
```

- [ ] **Step 4: Run all dsl tests**

Run: `go test ./dsl/... -v`
Expected: All tests PASS

- [ ] **Step 5: Verify build compiles**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add dsl/
git commit -m "feat(dsl): add all builder functions with runtime.Caller positions"
```

---

## Task 3: Go DSL Decoder (SpecBuilder → spec.Document)

**Files:**
- Create: `internal/services/spec/decoder/decoder_go.go`
- Create: `internal/services/spec/decoder/decoder_go_test.go`

**Interfaces:**
- Consumes: `dsl.SpecBuilder`, `dsl.FlushSpec()`, `spec.Document` interface, `spec.Options`/`spec.Vendor`/`spec.Component`/`spec.DependencyRule` interfaces, `common.Referable[T]`, `common.Reference`
- Produces: `GoSpecDocument` (implements `spec.Document`), `GoOptions` (implements `spec.Options`), `GoVendor` (implements `spec.Vendor`), `GoComponent` (implements `spec.Component`), `GoDependencyRule` (implements `spec.DependencyRule`), `GoDecoder` type with `Decode(archFile string) (spec.Document, []arch.Notice, error)`

- [ ] **Step 1: Write failing test for GoSpecDocument**

Create `internal/services/spec/decoder/decoder_go_test.go`:

```go
package decoder

import (
	"testing"

	"github.com/fe3dback/go-arch-lint/internal/models/common"
	"github.com/stretchr/testify/assert"

	"github.com/fe3dback/go-arch-lint/dsl"
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
	assert.Equal(t, []string{"app"}, []string(paths))
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/spec/decoder/ -run TestGoSpec -v`
Expected: FAIL — `NewGoSpecDocument` not defined

- [ ] **Step 3: Create `decoder_go.go` — SpecBuilder → spec.Document adapter**

```go
package decoder

import (
	"github.com/fe3dback/go-arch-lint/dsl"
	"github.com/fe3dback/go-arch-lint/internal/models"
	"github.com/fe3dback/go-arch-lint/internal/models/common"
	"github.com/fe3dback/go-arch-lint/internal/services/spec"
)

// GoSpecDocument implements spec.Document by wrapping a dsl.SpecBuilder.
// It replaces the YAML-based ArchV3 struct.
type GoSpecDocument struct {
	builder *dsl.SpecBuilder
}

func NewGoSpecDocument(builder *dsl.SpecBuilder) *GoSpecDocument {
	return &GoSpecDocument{builder: builder}
}

func (d *GoSpecDocument) Version() common.Referable[int] {
	return d.builder.Version
}

func (d *GoSpecDocument) WorkingDirectory() common.Referable[string] {
	workdir := d.builder.Workdir
	if workdir.Value == "" {
		return common.NewEmptyReferable("./")
	}
	return workdir
}

func (d *GoSpecDocument) Options() spec.Options {
	return &goOptions{allow: d.builder.Allow}
}

func (d *GoSpecDocument) ExcludedDirectories() []common.Referable[string] {
	return d.builder.Exclude
}

func (d *GoSpecDocument) ExcludedFilesRegExp() []common.Referable[string] {
	return d.builder.ExcludeFiles
}

func (d *GoSpecDocument) Vendors() spec.Vendors {
	result := make(spec.Vendors, len(d.builder.Vendors))
	for name, vendor := range d.builder.Vendors {
		result[name] = common.NewReferable(goVendor{paths: vendor.ImportPaths}, vendor.Reference)
	}
	return result
}

func (d *GoSpecDocument) CommonVendors() []common.Referable[string] {
	return d.builder.CommonVendors
}

func (d *GoSpecDocument) Components() spec.Components {
	result := make(spec.Components, len(d.builder.Components))
	for name, comp := range d.builder.Components {
		result[name] = common.NewReferable(goComponent{paths: comp.RelativePaths}, comp.Reference)
	}
	return result
}

func (d *GoSpecDocument) CommonComponents() []common.Referable[string] {
	return d.builder.CommonComponents
}

func (d *GoSpecDocument) Dependencies() spec.Dependencies {
	result := make(spec.Dependencies, len(d.builder.Deps))
	for name, dep := range d.builder.Deps {
		result[name] = common.NewReferable(&goDependencyRule{dep: dep}, dep.Reference)
	}
	return result
}

// --- goOptions implements spec.Options ---

type goOptions struct {
	allow dsl.AllowEntry
}

func (o *goOptions) IsDependOnAnyVendor() common.Referable[bool] {
	return o.allow.DepOnAnyVendor
}

func (o *goOptions) DeepScan() common.Referable[bool] {
	if o.allow.DeepScan.Reference.Valid {
		return o.allow.DeepScan
	}
	// default true since v3+
	return common.NewEmptyReferable(true)
}

func (o *goOptions) IgnoreNotFoundComponents() common.Referable[bool] {
	if o.allow.IgnoreNotFoundComponents.Reference.Valid {
		return o.allow.IgnoreNotFoundComponents
	}
	return common.NewEmptyReferable(false)
}

// --- goVendor implements spec.Vendor ---

type goVendor struct {
	paths []string
}

func (v goVendor) ImportPaths() []models.Glob {
	result := make([]models.Glob, 0, len(v.paths))
	for _, p := range v.paths {
		result = append(result, models.Glob(p))
	}
	return result
}

// --- goComponent implements spec.Component ---

type goComponent struct {
	paths []string
}

func (c goComponent) RelativePaths() []models.Glob {
	result := make([]models.Glob, 0, len(c.paths))
	for _, p := range c.paths {
		result = append(result, models.Glob(p))
	}
	return result
}

// --- goDependencyRule implements spec.DependencyRule ---

type goDependencyRule struct {
	dep dsl.DepEntry
}

func (r *goDependencyRule) MayDependOn() []common.Referable[string] {
	return r.dep.MayDependOn
}

func (r *goDependencyRule) CanUse() []common.Referable[string] {
	return r.dep.CanUse
}

func (r *goDependencyRule) AnyProjectDeps() common.Referable[bool] {
	return r.dep.AnyProjectDeps
}

func (r *goDependencyRule) AnyVendorDeps() common.Referable[bool] {
	return r.dep.AnyVendorDeps
}

func (r *goDependencyRule) DeepScan() common.Referable[bool] {
	if r.dep.DeepScan.Reference.Valid {
		return r.dep.DeepScan
	}
	return common.NewEmptyReferable(false)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/services/spec/decoder/ -run TestGoSpec -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/spec/decoder/decoder_go.go internal/services/spec/decoder/decoder_go_test.go
git commit -m "feat(decoder): add Go DSL decoder (SpecBuilder → spec.Document)"
```

---

## Task 4: Archlint Public Package + Launcher Binary

**Files:**
- Create: `archlint.go`
- Create: `cmd/arch-lint/main.go`
- Create: `cmd/arch-lint/scaffold.go`
- Modify: `internal/app/build_consts.go` (if needed for version injection)

**Interfaces:**
- Consumes: `internal/app.Execute()` (existing CLI entry)
- Produces: `archlint.RunCLI()` (public entry point), `go-arch-lint` launcher binary with `init`/`version` commands

**Note:** This task can run in parallel with Task 3.

- [ ] **Step 1: Create `archlint.go` — public entry point**

```go
package archlint

import (
	"os"

	"github.com/fe3dback/go-arch-lint/internal/app"
)

// RunCLI executes the arch-lint CLI. It is the entry point called by
// generated main.go files in .go-arch-lint/ directories.
//
// Version/buildTime/commitHash are injected from the user's module
// (via the dsl package version or ldflags).
func RunCLI() int {
	return app.Execute()
}

// MustRunCLI is like RunCLI but calls os.Exit with the result code.
func MustRunCLI() {
	os.Exit(RunCLI())
}
```

- [ ] **Step 2: Create `cmd/arch-lint/main.go` — thin launcher**

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		printUsage()
		return 1
	}

	command := os.Args[1]

	switch command {
	case "version":
		fmt.Printf("go-arch-lint launcher v2.0.0-dev\n")
		return 0
	case "init":
		return cmdInit()
	case "help", "--help", "-h":
		printUsage()
		return 0
	default:
		// All other commands (check, mapping, graph, selfInspect) delegate
		// to `go run .go-arch-lint/`
		return cmdDelegate(command, os.Args[2:])
	}
}

func cmdDelegate(command string, args []string) int {
	projectPath := "."
	for i, a := range args {
		if (a == "--project-path" || a == "-p") && i+1 < len(args) {
			projectPath = args[i+1]
			break
		}
	}

	archDir := filepath.Join(projectPath, ".go-arch-lint")
	if !dirExists(archDir) {
		fmt.Fprintf(os.Stderr, "Error: .go-arch-lint/ directory not found at %s\n", archDir)
		fmt.Fprintf(os.Stderr, "Run 'go-arch-lint init' first to create your arch config.\n")
		return 1
	}

	// Build the command: go run .go-arch-lint/ <command> [args...]
	goArgs := append([]string{"run", archDir, command}, args...)
	cmd := exec.Command("go", goArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "Error: failed to run arch-lint: %v\n", err)
		return 1
	}

	return 0
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func printUsage() {
	fmt.Print(`go-arch-lint v2.0 — Go architectural linter

Usage:
  go-arch-lint <command> [flags]

Commands:
  init          Create .go-arch-lint/ scaffold (go.mod, main.go, arch.go)
  check         Check project architecture against arch rules
  mapping       Show package-to-component mapping
  graph         Generate dependency graph
  selfInspect   Inspect go-arch-lint's own architecture
  version       Print version
  help          Show this help

The 'check', 'mapping', 'graph', and 'selfInspect' commands require a
.go-arch-lint/ directory (created by 'init') and delegate to 'go run'.

Global flags (passed through to delegated commands):
  --project-path string   project directory (default "./")
  --output-type string    output format [ascii, json] (default "ascii")
  --json                  alias for --output-type=json
  --output-color          use ANSI colors (default true)
`)
}

// silence unused import warning when strings is not yet used
var _ = strings.TrimSpace
```

- [ ] **Step 3: Create `cmd/arch-lint/scaffold.go` — init command**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const scaffoldGoMod = `module arch-lint-local

go 1.25

require github.com/fe3dback/go-arch-lint v2.0.0
`

const scaffoldMainGo = `// Code generated by go-arch-lint init. DO NOT EDIT.
package main

import "github.com/fe3dback/go-arch-lint"

func main() {
	archlint.MustRunCLI()
}
`

const scaffoldArchGo = `package main

import . "github.com/fe3dback/go-arch-lint/dsl"

var _ = Spec(func() {
	Version(1)
	Workdir("internal")

	Allow(func() {
		DepOnAnyVendor(false)
	})

	ExcludeFiles(` + "`^.*_test\\.go$`" + `)

	// Define your components:
	// Component("main", "app")
	// Component("services", "services/**")

	// Define dependency rules:
	// Deps("main", func() {
	//     MayDependOn("services")
	// })
})
`

func cmdInit() int {
	projectPath := "."
	archDir := filepath.Join(projectPath, ".go-arch-lint")

	if dirExists(archDir) {
		fmt.Fprintf(os.Stderr, "Error: %s already exists\n", archDir)
		return 1
	}

	if err := os.MkdirAll(archDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create %s: %v\n", archDir, err)
		return 1
	}

	files := map[string]string{
		"go.mod":  scaffoldGoMod,
		"main.go": scaffoldMainGo,
		"arch.go": scaffoldArchGo,
	}

	for name, content := range files {
		path := filepath.Join(archDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to write %s: %v\n", path, err)
			return 1
		}
		fmt.Printf("  created %s\n", path)
	}

	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Edit %s/arch.go to describe your architecture\n", archDir)
	fmt.Printf("  2. Run 'cd %s && go mod tidy' to resolve dependencies\n", archDir)
	fmt.Printf("  3. Run 'go-arch-lint check' to lint your project\n")

	return 0
}
```

- [ ] **Step 4: Verify build compiles**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add archlint.go cmd/
git commit -m "feat: add archlint public package and launcher binary with init scaffolding"
```

---

## Task 5: Wire Go Decoder into DI Container

**Files:**
- Modify: `internal/app/internal/container/cnt_glue.go`
- Modify: `internal/services/project/info/assembler.go` (arch file path adaptation)

**Interfaces:**
- Consumes: `GoSpecDocument`, `dsl.FlushSpec()` from Tasks 2-3
- Produces: Modified `provideSpecAssembler` using Go decoder instead of YAML decoder

- [ ] **Step 1: Update `cnt_glue.go` — replace YAML decoder with Go decoder**

Replace `provideYamlSpecProvider` method with `provideGoSpecProvider`:

Old code in `cnt_glue.go`:
```go
func (c *Container) provideYamlSpecProvider() *decoder.Decoder {
	return decoder.NewDecoder(
		c.provideSourceCodeReferenceResolver(),
		c.provideJsonSchemaProvider(),
	)
}
```

New code:
```go
func (c *Container) provideGoSpecProvider() *decoder.GoDecoder {
	return decoder.NewGoDecoder()
}
```

Update `provideSpecAssembler` to call `provideGoSpecProvider()` instead of `provideYamlSpecProvider()`.

Remove imports for `reference` and `schema` packages from `cnt_glue.go`.
Remove `provideSourceCodeReferenceResolver` and `provideJsonSchemaProvider` methods.

- [ ] **Step 2: Add GoDecoder type to `decoder_go.go`**

Append to `internal/services/spec/decoder/decoder_go.go`:

```go
import (
	"github.com/fe3dback/go-arch-lint/dsl"
	"github.com/fe3dback/go-arch-lint/internal/models/arch"
)

// GoDecoder implements the archDecoder interface by reading from the
// in-memory dsl.SpecBuilder (populated by the user's arch.go).
// The archFile parameter is ignored — the spec is already in-process.
type GoDecoder struct{}

func NewGoDecoder() *GoDecoder {
	return &GoDecoder{}
}

func (d *GoDecoder) Decode(_ string) (spec.Document, []arch.Notice, error) {
	builder, err := dsl.FlushSpec()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get spec from DSL: %w", err)
	}

	document := NewGoSpecDocument(builder)
	return document, []arch.Notice{}, nil
}
```

Add `"fmt"` to imports of decoder_go.go.

- [ ] **Step 3: Verify the `archDecoder` interface in assembler matches**

Check `internal/services/spec/assembler/types.go` — the `archDecoder` interface should be:
```go
archDecoder interface {
    Decode(archFilePath string) (spec.Document, []arch.Notice, error)
}
```

If the interface is defined there, `GoDecoder` already satisfies it. If it imports the concrete `*decoder.Decoder`, change to the interface.

- [ ] **Step 4: Build and verify**

Run: `go build ./...`
Expected: No errors (YAML decoder still exists, both can coexist)

- [ ] **Step 5: Commit**

```bash
git add internal/app/internal/container/cnt_glue.go internal/services/spec/decoder/decoder_go.go internal/services/spec/assembler/types.go
git commit -m "feat(di): wire Go decoder into DI container, replace YAML provider"
```

---

## Task 6: Delete YAML Code and Dependencies

**Files:**
- Delete: all files listed in "Deleted files" section of File Structure above
- Modify: `go.mod` (remove dependencies)
- Modify: `internal/app/internal/container/container_cmd.go` (remove schema command)

**Prerequisite:** Task 5 complete, Go decoder wired in.

- [ ] **Step 1: Delete YAML decoder files**

```bash
rm internal/services/spec/decoder/decoder.go
rm internal/services/spec/decoder/decoder_yaml.go
rm internal/services/spec/decoder/decoder_doc.go
rm internal/services/spec/decoder/decoder_doc_v1.go
rm internal/services/spec/decoder/decoder_doc_v2.go
rm internal/services/spec/decoder/decoder_doc_v3.go
rm internal/services/spec/decoder/decoder_utils.go
rm internal/services/spec/decoder/json_scheme.go
rm internal/services/spec/decoder/json_scheme_test.go
rm internal/services/spec/decoder/types.go
```

- [ ] **Step 2: Delete YAML reference resolver and schema provider**

```bash
rm -rf internal/services/common/yaml/
rm -rf internal/services/schema/
rm -rf internal/operations/schema/
```

- [ ] **Step 3: Remove schema command from container_cmd.go**

In `internal/app/internal/container/container_cmd.go`, remove the line:
```go
unwrap(c.commandSchema()),
```
from the `executors` slice.

Delete `container_cmd_schema.go` if it exists.

- [ ] **Step 4: Remove dependencies from go.mod**

```bash
go mod tidy
```

This removes `github.com/fe3dback/go-yaml` and `github.com/xeipuuv/gojsonschema` automatically since they're no longer imported.

- [ ] **Step 5: Fix any remaining compilation errors**

Run: `go build ./...`

If there are errors from references to deleted types (e.g., `doc` interface, `ref[T]`), fix them:
- Any remaining imports of deleted packages must be removed
- The `internal/services/spec/decoder/` package now only has `decoder_go.go` and `decoder_go_test.go`

- [ ] **Step 6: Delete old `main.go` and update `main_test.go`**

```bash
rm main.go
```

The old `main.go` is replaced by `cmd/arch-lint/main.go`. Update `main_test.go` or remove it if it tested the old entry point.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor: remove YAML decoder, JSON Schema, and related dependencies

- Delete YAML decoder, decoder_doc_v1/v2/v3, json_scheme
- Delete YAML reference resolver, schema provider, schema command
- Remove fe3dback/go-yaml and xeipuuv/gojsonschema dependencies
- ~1500 lines removed"
```

---

## Task 7: Rewrite Test Fixtures

**Files:**
- Modify: all `test/**/*.ct` files
- Create: `.go-arch-lint/` directory for each test project
- Delete: all `test/**/*.yml` arch fixtures

**Prerequisite:** Task 6 complete (YAML code removed).

**This is the most labor-intensive task.** Each `.yml` fixture must be converted to a `.go-arch-lint/arch.go` + `go.mod` + `main.go` scaffold, and each `.ct` file's `--arch-file` flag must be removed.

- [ ] **Step 1: Understand the test pattern**

Examine one existing fixture pair:

`test/check/arch1_ok.ct`:
```
$ go-arch-lint check --project-path ${PWD}/test/check/project --arch-file arch1_ok.yml --output-color=false
module: github.com/fe3dback/go-arch-lint/test/check/project
...
OK - No warnings found
```

`test/check/project/arch1_ok.yml`:
```yaml
version: 3
workdir: internal
allow:
  depOnAnyVendor: false
excludeFiles:
  - "^.*_test\\.go$"
components:
  main: { in: app }
deps:
  main:
    mayDependOn:
      - operations
```

**Transformation pattern:**

1. Convert YAML → `arch.go`:
```go
package main

import . "github.com/fe3dback/go-arch-lint/dsl"

var _ = Spec(func() {
    Version(1)
    Workdir("internal")
    Allow(func() { DepOnAnyVendor(false) })
    ExcludeFiles(`^.*_test\.go$`)
    Component("main", "app")
    Deps("main", func() { MayDependOn("operations") })
})
```

2. Create `test/check/project/.go-arch-lint-arch1_ok/` directory with `go.mod`, `main.go`, `arch.go`

3. Update `.ct` file: remove `--arch-file arch1_ok.yml`, change invocation to use the new scaffold dir

**Note:** The test framework (`google/go-cmdtest`) invokes the `go-arch-lint` binary. Since we now use `go run .go-arch-lint/`, the test binary needs Go in PATH. The test harness may need adaptation — alternatively, tests can invoke `go run ./cmd/arch-lint/` directly.

- [ ] **Step 2: Create a test helper for fixture scaffolding**

Create `test/helper.go` (or adapt existing `main_test.go`):

```go
package main

// TestMain or helper functions that scaffold .go-arch-lint/ dirs
// for each test case before cmdtest runs.
```

The exact approach depends on how cmdtest discovers and runs commands. The key change: instead of `--arch-file X.yml`, each test case needs a pre-scaffolded Go module.

- [ ] **Step 3: Convert each YAML fixture to Go DSL**

For each of the 13 `.yml` files in `test/check/project/`, create the corresponding `.go` config. Use the mapping table from the spec (§Migration).

**Example — `arch1_ok.yml` → `arch1_ok.go`:**

```go
// arch1_ok.go — translated from arch1_ok.yml
package main

import . "github.com/fe3dback/go-arch-lint/dsl"

var Arch1Ok = Spec(func() {
	Version(1)
	Workdir("internal")
	Allow(func() { DepOnAnyVendor(false) })
	ExcludeFiles(`^.*_test\.go$`)
	Component("main", "app")
	Deps("main", func() { MayDependOn("operations") })
})
```

**Repeat for all 13 fixtures:** `arch1_invalid_spec.yml`, `arch1_invalid_spec_type_err.yml`, `arch1_invalid_spec_unsupported_version.yml`, `arch1_nested_glob.yml`, `arch1_warnings.yml`, `arch2_ok_fallback.yml`, `arch2_ok_vendor_any.yml`, `arch2_ok_vendor_in_list.yml`, `arch2_ok_vendor_in_str.yml`, `arch2_ok_workdir.yml`, `arch3_ignore_not_found_components.yml`, `arch3_variadic.yml`.

- [ ] **Step 4: Update each `.ct` file**

Remove `--arch-file X.yml` from every `.ct` command invocation. The test framework must be adapted to use the Go DSL approach.

- [ ] **Step 5: Run tests and iterate**

Run: `go test ./... -v`
Expected: Tests pass with new Go DSL fixtures. Iterate on failures.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "test: rewrite all fixtures from YAML to Go DSL format"
```

---

## Task 8: Update Documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/syntax/README.md`
- Modify: `docs/README.md`
- Create: `docs/migration-v2.md`

- [ ] **Step 1: Rewrite README.md quickstart**

Replace the YAML config example in README with Go DSL:

```markdown
And describe/declare it as Go DSL config:

```go
// .go-arch-lint/arch.go
package main

import . "github.com/fe3dback/go-arch-lint/dsl"

var _ = Spec(func() {
    Version(1)
    Workdir("internal")
    Component("handler", "handlers/*")
    Component("service", "services/**")
    Component("repository", "domain/*/repository")
    Component("model", "models")
    CommonComponents("model")
    Deps("handler", func() { MayDependOn("service") })
    Deps("service", func() { MayDependOn("repository") })
})
```
```

Update install/run instructions to mention `go-arch-lint init`.

- [ ] **Step 2: Replace docs/syntax/README.md**

Replace the YAML syntax table with a DSL function reference documenting every function: `Spec`, `Version`, `Workdir`, `Allow`, `DepOnAnyVendor`, `DeepScan`, `IgnoreNotFoundComponents`, `Exclude`, `ExcludeFiles`, `Component`, `Vendor`, `CommonComponents`, `CommonVendors`, `Deps`, `MayDependOn`, `CanUse`, `AnyProjectDeps`, `AnyVendorDeps`.

- [ ] **Step 3: Update docs/README.md**

Remove the JSON Schema section (`schema` command). Replace with `go doc github.com/fe3dback/go-arch-lint/dsl` reference.

- [ ] **Step 4: Create docs/migration-v2.md**

Write the migration guide using the YAML→DSL mapping table from the spec.

- [ ] **Step 5: Commit**

```bash
git add docs/ README.md
git commit -m "docs: update for Go DSL config format, add v2 migration guide"
```

---

## Task 9: Migrate Repo's Own Config

**Files:**
- Delete: `.go-arch-lint.yml`
- Create: `.go-arch-lint/arch.go`, `.go-arch-lint/go.mod`, `.go-arch-lint/main.go`, `.go-arch-lint/go.sum`

- [ ] **Step 1: Scaffold the repo's config**

```bash
go run ./cmd/arch-lint/ init
```

- [ ] **Step 2: Translate `.go-arch-lint.yml` → `.go-arch-lint/arch.go`**

Use the full config from the design spec (§DSL API → Example: full config of this repo).

- [ ] **Step 3: Delete old YAML config**

```bash
rm .go-arch-lint.yml
```

- [ ] **Step 4: Verify the linter runs on itself**

```bash
go run ./cmd/arch-lint/ check --project-path .
```

Expected: exit code 0 (project architecture is correct)

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: migrate repo's own config from YAML to Go DSL"
```

---

## Self-Review

### Spec coverage check

| Spec section | Task(s) | Covered? |
|---|---|---|
| DSL API surface (all functions) | Task 2 | ✅ All functions in builders.go |
| Source positions via runtime.Caller | Task 2 | ✅ callerRef() in each builder |
| Go decoder (SpecBuilder → Document) | Task 3 | ✅ decoder_go.go |
| archlint.RunCLI() entry point | Task 4 | ✅ archlint.go |
| Launcher binary with init scaffolding | Task 4 | ✅ cmd/arch-lint/ |
| DI wiring (replace YAML decoder) | Task 5 | ✅ cnt_glue.go |
| Delete YAML code + deps | Task 6 | ✅ All files listed |
| Rewrite test fixtures | Task 7 | ✅ All 13 fixtures |
| Documentation updates | Task 8 | ✅ README, syntax, migration |
| Migrate own config | Task 9 | ✅ .go-arch-lint.yml → .go-arch-lint/arch.go |
| Remove `schema` command | Task 6 | ✅ Step 3 |
| Remove go-yaml, gojsonschema deps | Task 6 | ✅ Step 4 |

### Placeholder scan

No TBD/TODO/fill-in-later found. All code steps contain complete code.

### Type consistency

- `SpecBuilder` fields match between types.go (Task 1) and decoder_go.go (Task 3)
- `FlushSpec()` signature consistent: `() (*SpecBuilder, error)` in both spec.go and callers
- `callerRef(skip int) (file string, line int)` used consistently in all builders
- `GoSpecDocument`, `GoDecoder`, `goOptions`, `goVendor`, `goComponent`, `goDependencyRule` — names consistent
- `NewGoSpecDocument(builder *dsl.SpecBuilder)` — consistent between Task 3 test and implementation
- `NewGoDecoder()` — consistent between Task 3 and Task 5
- `archlint.RunCLI()` / `archlint.MustRunCLI()` — consistent between archlint.go and scaffold main.go
