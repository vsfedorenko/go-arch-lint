# Design: Migrate config format from YAML to Pure Go DSL

**Date:** 2026-07-07
**Status:** Approved (brainstorming complete)
**Major version impact:** v1.x → v2.0 (breaking change, hard cutover)

## TL;DR

Replace the YAML-based config (`.go-arch-lint.yml`) with a Pure Go DSL. Users
write a `.go-arch-lint/arch.go` file using the `github.com/fe3dback/go-arch-lint/dsl`
package. A generated `main.go` imports the linter as a library and runs `check`
in-process, giving native Go types end-to-end with zero serialization. YAML
support is removed entirely (major version bump to 2.0).

## Motivation

The current YAML config has four pain points, all of which this design
addresses:

1. **Syntax/ergonomics** — YAML's inline maps (`{ in: x }`), implicit type
   coercion, and indentation sensitivity are error-prone. A Go DSL gives
   structured, type-checked syntax with IDE autocomplete.
2. **No variables/expressions** — YAML is static data. The Go DSL enables DRY:
   variables, loops, helper functions, conditional rules.
3. **Validation/tooling** — Current validation relies on a custom YAML fork
   (`github.com/fe3dback/go-yaml`) plus a JSON Schema layer for error messages.
   Go's compiler and the `dsl` package signatures ARE the schema.
4. **Linter code complexity** — The `ref[T]` generic wrapper, custom YAML fork,
   JSON Schema generation, and three schema versions (V1/V2/V3) add ~1500 lines
   of machinery. The Go DSL eliminates all of it.

## Decisions (from brainstorming)

| Decision | Choice | Alternatives considered |
|---|---|---|
| **Config format** | Pure Go DSL | CUE, Starlark, HCL, improved YAML |
| **Execution model** | `go run` codegen | yaegi interpreter, `plugin.Open` |
| **Process boundary** | Generated `main.go` imports linter library; runs in-process; zero serialization | JSON contract, gob contract, `plugin.Open` native |
| **Backward compatibility** | Hard cutover, major bump to v2.0 | Dual-read, migration+deprecation, legacy freeze |

### Why Pure Go DSL over CUE/Starlark/HCL

The target audience is Go developers who already have the Go toolchain
installed (they lint Go code). A Go DSL gives:
- Zero new languages to learn
- Full IDE support (autocomplete, jump-to-definition, refactoring) via the
  Go compiler
- Unlimited DRY power (loops, functions, generation)
- Type safety enforced at compile time

CUE was the strongest declarative alternative (types=schema, native source
positions, `cue export` for back-compat), but config-as-code won on the
"zero new languages for users" axis.

### Why `go run` codegen over yaegi / plugin.Open

- **yaegi** — fast startup but limited (generics, reflect, CGO). Would silently
  reject valid Go patterns users expect to work.
- **plugin.Open** — native interface but Linux/macOS only, requires exact Go
  version match between plugin and host binary, needs CGO. Not viable for a
  cross-platform public tool.
- **`go run` codegen** — full Go power, native compiler errors (best IDE UX),
  proven model (goa.design). Cold start ~1-3s (cached after via `$GOCACHE`).
  Target users have Go installed.

### Why generated `main.go` + linter-as-library over JSON/gob contract

The first design iteration proposed `go run` of a `dump` command that emits
JSON (later refined to `gob`) which the linter binary parses. User feedback
identified a cleaner approach: generate a `main.go` inside `.go-arch-lint/`
that imports the linter as a library and runs `check` in-process.

This eliminates:
- The serialization contract (JSON/gob) and its schema maintenance
- The `dump` command as a separate entry point
- The position-encoding-as-data workaround

Source positions are captured natively via `runtime.Caller(1)` inside DSL
functions and stored as `common.Referable[T]`, directly replacing the `ref[T]`
generic wrapper.

### Why hard cutover

The project is public OSS (Docker images, contributors, issue tracker), so
hard cutover has real user cost. It was chosen because:
- Keeping YAML reader defeats the simplification goal
- Dual-read doubles the schema/validation maintenance
- A clean v2.0 with thorough migration docs is more honest than a long
  deprecation tail
- Migration is mechanical (documented mapping table, see §Migration)

## Architecture

### Data flow

```
User: go-arch-lint check --project-path ~/code/proj
                    │
                    ▼
go-arch-lint binary (thin launcher):
  1. locate .go-arch-lint/ in project-path
  2. exec: go run .go-arch-lint/ check --project-path ...
  3. pipe stdout/stderr, propagate exit code
                    │
                    ▼  (go run: compile + run, cached by $GOCACHE)
.go-arch-lint/  (isolated module, scaffolded by `init`):

  go.mod  → require github.com/fe3dback/go-arch-lint v2.0.0

  main.go (GENERATED, do-not-edit):
    package main
    import "github.com/fe3dback/go-arch-lint"
    func main() { archlint.RunCLI() }

  arch.go (USER edits):
    package main
    import . "github.com/fe3dback/go-arch-lint/dsl"
    var _ = Spec(func() {
        Version(1)
        Workdir("internal")
        Component("main", "app")
        Deps("container", func() { MayDependOn("ops") })
    })

  → main.go + arch.go compile together into one binary
                    │
                    ▼  (IN-PROCESS, native Go heap)
archlint.RunCLI()  (linter library, runs IN-PROCESS):
  1. dsl.FlushSpec() → *dsl.SpecBuilder
     (native common.Referable[T] with positions from runtime.Caller)
  2. SpecBuilder → spec.Document (in-process mapping)
  3. assembler → validators → checker → renderer (UNCHANGED)
  4. print result → stdout, os.Exit(0 or 1)
```

### Key invariant

The subprocess boundary (`go run`) exists only to compile user's `arch.go`.
Inside the subprocess, all linter code runs in-process with native Go types.
There is no serialization contract between user config and linter internals.

### Caching

- `$GOCACHE` caches compilation of `.go-arch-lint/`
- Cold start: ~1-3s (first compile)
- Warm: ~200-500ms (Go build cache hit)
- `--no-cache` flag forces recompilation (passes `go run -a`)

## DSL API

Package: `github.com/fe3dback/go-arch-lint/dsl`

Callback-builder pattern (goa-style). Each DSL function captures source
position via `runtime.Caller(1)`.

### Full API surface

```go
package dsl

// Entry point. Registers spec-builder, executes fn.
func Spec(fn func())

// Top-level attributes
func Version(v int)        // DSL schema version, always 1 for v2.0
func Workdir(path string)

// Global rules (callback for grouping)
func Allow(fn func())
  func DepOnAnyVendor(b bool)
  func DeepScan(b bool)
  func IgnoreNotFoundComponents(b bool)

// Exclusions
func Exclude(paths ...string)         // directories
func ExcludeFiles(patterns ...string) // regex on filenames

// Components — positional (only `in` field)
func Component(name string, paths ...string)

// Vendors — positional (only `in` field)
func Vendor(name string, importPaths ...string)

// Common access lists
func CommonComponents(names ...string)
func CommonVendors(names ...string)

// Deps — callback (multiple optional fields)
func Deps(component string, fn func())
  func MayDependOn(components ...string)
  func CanUse(vendors ...string)
  func AnyProjectDeps(b bool)
  func AnyVendorDeps(b bool)
  func DeepScan(b bool)  // per-component override of allow.deepScan
```

### Source position tracking

```go
// inside dsl package
var currentBuilder *specBuilder // stack-based, like goa design stack

func Component(name string, paths ...string) {
    _, file, line, _ := runtime.Caller(1)
    currentBuilder.addComponent(name, paths, file, line)
}

func Deps(component string, fn func()) {
    _, file, line, _ := runtime.Caller(1)
    prev := currentBuilder
    depBuilder := newDepBuilder(component, file, line)
    currentBuilder = depBuilder
    fn()  // inner funcs (MayDependOn, CanUse, ...) each capture their own pos
    currentBuilder = prev
    prev.addDep(depBuilder)
}
```

`runtime.Caller(1)` returns the file:line of the call site in the user's
`arch.go`, not inside the dsl package internals. This natively replaces the
`ref[T]` generic wrapper and the custom YAML fork's AST node access.

### Example: full config of this repo

```go
// .go-arch-lint/arch.go
package arch

import . "github.com/fe3dback/go-arch-lint/dsl"

var _ = Spec(func() {
    Version(1)
    Workdir("internal")

    Allow(func() {
        DepOnAnyVendor(false)
    })

    ExcludeFiles(
        `^.*_test\.go$`,
        `^.*\/test\/.*$`,
    )

    // DRY via ordinary Go variables
    goAstImports := []string{
        "golang.org/x/mod/modfile",
        "golang.org/x/tools/go/packages",
    }

    Vendor("go-common", "golang.org/x/sync/errgroup")
    Vendor("go-ast", goAstImports...)
    Vendor("3rd-cobra", "github.com/spf13/cobra")
    Vendor("3rd-color-fmt", "github.com/logrusorgru/aurora/v3")
    Vendor("3rd-code-highlight", "github.com/alecthomas/chroma/*")
    Vendor("3rd-json-scheme", "github.com/xeipuuv/gojsonschema")
    Vendor("3rd-graph", "oss.terrastruct.com/d2/**")
    Vendor("3rd-yaml",
        "github.com/goccy/go-yaml",
        "github.com/goccy/go-yaml/**",
    )

    Component("main", "app")
    Component("container", "app/internal/container/**")
    Component("operations", "operations/*")
    Component("services", "services/**")
    Component("view", "view")
    Component("models", "models/**")

    CommonVendors("go-common")
    CommonComponents("models")

    Deps("main", func() {
        MayDependOn("container")
    })

    Deps("container", func() {
        AnyVendorDeps(true)
        MayDependOn("operations", "services", "view")
    })

    Deps("operations", func() {
        MayDependOn("services")
        CanUse("3rd-graph")
    })

    Deps("services", func() {
        MayDependOn("services")
        CanUse("go-ast", "3rd-yaml", "3rd-color-fmt", "3rd-code-highlight", "3rd-json-scheme")
    })
})
```

## Package layout

```
github.com/fe3dback/go-arch-lint/
├── archlint.go              ← NEW: package archlint, RunCLI() entry point
├── dsl/                     ← NEW: package dsl
│   ├── spec.go              ← SpecBuilder type + Spec(fn) entry
│   ├── builders.go          ← Component, Vendor, Deps, Allow, ...
│   │                          (each with runtime.Caller)
│   ├── context.go           ← builder stack (goa design stack pattern)
│   └── types.go             ← shared SpecBuilder / DepEntry / ComponentEntry
├── internal/                ← existing, mostly unchanged
│   ├── app/cli.go           ← invoked from archlint.RunCLI()
│   ├── operations/check/    ← UNCHANGED
│   ├── operations/mapping/  ← UNCHANGED
│   ├── operations/graph/    ← UNCHANGED
│   ├── operations/selfInspect/ ← UNCHANGED
│   ├── operations/version/  ← UNCHANGED
│   ├── operations/schema/   ← DELETED (no JSON Schema anymore)
│   ├── services/spec/
│   │   ├── decoder/
│   │   │   ├── decoder.go        ← DELETED (YAML-specific)
│   │   │   ├── decoder_yaml.go   ← DELETED
│   │   │   ├── decoder_doc.go    ← DELETED
│   │   │   ├── decoder_doc_v1.go ← DELETED
│   │   │   ├── decoder_doc_v2.go ← DELETED
│   │   │   ├── decoder_doc_v3.go ← DELETED (logic moves to dsl types)
│   │   │   ├── decoder_utils.go  ← DELETED (ref[T] utils)
│   │   │   ├── json_scheme.go    ← DELETED
│   │   │   ├── json_scheme_test.go ← DELETED
│   │   │   ├── types.go          ← UPDATED (interfaces for new decoder)
│   │   │   └── decoder_go.go     ← NEW: SpecBuilder → spec.Document
│   │   ├── assembler/        ← UNCHANGED
│   │   ├── validator/        ← UNCHANGED (validators operate on Document)
│   │   └── document.go       ← UPDATED (doc interface adapts to SpecBuilder)
│   ├── services/schema/      ← DELETED (JSON Schema provider)
│   ├── services/common/yaml/ ← DELETED (YAML path → source ref resolver)
│   └── ...
├── cmd/
│   └── arch-lint/            ← NEW: thin launcher binary
│       └── main.go           ← locate .go-arch-lint/, exec go run, handle init/version
├── main.go                   ← SPLIT: launcher → cmd/arch-lint/, CLI → archlint.go
└── main_test.go              ← UPDATED for new architecture
```

### Rules

- `archlint` (root public package) — sole public entry point; imports `internal/`
- `dsl` (public package) — imported by user's `arch.go` AND by `archlint`
- `internal/` — stays internal; only the spec decoder layer is refactored
- `cmd/arch-lint/` — binary entry; thin launcher + init scaffolding logic

## `.go-arch-lint/` scaffolding (generated by `init`)

```
.go-arch-lint/
├── go.mod          ← module arch-lint-local
│                   ← require github.com/fe3dback/go-arch-lint v2.0.0
├── main.go         ← GENERATED (do-not-edit):
│   // Code generated by go-arch-lint init. DO NOT EDIT.
│   package main
│   import "github.com/fe3dback/go-arch-lint"
│   func main() { archlint.RunCLI() }
├── arch.go         ← USER edits (example spec at init time)
│   package main
│   import . "github.com/fe3dback/go-arch-lint/dsl"
│   var _ = Spec(func() {
│       Version(1)
│       Workdir("internal")
│       Component("main", "app")
│       // ... TODO: describe your architecture
│   })
└── go.sum
```

## Command surface (new UX)

| Command | Behavior |
|---|---|
| `go-arch-lint init [--project-path .]` | Binary scaffolds `.go-arch-lint/` (go.mod, main.go, arch.go, go.sum) |
| `go-arch-lint version` | Binary prints version |
| `go-arch-lint check [flags]` | Binary → `go run .go-arch-lint/ check [flags]`, pipes I/O |
| `go-arch-lint mapping [flags]` | Binary → `go run .go-arch-lint/ mapping [flags]` |
| `go-arch-lint graph [flags]` | Binary → `go run .go-arch-lint/ graph [flags]` |
| `go-arch-lint selfInspect` | Binary → `go run .go-arch-lint/ selfInspect` |
| `go-arch-lint schema` | DELETED (JSON Schema no longer exists; replacement: `go doc github.com/fe3dback/go-arch-lint/dsl`) |

`archlint.RunCLI()` inside the `go run` process parses the same cobra commands
previously wired in `main.go`. Flags (`--project-path`, `--max-warnings`,
`--json`, `--output-type`, etc.) are preserved.

## Codebase delta

### Deleted (~1500 lines, 2 dependencies)

| Path | Reason |
|---|---|
| `internal/services/spec/decoder/decoder.go` | YAML-specific decode |
| `internal/services/spec/decoder/decoder_yaml.go` | ref[T] + YAML AST hooks |
| `internal/services/spec/decoder/decoder_doc.go` | doc interface, replaced |
| `internal/services/spec/decoder/decoder_doc_v1.go` | V1 schema |
| `internal/services/spec/decoder/decoder_doc_v2.go` | V2 schema |
| `internal/services/spec/decoder/decoder_doc_v3.go` | V3 schema (logic moves to dsl) |
| `internal/services/spec/decoder/decoder_utils.go` | ref[T] utilities |
| `internal/services/spec/decoder/json_scheme.go` | JSON Schema validation |
| `internal/services/spec/decoder/json_scheme_test.go` | test for above |
| `internal/services/common/yaml/reference/` | YAML path → source ref resolver |
| `internal/services/schema/provider.go` | JSON Schema provider |
| `internal/operations/schema/` | `schema` command |
| `github.com/fe3dback/go-yaml` (go.mod) | custom YAML fork, no longer needed |
| `github.com/xeipuuv/gojsonschema` (go.mod) | JSON Schema validator, no longer needed |

### Updated

| Path | Change |
|---|---|
| `internal/services/spec/document.go` | doc interface adapts to SpecBuilder source |
| `internal/services/spec/decoder/types.go` | interfaces for new Go decoder |
| `main.go` | split: launcher → `cmd/arch-lint/main.go`, CLI → `archlint.go` |
| `main_test.go` | updated for new architecture |
| `go.mod` | remove 2 deps; module path unchanged |

### Added (~730 lines)

| Path | Purpose | Est. LoC |
|---|---|---|
| `archlint.go` | RunCLI() + cobra wiring | ~100 |
| `dsl/spec.go` | SpecBuilder type + Spec(fn) entry | ~80 |
| `dsl/builders.go` | Component, Vendor, Deps, Allow, ... with runtime.Caller | ~180 |
| `dsl/context.go` | builder stack | ~60 |
| `dsl/types.go` | shared types | ~80 |
| `internal/services/spec/decoder/decoder_go.go` | SpecBuilder → spec.Document | ~150 |
| `cmd/arch-lint/main.go` | thin launcher + init scaffolding | ~80 |

**Net:** −~1500 / +~730 = ~770 lines removed; 2 dependencies dropped; decoder
collapses from 3 schema versions to 1.

## Migration (for users)

Hard cutover means YAML reader is removed. Users migrate by:

1. `go-arch-lint init` in project root → generates `.go-arch-lint/` scaffold
2. Translate `.go-arch-lint.yml` → `.go-arch-lint/arch.go` using the mapping
   table below
3. Delete old `.go-arch-lint.yml`
4. `go-arch-lint check` works as before

### YAML → DSL mapping table

| YAML | Go DSL |
|---|---|
| `version: 3` | `Version(1)` (DSL schema version, always 1 for v2.0) |
| `workdir: internal` | `Workdir("internal")` |
| `allow: { depOnAnyVendor: false }` | `Allow(func() { DepOnAnyVendor(false) })` |
| `allow: { deepScan: true }` | `DeepScan(true)` inside `Allow` callback |
| `allow: { ignoreNotFoundComponents: true }` | `IgnoreNotFoundComponents(true)` inside `Allow` callback |
| `exclude: [a, b]` | `Exclude("a", "b")` |
| `excludeFiles: [regex]` | `ExcludeFiles("regex")` |
| `vendors: { name: { in: x } }` | `Vendor("name", "x")` |
| `vendors: { name: { in: [a,b] } }` | `Vendor("name", "a", "b")` |
| `components: { name: { in: x } }` | `Component("name", "x")` |
| `commonComponents: [a, b]` | `CommonComponents("a", "b")` |
| `commonVendors: [a, b]` | `CommonVendors("a", "b")` |
| `deps: { name: { mayDependOn: [...] } }` | `Deps("name", func() { MayDependOn("...") })` |
| `deps.name.canUse: [...]` | `CanUse("...")` inside `Deps` callback |
| `deps.name.anyVendorDeps: true` | `AnyVendorDeps(true)` inside `Deps` callback |
| `deps.name.anyProjectDeps: true` | `AnyProjectDeps(true)` inside `Deps` callback |
| `deps.name.deepScan: true` | `DeepScan(true)` inside `Deps` callback (overrides global) |

V1/V2 YAML configs should first upgrade to V3 (per existing docs), then
translate to DSL.

### Documentation updates

- `README.md` — quickstart rewritten with Go DSL example
- `docs/syntax/README.md` — replaced with DSL function reference
- `docs/migration-v2.md` — NEW: migration guide with the table above
- `.go-arch-lint.yml` (repo's own config) → `.go-arch-lint/arch.go`

## Testing

| Layer | Strategy |
|---|---|
| `dsl` package unit tests | Each builder: `Component("x","y")` → SpecBuilder contains correct entry + position. Position via test helper file. |
| `decoder_go.go` | SpecBuilder → spec.Document mapping; edge cases (empty config, missing required fields, invalid globs) |
| cmdtest (`.cmd` fixtures) | Rewrite ALL existing fixtures: each test gets `.go-arch-lint/arch.go` instead of `.go-arch-lint.yml`. Existing `google/go-cmdtest` works unchanged — it tests CLI, not config format. |
| Integration tests | `init` → scaffold, then `check` on a test project, verify exit code + output |
| Regression: assembler/checker/renderer | Untouched — these layers are isolated from config format |

**cmdtest impact:** Existing fixtures in `test/` use `.go-arch-lint.yml` and
must be rewritten as `.go-arch-lint/arch.go`. Volume: ~20-30 fixtures. This is
the most labor-intensive part of the migration.

## Open questions (to resolve in implementation plan)

1. **Version drift awareness**: The launcher binary is thin — `go run` uses
   whatever `dsl` version is pinned in `.go-arch-lint/go.mod`. So launcher v2.0
   can safely run a project pinned to dsl v2.1. The real risk is API drift: if
   the dsl package has a breaking change and user's `arch.go` was written for
   an older API, `go run` produces a Go compiler error (good UX, but confusing
   without context). Mitigation: `go-arch-lint check` wraps `go run` stderr to
   add context ("your arch.go failed to compile against dsl vX.Y.Z — check
   migration docs"); `init` pins the exact released dsl version matching the
   launcher.

2. **Caching invalidation**: When user edits `arch.go`, Go build cache handles
   recompilation automatically. When linter library is updated (user bumps
   `go.mod`), cache invalidates correctly. Document `--no-cache` for edge cases.

3. **Multi-module workspaces (go.work)**: `.go-arch-lint/` is a separate
   module. If the host project uses `go.work`, `.go-arch-lint/` should not be
   added to the workspace (it's a tool module, not project code). Document this.

4. **CI environments**: `go run` requires Go toolchain in CI. For Docker-based
   CI, the official `fe3dback/go-arch-lint` Docker image must include Go, OR
   the image runs `go run` internally. Decide: bundle Go in Docker image, or
   require users to provide Go in their CI runner.

5. **Error message quality**: When user's `arch.go` has a compile error,
   `go run` surfaces the Go compiler error. This is good UX (precise, with
   file:line), but the linter should wrap it to explain context ("failed to
   compile your arch config"). Decide on wrapping format.

## Non-goals

- **Dual-read support** — explicitly rejected; YAML removed entirely.
- **Migration automation** (`go-arch-lint migrate` command) — rejected in favor
  of documented manual migration table. Can be added later if users struggle.
- **CUE/Starlark/HCL support** — out of scope; Go DSL is the only format.
- **Refactoring assembler/checker/renderer** — these layers are untouched.
- **JSON Schema export** — `schema` command removed; `go doc` replaces it.
