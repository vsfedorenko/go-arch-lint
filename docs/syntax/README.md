# Arch DSL Reference

The arch config is a Go file (`.go-arch-lint/arch.go`) that uses the
`github.com/fe3dback/go-arch-lint/dsl` package. Each DSL function captures its
source position via `runtime.Caller`, so error messages point back to the exact
line in your `arch.go`.

## Entry point

### `func Spec(fn func()) struct{}`

Registers a new spec builder, sets it as the current context, and executes `fn`.
DSL functions called inside `fn` populate the builder. Returns a zero-value
`struct{}` so the `var _ = Spec(...)` pattern runs at package init time, before
`main()`.

```go
var _ = Spec(func() {
    Version(1)
    Workdir("internal")
    Component("main", "app")
})
```

## Top-level attributes

### `func Version(v int)`

Sets the DSL schema version. Always `1` for v2.0.

```go
Version(1)
```

### `func Workdir(path string)`

Sets the relative working directory for analysis. The linter only checks Go
packages under this directory.

```go
Workdir("internal")
```

## Global rules

### `func Allow(fn func())`

Opens a callback block for global allow rules. Call `DepOnAnyVendor`,
`DeepScan`, and `IgnoreNotFoundComponents` inside `fn`.

```go
Allow(func() {
    DepOnAnyVendor(false)
    DeepScan(true)
    IgnoreNotFoundComponents(false)
})
```

### `func DepOnAnyVendor(b bool)`

Sets whether any project code may import any vendor library. Default `false`.
Call inside `Allow`.

### `func DeepScan(b bool)`

Enables or disables advanced AST analysis (constructor injection tracking).
Default `true`.

Inside `Allow`: sets the global default. Inside `Deps`: overrides the setting
for a single component.

### `func IgnoreNotFoundComponents(b bool)`

When `true`, components whose glob matches no packages are silently skipped
instead of producing an error. Default `false`. Call inside `Allow`.

## Exclusions

### `func Exclude(paths ...string)`

Adds directories (relative paths) to exclude from analysis.

```go
Exclude("vendor", "testdata")
```

### `func ExcludeFiles(patterns ...string)`

Adds regular expression patterns for filenames to exclude. Matching files and
their packages are skipped during analysis.

```go
ExcludeFiles(`^.*_test\.go$`, `^.*\/mock\/.*$`)
```

## Components

A component is an abstraction over one or more Go packages. One component can
map to many packages via glob patterns.

### `func Component(name string, paths ...string)`

Defines a named component mapping to one or more relative package paths. Supports
glob masking (`src/*/engine/**`).

```go
Component("handler", "handlers/*")
Component("services", "services/**", "lib/svc")
```

## Vendors

Vendors are external libraries from `go.mod`.

### `func Vendor(name string, importPaths ...string)`

Defines a named vendor mapping to one or more import paths. Supports glob
masking (`github.com/abc/*/engine/**`).

```go
Vendor("cobra", "github.com/spf13/cobra")
Vendor("yaml", "github.com/goccy/go-yaml", "github.com/goccy/go-yaml/**")
```

## Common access lists

### `func CommonComponents(names ...string)`

Marks components as importable by any project package, bypassing dependency
rules. Useful for shared models or utility packages.

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

Defines dependency rules for a component. Call `MayDependOn`, `CanUse`,
`AnyProjectDeps`, `AnyVendorDeps`, and `DeepScan` inside `fn`. The component
name must match one defined via `Component`.

```go
Deps("handler", func() {
    MayDependOn("service", "model")
    CanUse("cobra")
})
```

### `func MayDependOn(components ...string)`

Lists project components that this component may import. Must be called inside
`Deps`.

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

When `true`, allows this component to import any other project package. Useful
for DI containers or main entry points. Must be called inside `Deps`.

```go
Deps("container", func() {
    AnyProjectDeps(true)
})
```

### `func AnyVendorDeps(b bool)`

When `true`, allows this component to import any vendor package. Must be called
inside `Deps`.

```go
Deps("container", func() {
    AnyVendorDeps(true)
})
```

## Full example

```go
// .go-arch-lint/arch.go
package main

import . "github.com/fe3dback/go-arch-lint/dsl"

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

For the YAML to DSL migration table, see [migration-v2.md](../migration-v2.md).

Examples:
- [.go-arch-lint/arch.go](../../.go-arch-lint/arch.go)
