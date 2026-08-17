# Arch DSL Reference

The go-arch-lint configuration is a Go file (`.go-arch-lint/arch.go`) that imports the
`github.com/vsfedorenko/go-arch-lint/dsl` package. Every DSL function captures its position
in the source via `runtime.Caller`, so error messages point at the exact line in your `arch.go`.

> [Russian version](README.md)

## Entry point

### `func Spec(fn func()) SpecDef`

Registers a new spec builder, sets it as the current context, and executes `fn`.
DSL functions called inside `fn` populate the builder. Returns a `SpecDef` so the
`var _ = Spec(...)` pattern fires at package initialization time, before `main()`.

```go
var _ = Spec(func() {
    Version(1)
    Workdir("internal")
    Component("main", "app")
})
```

You can assign the result to a variable and pass it to `archlint.MustRun` / `archlint.Run`
instead of relying on the package-level global:

```go
var spec = Spec(func() {
    Version(1)
    // ...
})

func main() {
    archlint.MustRun(spec)
}
```

### `func MergeSpecs(specs ...SpecDef) SpecDef`

Merges multiple specs into one. The first non-empty value wins for scalar fields
(`Version`, `Workdir`, `Allow.*`); lists (`Exclude`, `ExcludeFiles`, `CommonComponents`,
`CommonVendors`) are concatenated; maps (`Components`, `Vendors`, `Deps`) are merged
key-by-key. This lets you split a large configuration across several files and combine
them in `main.go`.

```go
var spec = MergeSpecs(
    componentsSpec,
    depsSpec,
    vendorsSpec,
)
```

## Top-level attributes

### `func Version(v int)`

Sets the DSL schema version. For v2.0 always `1`.

```go
Version(1)
```

### `func Workdir(path string)`

Sets the relative working directory for analysis. The linter only checks Go packages
inside this directory.

```go
Workdir("internal")
```

## Global rules

### `func Allow(fn func())`

Opens a callback block for global allow rules. Call `DepOnAnyVendor`, `DeepScan`, and
`IgnoreNotFoundComponents` inside `fn`.

```go
Allow(func() {
    DepOnAnyVendor(false)
    DeepScan(true)
    IgnoreNotFoundComponents(false)
})
```

### `func DepOnAnyVendor(b bool)`

Determines whether any project code may import any vendor library. Defaults to `false`.
Call inside `Allow`.

### `func DeepScan(b bool)`

Enables or disables advanced AST analysis (tracking injections through constructors).
Defaults to `true`.

Inside `Allow`: sets the global default. Inside `Deps`: overrides the setting for an
individual component.

### `func IgnoreNotFoundComponents(b bool)`

When `true`, components whose glob does not match any package are silently skipped
instead of producing an error. Defaults to `false`. Call inside `Allow`.

## Exclusions

### `func Exclude(paths ...string)`

Adds directories (relative paths) to exclude from analysis.

```go
Exclude("vendor", "testdata")
```

### `func ExcludeFiles(patterns ...string)`

Adds regular-expression patterns for file names to exclude. Matching files and their
packages are skipped during analysis.

```go
ExcludeFiles(`^.*_test\.go$`, `^.*\/mock\/.*$`)
```

## Components

A component is an abstraction over one or more Go packages. A single component can map
to many packages via glob patterns.

### `func Component(name string, paths ...string)`

Defines a named component mapped to one or more relative package paths. Supports glob
masks (`src/*/engine/**`). The name must be non-empty.

```go
Component("handler", "handlers/*")
Component("services", "services/**", "lib/svc")
```

## Vendors

Vendors are external libraries from `go.mod`.

### `func Vendor(name string, importPaths ...string)`

Defines a named vendor mapped to one or more import paths. Supports glob masks
(`github.com/abc/*/engine/**`). The name must be non-empty.

```go
Vendor("cobra", "github.com/spf13/cobra")
Vendor("yaml", "github.com/goccy/go-yaml", "github.com/goccy/go-yaml/**")
```

## Common allow lists

### `func CommonComponents(names ...string)`

Marks components as importable by any project package, bypassing dependency rules.
Useful for shared models or utility packages.

```go
CommonComponents("models", "utils")
```

### `func CommonVendors(names ...string)`

Marks vendors as importable by any project package.

```go
CommonVendors("go-common")
```

## Dependency rules

### `func Deps(component string, fn func())`

Defines dependency rules for a component. Call `MayDependOn`, `CanUse`, `AnyProjectDeps`,
`AnyVendorDeps`, and `DeepScan` inside `fn`. The component name must match one defined
via `Component`.

```go
Deps("handler", func() {
    MayDependOn("service", "model")
    CanUse("cobra")
})
```

### `func MayDependOn(components ...string)`

Lists project components that this component may import. Must be called inside `Deps`.

```go
Deps("handler", func() {
    MayDependOn("service")
})
```

### `func CanUse(vendors ...string)`

Lists vendor names that this component may import. Must be called inside `Deps`.

```go
Deps("services", func() {
    CanUse("cobra", "yaml")
})
```

### `func AnyProjectDeps(b bool)`

When `true`, allows the component to import any other project package. Useful for DI
containers or entry points. Must be called inside `Deps`.

```go
Deps("container", func() {
    AnyProjectDeps(true)
})
```

### `func AnyVendorDeps(b bool)`

When `true`, allows the component to import any vendor package. Must be called inside `Deps`.

```go
Deps("container", func() {
    AnyVendorDeps(true)
})
```

## Layered architecture rules

Beyond permission-based rules (`Deps`/`MayDependOn`), the linter checks
structural conventions against the actual import graph. All are opt-in.

### `func Tiers(names ...string)` / `func Tier(name string, components ...string)`

Declare ordered layers; dependencies may only flow downward (index 0 is the
highest layer). Upward edges are violations, independent of `MayDependOn`
permissions.

```go
Tiers("domain", "app", "infra")
Tier("domain", "user", "order")
Tier("app", "handler")
Tier("infra", "postgres")
```

### `func Naming(fn func())` / `func ForbiddenPackages(names ...string)`

Ban junk-drawer package names. One violation per package, with the file
count and the first witness file.

```go
Naming(func() {
    ForbiddenPackages("utils", "helpers", "common", "misc", "stuff")
})
```

### `func Interfaces(fn func())` / `func MustLiveWithConsumer()`

Hexagonal-ports rule: an interface consumed by exactly one other component
must be declared in that component — not next to its implementation.
Shared interfaces (2+ consumers) are allowed to stay. Syntax-only analysis.

```go
Interfaces(func() {
    MustLiveWithConsumer()
})
```

### `func Visibility(fn func())` / `func VisibleTo(component string, allowed ...string)`

Restrict which components may consume a component's exported API, checked
against the actual import graph. The component itself is implicitly
allowed; multiple rules for one component accumulate.

```go
Visibility(func() {
    VisibleTo("services")                        // fully internal
    VisibleTo("models", "services", "container") // only these consumers
})
```

## Function summary

| Function | Scope | Description |
|---|---|---|
| `Spec(fn)` | top-level | Register and populate the spec builder |
| `MergeSpecs(specs...)` | top-level | Combine multiple specs into one |
| `Version(v)` | top-level | Schema version (always `1` for v2.0) |
| `Workdir(path)` | top-level | Relative working directory for analysis |
| `Allow(fn)` | top-level | Global rules block |
| `DepOnAnyVendor(b)` | inside `Allow` | Allow any project code to import any vendor |
| `DeepScan(b)` | inside `Allow` or `Deps` | Toggle advanced AST analysis (global default, or per-component override) |
| `IgnoreNotFoundComponents(b)` | inside `Allow` | Skip unmatched components silently |
| `Exclude(paths...)` | top-level | Exclude directories from analysis |
| `ExcludeFiles(patterns...)` | top-level | Exclude files matching regex patterns |
| `Component(name, paths...)` | top-level | Define a named component |
| `Vendor(name, importPaths...)` | top-level | Define a named vendor |
| `CommonComponents(names...)` | top-level | Components importable by anyone |
| `CommonVendors(names...)` | top-level | Vendors importable by anyone |
| `Deps(component, fn)` | top-level | Dependency rules block for a component |
| `MayDependOn(components...)` | inside `Deps` | Components this one may import |
| `CanUse(vendors...)` | inside `Deps` | Vendors this one may import |
| `AnyProjectDeps(b)` | inside `Deps` | Allow importing any project package |
| `AnyVendorDeps(b)` | inside `Deps` | Allow importing any vendor package |

## Full example

```go
// .go-arch-lint/arch.go
package main

import . "github.com/vsfedorenko/go-arch-lint/dsl"

var _ = Spec(func() {
    Version(1)
    Workdir("internal")

    Allow(func() {
        DepOnAnyVendor(false)
    })

    ExcludeFiles(`^.*_test\.go$`)

    Vendor("cobra", "github.com/spf13/cobra")
    Vendor("yaml", "github.com/goccy/go-yaml", "github.com/goccy/go-yaml/**")

    Component("main", "app")
    Component("services", "services/**")
    Component("models", "models/**")

    CommonComponents("models")
    CommonVendors("cobra")

    Deps("main", func() {
        MayDependOn("services")
    })

    Deps("services", func() {
        MayDependOn("services")
        CanUse("cobra", "yaml")
    })
})
```

For the YAML-to-DSL mapping table see [migration-v2.md](../migration-v2.md).
For ready-to-use architecture patterns see [the cookbook](../cookbook.md).

Examples:
- [Project's own `.go-arch-lint/main.go`](../../.go-arch-lint/main.go)
- [`examples/basic`](../../examples/basic) — layered (handler → service → repository)
- [`examples/ddd`](../../examples/ddd) — domain-driven design
- [`examples/hexagonal`](../../examples/hexagonal) — hexagonal / ports & adapters
