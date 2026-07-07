# Docs

- [syntax](syntax/README.md)
- [migration-v2](migration-v2.md)

## Advanced usages

### DSL API reference

The config schema is defined by the `github.com/fe3dback/go-arch-lint/dsl`
package. Since the config is now pure Go, the compiler and IDE autocomplete
replace the old JSON Schema layer.

To browse the full API with signatures and docs, run:

```bash
go doc github.com/fe3dback/go-arch-lint/dsl
```

Or view a specific function:

```bash
go doc github.com/fe3dback/go-arch-lint/dsl.Spec
go doc github.com/fe3dback/go-arch-lint/dsl.Component
go doc github.com/fe3dback/go-arch-lint/dsl.Deps
```

See [syntax/README.md](syntax/README.md) for a prose reference with examples.

### mapping

you can see archfile mapping to source files wia `mapping` command

two modes available:
- list (default)
- grouped by component

```bash
go-arch-lint mapping

module: github.com/fe3dback/go-arch-lint
Project Packages:
   app                 /internal/app
   container           /internal/app/internal/container
   commands            /internal/commands/check
   commands            /internal/commands/mapping
   ...
```

```bash
go-arch-lint mapping --scheme grouped

module: github.com/fe3dback/go-arch-lint
Project Packages:
   app:
     /internal/app
   commands:
     /internal/commands/check
     /internal/commands/mapping
   ...
```

same data available in json format, with `--json` option
