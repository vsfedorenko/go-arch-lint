[Русский](README.md) | [**English**](README.en.md)

---

![Logo image](docs/images/logo.png)

**Go architecture linter** — `go-arch-lint` enforces **clean architecture**,
**hexagonal**, **onion**, and **DDD** dependency rules in Go projects. Describe
your layers and dependency rules in a type-safe Go DSL; the linter finds import
and dependency-injection violations automatically.

[![CI](https://github.com/vsfedorenko/go-arch-lint/actions/workflows/ci.yml/badge.svg)](https://github.com/vsfedorenko/go-arch-lint/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/vsfedorenko/go-arch-lint)](https://goreportcard.com/report/github.com/vsfedorenko/go-arch-lint)
[![Go Reference](https://pkg.go.dev/badge/github.com/vsfedorenko/go-arch-lint.svg)](https://pkg.go.dev/github.com/vsfedorenko/go-arch-lint)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.25-blue)](https://golang.org/dl/)
[![License: MIT](https://img.shields.io/github/license/vsfedorenko/go-arch-lint)](LICENSE)
[![Release](https://img.shields.io/github/v/release/vsfedorenko/go-arch-lint)](https://github.com/vsfedorenko/go-arch-lint/releases)
[![go-recipes](https://raw.githubusercontent.com/nikolaydubina/go-recipes/main/badge.svg?raw=true)](https://github.com/nikolaydubina/go-recipes)

## Install

```bash
go install github.com/vsfedorenko/go-arch-lint/v3/cmd/arch-lint@latest
```

Or use [Docker](https://github.com/vsfedorenko/go-arch-lint/pkgs/container/go-arch-lint):

```bash
docker run --rm -v ${PWD}:/app ghcr.io/vsfedorenko/go-arch-lint:latest check --project-path /app
```

Or grab a [binary from releases](https://github.com/vsfedorenko/go-arch-lint/releases).

> The module path is `github.com/vsfedorenko/go-arch-lint/v3` (the suffix is
> mandatory for Go major versions). Upgrading from v2.x: update the
> imports in `.go-arch-lint/` to `/v3` and migrate the spec (see “Migrating from v2.x” below), or start fresh: `rm -rf .go-arch-lint && go-arch-lint init`.

## Examples: hexagonal and clean architectures

A spec is the dependency tree written dependencies-first. Two canonical
templates:

### Hexagonal (ports & adapters)

```
cmd/
└── app/               # the entry point: wires everything
internal/
├── domain/            # the heart: entities, zero external dependencies
├── core/              # application logic: uses domain only
└── adapter/           # the periphery: delivers requests into core
    ├── http/
    └── db/
```

```go
var build = Spec(func() {
	domain := Path("internal/domain")

	core := Path("internal/core", func() { Use(domain) })

	http := Path("internal/adapter/http", func() { Use(core) })
	db := Path("internal/adapter/db", func() { Use(core, domain) })

	Path("cmd", func() {
		Path("app", func() { Use(core, db, http) }) // the entry point wires everything
	})
})
```

Arrows point inward: adapters know about core, core only about domain.
Importing `domain` from `adapter/http` fails the check — its `Use` rule
does not allow it.

### Clean architecture

```
internal/
├── entity/        # enterprise rules: entities
├── usecase/       # application scenarios
├── adapter/       # interfaces to the outside: repo, web
│   ├── repo/
│   └── web/
└── framework/     # drivers: DB, http server
```

```go
var build = Spec(func() {
	entity := Path("internal/entity")

	usecase := Path("internal/usecase", func() { Use(entity) })

	repo := Path("internal/adapter/repo", func() { Use(usecase, entity) })
	web := Path("internal/adapter/web", func() { Use(usecase, entity) })

	pg := Vendor("pgx", "github.com/jackc/pgx/v5")
	chi := Vendor("chi", "github.com/go-chi/chi/v5")

	Path("internal/framework", func() { Use(repo, web, usecase, entity, pgx, chi) })

	Path("cmd", func() {
		Path("app", func() { Use(usecase, entity, repo, web, pgx, chi) })
	})
})
```

Vendors (`pgx`, `chi`) live in the single outermost layer — framework and
the entry point; inner layers know nothing about them. Importing `pgx`
from `usecase` is a file:line violation.

All three are real projects in [examples/](examples/) (basic — layers, ddd —
bounded contexts, hexagonal — ports and adapters), and CI checks every one of
them: the `examples` job runs `go-arch-lint check` on all three per PR.

## Requirements

- **Go 1.25+** (the two latest major versions are supported: 1.25 and 1.26; CI tests both).
- `go` must be on your `PATH` — the CLI compiles `.go-arch-lint/arch.go` via `go run` (cached, steady-state runs take ~2s).
- **Go workspaces (`go.work`).** A project with a root `go.mod` works fully — including sibling workspace member modules: `init` excludes them from the architecture by default, and you can declare them as components (`Path("two/y")`) to write cross-module `Use` rules. A member module may carry any module path of its own (`example.com/y`, not necessarily `example.com/root/two/y`) — member imports classify as project code, not vendor. A workspace of sibling modules with no root `go.mod` is not supported yet — the CLI explains this when you run it.

## Configuration

The config is a Go file. Not YAML, not JSON — plain Go code with type safety and IDE autocomplete.

```bash
cd ~/code/my-project
go-arch-lint init
```

Creates `.go-arch-lint/` with three files. The spec and the runner are separate: `arch.go` is yours to edit, `main.go` is generated plumbing you never touch:

```go
// arch.go — this file describes your architecture
package main

import (
	. "github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

var build = Spec(func() {
	// Every directory with Go code is declared, and every import
	// that exists today is allowed: the scaffold mirrors the code
	// as-is. Tighten the Use rules as your architecture takes shape:
	//
	//     domain := Path("internal/domain")
	//     Path("internal/core", func() { Use(domain) })
	domain := Path("internal/domain")
	handlers := Path("internal/handlers", func() { Use(domain) })
	Path(".", func() { Use(domain, handlers) })
	Path("cmd/app", func() { Use(domain, handlers) })
})
```

```go
// main.go — generated, do not edit: forwards CLI flags into the check
package main

import (
	"os"

	"github.com/vsfedorenko/go-arch-lint/v3"
)

func main() {
	archlint.MustRunCLI(build, os.Args[1:])
}
```

How this works:

— `init` scans the project and declares every directory with Go code as a component (`Path`), every internal import becomes a `Use` rule, and every third-party library becomes a `Vendor` declaration + `Use` — the fresh scaffold mirrors the code as-is and checks green on day one. Tightening a rule is then deleting a `Use` line and watching the violations surface.
— `Use` is the only rule: "this path uses these targets". Without an explicit `Use`, everything is denied.
— `Vendor(name, import)` — an external library as a legal target; the standard library is always allowed.
— Declaration order mirrors dependency direction: referring forward is a Go compile error.

Full DSL function reference: [syntax docs](docs/syntax/README.md) or `go doc github.com/vsfedorenko/go-arch-lint/v3/dsl`.

### Architecture templates

Skip the blank page — start from [`examples/`](examples/): copy the `.go-arch-lint/` from [basic](examples/basic/) (layers), [ddd](examples/ddd/) (bounded contexts) or [hexagonal](examples/hexagonal/) (ports & adapters) into your project and adjust the paths. Every example is a complete project with its spec on the current DSL.

The `--recipe` flag existed in v1/v2 and was removed in v3: `init` no longer accepts it — or any other unknown flag; silently falling back to the default spec would be worse than an error.

## Check

```bash
go-arch-lint check
```

The linter builds an import graph from the actual code, compares it to the configured dependency graph, and reports violations:

![Check output](docs/images/check-example.png)

| Exit code | Meaning                                                     |
|-----------|-------------------------------------------------------------|
| 0         | No violations                                               |
| 1         | Violations found                                            |
| 2         | Broken config: arch.go does not compile, or the project is unreadable |

Exit code 2 means "the check did not run" — fix the config, not the code.

Use `--json` for machine-readable output in CI pipelines, `--format sarif`
for a SARIF 2.1.0 log that GitHub Code Scanning and other code-scanning
tools ingest natively, `--format junit` for a JUnit-style XML report
that CI test dashboards (GitLab CI, Jenkins, Buildkite) render natively,
or `--format html` for a standalone HTML report humans can open directly
(see [docs/json-schema.md](docs/json-schema.md#sarif-output-for-github-code-scanning)).

### Suppressing known violations (baseline)

For incremental adoption on legacy codebases, annotate a known violation
in source with a directive — the check passes, while new violations
still fail the build:

```go
//go-arch-lint:ignore            // suppress any violation on the line
//go-arch-lint:ignore beta       // only when the dependency target is beta
import _ "example.com/app/internal/beta"

//go-arch-lint:ignore-file       // suppress every violation in the file
package legacy
```

Place the directive on the offending line itself or the line directly
above it. The argument (space-separated, multiple allowed) filters by
target: component name (`beta`) or the last segment of the import path
(`internal/beta` → `beta`). Suppressed violations stay visible: the
report footer prints `suppressed: N (by //go-arch-lint:ignore
directives)` and the JSON output carries a `SuppressedCount` field, so
technical debt never disappears silently.

### Baseline file: incremental adoption

When there are hundreds of violations and annotating sources with
directives is unrealistic, record them into a baseline file — the check
then fails only on NEW violations ("don't fix everything, don't add
new"):

```bash
# 1. Record the current violations (commit the file to the repository):
go-arch-lint check --baseline .go-arch-lint/baseline.json --baseline-update

# 2. In CI — compare only: known debt is tolerated,
#    a new violation fails the build (exit 1):
go-arch-lint check --baseline .go-arch-lint/baseline.json
```

The baseline is a JSON document with a schema version and violation
fingerprints: `kind|rule|file` without line numbers, so edits higher up
in a file never "resurrect" old debt as new. The text summary prints
`baseline: N new, M known (tolerated)`; the JSON output carries
`BaselineNewCount` and `BaselineKnownCount`. A missing baseline file in
compare mode is a configuration error (exit 2), never a silent pass.
Once the debt is fixed, simply re-record the baseline (stale
fingerprints are ignored).

Under the hood, `check`/`mapping`/`graph`/`selfInspect` delegate to
`.go-arch-lint/` via `go run` — flag routing, exit codes, and caching
(~45 s cold start, ~2 s steady state) are documented in
[docs/delegation.md](docs/delegation.md).

## Dependency graph

```bash
go-arch-lint graph --format=mermaid
```

```
graph LR
  handler --> service
  service --> repository
  handler -.-> n0["3rd-cobra"]
```

Four output formats:

| `--format`  | Output                      | Use case                            |
|-------------|-----------------------------|-------------------------------------|
| `svg`       | file (default)              | ready-to-use image                  |
| `d2`        | stdout                      | d2 source for manual tweaking       |
| `plantuml`  | stdout                      | render via PlantUML or CI           |
| `mermaid`   | stdout                      | Markdown, GitHub, GitLab            |

Additional flags: `--type=di` (reverse graph, DI direction), `--focus=handler` (single component and its deps), `--include-vendors` (show third-party libraries).

## Programmatic API

go-arch-lint is not just a CLI — it's a library. The spec is written in
the Path DSL — the entire language is four calls:

```go
import (
	"github.com/vsfedorenko/go-arch-lint/v3"
	. "github.com/vsfedorenko/go-arch-lint/v3/dsl"
)

func runArchCheck() error {
	build := Spec(func() {
		domain := Path("shop/domain")
		pgx := Vendor("pgx", "github.com/jackc/pgx/v5")

		Path("shop/core", func() {
			Use(domain, pgx) // core uses domain and pgx
		})
	})

	return archlint.Run(build, archlint.WithProjectPath("."))
}
```

Both forms are equivalent: the dot import with `Spec(func() {...})` is the
compact style, while the explicit `Spec(func(s *dsl.SpecBuilder) {...})`
with methods does without the package-level routing. The styles may be
mixed within one spec.

The rules are simple:

- `Path("a/b")` — a directory with Go code, i.e. a component.
  `Path("a/b/**")` — the whole subtree as one component. `Path(".")` —
  the module root (the root package, without subdirectories).
- `Use(...)` — the only rule: "this path uses these targets". Targets
  are `Path`/`Vendor` variables only, mixed freely. Imports inside a
  declared path into its own subdirectories are always allowed.
- `Vendor(name, imports...)` — an external package (multiple import
  paths per name allowed), a legal `Use` target. The standard library
  (`fmt`, `strings`, …) is always allowed and never needs a `Vendor`.
- `Exclude(paths...)` — glob paths outside the architecture: test
  fixtures, examples, nested modules. A directory neither declared in
  `Path` nor excluded fails with `not attached to any component`.
- By default everything is denied until a `Use` allows it.

Declaration order mirrors dependency direction: referring forward is a
Go compile error. Typos and malformed specs panic at build time with
file:line. Directories are verified against the filesystem: a missing
path is a config error with a "did you mean" hint.

`archlint.MustRun(build)` does the same but exits the process with the
conventional exit code: `1` on violations, `2` on a configuration error
(see `archlint.ExitCode`).

`archlint.RunCLI(build, os.Args[1:])` is the entry point for the scaffolded
`.go-arch-lint/main.go`: it routes the delegated commands (`check`,
`mapping`, `graph`, `self-inspect`) to their own behavior instead of
silently running `check`. The launcher dialect (`-p`, `--no-colors`,
`selfInspect`) is translated automatically, and an invocation without a
command defaults to `check`. `MustRunCLI` is the conventional-exit-code
variant.

### Migrating from v2.x

The first-generation DSL (`Version`/`Workdir`/`Component`/`Deps`/
`CommonComponents`) has been removed. Mechanical mapping:

| v2.x (removed)         | v3                                            |
|------------------------|-----------------------------------------------|
| `Component("n", "a/b")`| `n := Path("a/b")`                             |
| `Deps("n", …MayDependOn("m"))` | `Path("a/b", func() { Use(m) })`     |
| `CommonComponents("n")`| a `Use(n)` for everyone who needs it          |
| `AnyProjectDeps(true)` | list the targets explicitly in `Use`          |
| `Vendor(name, imp)` (single path) | `s.Vendor(name, imp1, imp2)`     |
| `Workdir("internal")`  | drop it: `Path` paths are module-relative     |

The easiest route is a fresh scaffold: `rm -rf .go-arch-lint && go-arch-lint init`
regenerates a spec with every directory of your project (`init` refuses to
overwrite an existing directory, and the v2 spec is incompatible anyway). `RunV2`/`MustRunV2`/`RunCLIV2`/
`MustRunCLIV2` were renamed to `Run`/`MustRun`/`RunCLI`/`MustRunCLI`.
The first-generation `Naming`, `Tiers`, `Visibility` and `Interfaces`
rules are not part of v3 — candidates to return as separate extensions.

## Commands

| Command       | Purpose                                          |
|---------------|--------------------------------------------------|
| `init`        | Create `.go-arch-lint/` scaffold (the only flag is `--project-path`/`-p`; architecture templates live in [examples/](examples/)) |
| `check`       | Check project architecture                       |
| `graph`       | Generate dependency graph                        |
| `mapping`     | Show package-to-component mapping                |
| `selfInspect` | Inspect go-arch-lint's own architecture          |
| `version`     | Print version (`version`, `--version`, `-v`)       |

Global flags: `--project-path` (short `-p`), `--max-warnings N` (display cap for violations, default 512; the exit code reflects the full count — see [docs/json-schema.md](docs/json-schema.md#output-cap---max-warnings)), `--format text|json|sarif|junit|github-actions|html` (check), `--baseline <file>` + `--baseline-update` (incremental adoption: known violations are tolerated, only NEW ones fail the check — see [Baseline / incremental mode](#baseline--incremental-mode)), `--output-type` (`ascii`/`json`; an unknown value is a config error), `--json` (shorthand for `--output-type=json`), `--output-json-one-line` (single-line JSON; without json output it is a config error, not a silent no-op), `--output-color` / `--no-colors` (disable ANSI colors).

### Visibility rules

`Visibility(func(){ VisibleTo(...) })` restricts which components may consume another component's API:

```go
Visibility(func() {
    VisibleTo("services")                         // fully internal: nobody may import
    VisibleTo("models", "services", "container")  // only these consumers
})
```

- Checked against the **actual import graph**: importing a restricted component from outside its allow list is a violation
- The component itself is always implicitly allowed
- Multiple rules for one component accumulate (allow lists are unioned)
- Violations point at the first importing file and list the visible set

### Interface placement (ports live with the consumer)

`Interfaces(func(){ MustLiveWithConsumer() })` enforces the hexagonal-ports rule: an interface used by exactly **one other** component must be declared in that component — not next to its implementation:

```go
Interfaces(func() {
	MustLiveWithConsumer()
})
```

```
interface 'UserRepo' must live with its consumer 'service' (declared in component 'repository')
```

- Shared interfaces (2+ consumers) legitimately stay where they are
- Same-component usage (including bare identifiers in the declaring package) counts as internal consumption
- Syntax-only single-pass analysis — fast, no type-checking, no network
- go-arch-lint's own spec enables this rule

### Package naming conventions

`Naming(func(){ ForbiddenPackages(...) })` bans junk-drawer package names (`utils`, `helpers`, `common`, …) — the sinks all code eventually leaks into. Enable it with one declaration in the spec:

```go
Naming(func() {
	ForbiddenPackages("utils", "helpers", "common", "misc", "stuff")
})
```

The checker compares the names of all scanned packages (the actual `package X` clauses, not paths) and reports one violation per package with the file count:

```
Package name utils is forbidden internal/utils (3 file(s)): first at internal/utils/a.go
```

In JSON output (`--format json`) these violations carry the `naming` type. go-arch-lint's own spec already bans `utils`, `helpers`, `common`, `misc`, `stuff`.

### Coupling metrics (mapping)

`mapping -s grouped` reports instability metrics per component (Robert C. Martin):

```
component 'service'
    coupling: out 3 | in 2 | stability 0.60
```

`Ce` (outgoing dependencies), `Ca` (incoming), stability = `Ce/(Ca+Ce)` — closer to 0 means more stable. Also available in JSON via `--format json`.

### JSON output for CI

`check --format json` prints a flat JSON array of violations for CI pipelines; `--format sarif` produces a SARIF 2.1.0 log for GitHub Code Scanning; `--format junit` produces a JUnit-style XML report for CI test dashboards; `--format html` produces a standalone HTML report for humans and archives. Schemas and recipes: [docs/json-schema.md](docs/json-schema.md).

For pull requests there is an official GitHub Action: `::error` annotations
directly on the diff lines, binary installed from the release — no JS glue:

```yaml
- uses: actions/checkout@v7
- uses: actions/setup-go@v7
  with: { go-version: '1.25.13' }
- uses: vsfedorenko/go-arch-lint@v3.1.5
```

Details and all inputs: [docs/json-schema.md → GitHub Action](docs/json-schema.md#github-action-with-inline-annotations).

### Exit codes (check)

| Code | Meaning                                                          |
|------|------------------------------------------------------------------|
| `0`  | No violations                                                    |
| `1`  | Architecture violations found                                    |
| `2`  | Configuration/system error (spec does not compile, project unreadable) |

CI pipelines can branch on this: fail the build on 1, page a maintainer on 2 (a broken config lints nothing).

## Examples

The [`examples/`](examples/) directory contains three demo projects:

— **[basic](examples/basic/)** — layered architecture (handler → service → repository).
— **[ddd](examples/ddd/)** — domain-driven design (domain → application → infrastructure → interfaces).
— **[hexagonal](examples/hexagonal/)** — ports and adapters (core → adapters → domain).

Each example includes a `.go-arch-lint/` directory with an arch-lint configuration (`arch.go` + `main.go`).

## How it works

![How is working](docs/images/how-is-working.png)

The linter maps Go packages to components via glob patterns, extracts imports from AST, builds the actual dependency graph, and compares it to the desired graph from the configuration. Mismatches are architecture violations.

Deep scan mode analyzes method calls and dependency injections — not just imports, but structural type usage between components.

## License

[MIT](LICENSE). Forked from [go-arch-lint](https://github.com/fe3dback/go-arch-lint) © [fe3dback](https://github.com/fe3dback).
