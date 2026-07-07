# Migrating to v2.0 (Go DSL Config)

go-arch-lint v2.0 replaces the YAML config file (`.go-arch-lint.yml`) with a
pure Go DSL. The config is now a `.go-arch-lint/arch.go` file that imports the
`github.com/fe3dback/go-arch-lint/dsl` package. This gives you type checking,
IDE autocomplete, and the full power of Go (variables, loops, helper functions)
inside your config.

This is a hard cutover. The YAML reader is removed. You must migrate your
config to the Go DSL format.

## Migration steps

1. Install the v2.0 binary:
   ```bash
   go install github.com/fe3dback/go-arch-lint@latest
   ```

2. Scaffold the new config directory in your project root:
   ```bash
   cd ~/code/my-project
   go-arch-lint init
   ```
   This creates a `.go-arch-lint/` directory containing:
   - `go.mod` (pins the linter version)
   - `main.go` (generated, do not edit)
   - `arch.go` (you edit this)

3. Translate your `.go-arch-lint.yml` into `.go-arch-lint/arch.go` using the
   mapping table below.

4. Delete the old `.go-arch-lint.yml`.

5. Run `go-arch-lint check` to verify. The first run compiles your `arch.go`,
   which takes 1 to 3 seconds. Subsequent runs use the Go build cache and are
   much faster.

## YAML to DSL mapping

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

If you are on YAML schema V1 or V2, first upgrade to V3 using the existing
docs, then translate to the DSL.

## Worked example

### Before (YAML, `.go-arch-lint.yml`)

```yaml
version: 3
workdir: internal
allow:
  depOnAnyVendor: false
excludeFiles:
  - "^.*_test\\.go$"
components:
  main:       { in: app }
  services:   { in: services/** }
  models:     { in: models/** }
commonComponents:
  - models
vendors:
  cobra: { in: github.com/spf13/cobra }
deps:
  main:
    mayDependOn:
      - services
  services:
    mayDependOn:
      - services
    canUse:
      - cobra
```

### After (Go DSL, `.go-arch-lint/arch.go`)

```go
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

    Component("main", "app")
    Component("services", "services/**")
    Component("models", "models/**")

    CommonComponents("models")

    Deps("main", func() {
        MayDependOn("services")
    })

    Deps("services", func() {
        MayDependOn("services")
        CanUse("cobra")
    })
})
```

## What changed beyond the config

- The `schema` command is removed. JSON Schema export no longer exists. Use
  `go doc github.com/fe3dback/go-arch-lint/dsl` for API reference.
- The `--arch-file` flag is vestigial. The config always lives at
  `.go-arch-lint/arch.go` inside your project.
- The `.go-arch-lint/` directory is a separate Go module. If your project uses
  `go.work`, do not add `.go-arch-lint/` to the workspace. It is a tool module,
  not project code.

## What stays the same

- The `check`, `mapping`, `graph`, and `selfInspect` commands work the same way.
- Flags like `--project-path`, `--max-warnings`, `--json`, and `--output-type`
  are preserved.
- The linter logic (assembler, checker, renderer) is unchanged. Only the config
  format changed.
- Exit codes: `0` for clean, `1` for warnings.

## Troubleshooting

**"Spec() was not called":** Your `arch.go` must contain
`var _ = Spec(func() { ... })` at package level. The `var _ =` part ensures it
runs at init time.

**Go compiler errors in `arch.go`:** The DSL function signatures ARE the
schema. If the compiler complains, check the [syntax reference](syntax/README.md)
or run `go doc github.com/fe3dback/go-arch-lint/dsl` for the exact signatures.

**Slow first run:** The first `go-arch-lint check` compiles your config. This
takes 1 to 3 seconds. Subsequent runs are cached by `$GOCACHE` and drop to a
few hundred milliseconds. Use `go-arch-lint check --no-cache` to force a
rebuild.
