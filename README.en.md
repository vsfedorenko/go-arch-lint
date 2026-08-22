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
go install github.com/vsfedorenko/go-arch-lint/v2/cmd/arch-lint@latest
```

Or use [Docker](https://github.com/vsfedorenko/go-arch-lint/pkgs/container/go-arch-lint):

```bash
docker run --rm -v ${PWD}:/app ghcr.io/vsfedorenko/go-arch-lint:latest check --project-path /app
```

Or grab a [binary from releases](https://github.com/vsfedorenko/go-arch-lint/releases).

> The module path is `github.com/vsfedorenko/go-arch-lint/v2` (the suffix is
> mandatory for Go major versions). Upgrading from v2.0/v2.1: update the
> imports in `.go-arch-lint/` to `/v2`, or just re-run `go-arch-lint init`.

## Requirements

- **Go 1.25+** (the two latest major versions are supported: 1.25 and 1.26; CI tests both).
- `go` must be on your `PATH` — the CLI compiles `.go-arch-lint/arch.go` via `go run` (cached, steady-state runs take ~2s).

## Configuration

The config is a Go file. Not YAML, not JSON — plain Go code with type safety and IDE autocomplete.

```bash
cd ~/code/my-project
go-arch-lint init
```

Creates `.go-arch-lint/` with three files. The spec and the runner are separate: `arch.go` is yours to edit, `main.go` is generated plumbing you never touch:

```go
// arch.go — describe your architecture here
package main

import (
	v2 "github.com/vsfedorenko/go-arch-lint/v2/dsl/v2"
)

var build = v2.Spec(func(s *v2.SpecBuilder) {
	// Every directory with Go code is declared: the v2 language
	// fails on undeclared directories. Add Use rules as your
	// architecture takes shape:
	//
	//     domain := s.Path("internal/domain")
	//     s.Path("internal/core", func() { s.Use(domain) })
	s.Path(".")
	s.Path("cmd/app")
	s.Path("internal/handlers")
	s.Path("internal/services")
})
```

```go
// main.go — generated, do not edit: forwards CLI flags into the check
package main

import (
	"os"

	"github.com/vsfedorenko/go-arch-lint/v2"
)

func main() {
	archlint.MustRunCLIV2(build, os.Args[1:])
}
```

How this works:

— `init` scans the project and declares every directory with Go code as a component (`Path`). The fresh scaffold is honest: all paths are declared, and real violations of missing `Use` rules show up immediately.
— `Use` is the only rule: "this path uses these targets". Without an explicit `Use`, everything is denied.
— `Vendor(name, import)` — an external library as a legal target; the standard library is always allowed.
— Declaration order mirrors dependency direction: referring forward is a Go compile error.

Full v2 walkthrough — in the [v2 DSL](#v2-dsl-experimental) section below.
The first-generation DSL reference (`Component`/`Deps`/`Workdir`) lives in
the [syntax docs](docs/syntax/README.md) or via `go doc github.com/vsfedorenko/go-arch-lint/v2/dsl`.

### Init recipes

Skip the blank page — start from a known architecture pattern:

```bash
go-arch-lint init --recipe hexagonal   # ports & adapters: domain+core at the center, http/db depend inward
go-arch-lint init --recipe ddd         # DDD: bounded contexts + application/infrastructure/interfaces
go-arch-lint init --recipe clean       # clean architecture: domain ← usecase ← delivery
```

A recipe writes the same scaffold (`.go-arch-lint/`), but `arch.go` already describes the layers and dependency rules of the chosen pattern in the first-generation DSL (`Spec`/`Component`/`Deps` — reference in [docs/syntax](docs/syntax/README.md)). The spec sets `IgnoreNotFoundComponents(true)` — you can create the layer directories gradually; the linter won't fail while a layer is still missing (v2 offers no such leniency: it requires the directories to exist).

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

go-arch-lint is not just a CLI — it's a library. Run checks from Go code:

```go
import (
	"github.com/vsfedorenko/go-arch-lint/v2"
	. "github.com/vsfedorenko/go-arch-lint/v2/dsl"
)

func runArchCheck() error {
	spec := Spec(func() {
		Version(1)
		Workdir("internal")
		Component("handler", "handlers/*")
		Component("service", "services/**")
		Deps("handler", func() { MayDependOn("service") })
	})

	return archlint.Run(spec,
		archlint.WithProjectPath("."),
		archlint.WithMaxWarnings(100),
	)
}
```

`archlint.MustRun(spec)` does the same but exits the process with the
conventional exit code: `1` on violations, `2` on a configuration error
(see `archlint.ExitCode`).

`archlint.RunCLI(spec, os.Args[1:])` is the entry point for the scaffolded
`.go-arch-lint/main.go`: it routes the delegated commands (`check`,
`mapping`, `graph`, `self-inspect`) to their own behavior instead of
silently running `check`. The launcher dialect (`-p`, `--no-colors`,
`selfInspect`) is translated automatically, and an invocation without a
command defaults to `check`. `MustRunCLI` is the conventional-exit-code
variant.

### v2 DSL (experimental)

The DSL above is the first generation (`dsl`). There is a second, simpler
one — the entire language is four calls.

```go
import (
	"github.com/vsfedorenko/go-arch-lint/v2"
	v2 "github.com/vsfedorenko/go-arch-lint/v2/dsl/v2"
)

func runArchCheck() error {
	build := v2.Spec(func(s *v2.SpecBuilder) {
		domain := s.Path("shop/domain")

		s.Path("shop/core", func() {
			s.Use(domain) // core may use domain
		})
	})

	return archlint.RunV2(build, archlint.WithProjectPath("."))
}
```

The rules are simple:

- `Path("a/b")` — a directory with Go code, i.e. a component.
  `Path("a/b/**")` — the whole subtree as one component. `Path(".")` —
  the module root (the root package, without subdirectories).
- `Use(...)` — the only rule: "this path uses these targets". Targets
  are `Path`/`Vendor` variables only, mixed freely.
- `Vendor(name, import)` — an external package, a legal `Use` target.
  The standard library (`fmt`, `strings`, …) is always allowed and
  never needs a `Vendor`.
- By default everything is denied until a `Use` allows it. Every
  directory with Go files must be declared with `Path` — an undeclared
  directory is not "ignored", it fails with `not attached to any
  component`.

Declaration order mirrors dependency direction: referring forward is a
Go compile error. Typos and malformed specs panic at build time with
file:line. Directories are verified against the filesystem: a missing
path is a config error with a "did you mean" hint.

`archlint.MustRunV2(build)` is the conventional-exit-code variant.
`archlint.RunCLIV2(build, os.Args[1:])` / `MustRunCLIV2` is the CLI wrapper
over a v2 build: it routes the delegated commands (`check`, `mapping`,
`graph`, `self-inspect`) to their own behavior; an invocation without a
command defaults to `check`. This is what the `init` scaffold writes into
`main.go`.

The package is experimental: the API may change before it replaces the
first generation (stage 4 of the v3 roadmap).

## Commands

| Command       | Purpose                                          |
|---------------|--------------------------------------------------|
| `init`        | Create `.go-arch-lint/` scaffold; `--recipe clean\|hexagonal\|ddd` starts from a known pattern |
| `check`       | Check project architecture                       |
| `graph`       | Generate dependency graph                        |
| `mapping`     | Show package-to-component mapping                |
| `selfInspect` | Inspect go-arch-lint's own architecture          |
| `version`     | Print version                                    |

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
- uses: vsfedorenko/go-arch-lint@v2.4.3
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
