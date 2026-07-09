[Русский](README.md) | [**English**](README.en.md)

---

![Logo image](docs/images/logo.png)

Architecture linter for Go: describe your layers and dependency rules in a Go DSL — the linter finds import and dependency injection violations.

[![Go Report Card](https://goreportcard.com/badge/github.com/vsfedorenko/go-arch-lint)](https://goreportcard.com/report/github.com/vsfedorenko/go-arch-lint)
[![go-recipes](https://raw.githubusercontent.com/nikolaydubina/go-recipes/main/badge.svg?raw=true)](https://github.com/nikolaydubina/go-recipes)

## Install

```bash
go install github.com/vsfedorenko/go-arch-lint@latest
```

Or use [Docker](https://github.com/vsfedorenko/go-arch-lint/pkgs/container/go-arch-lint):

```bash
docker run --rm -v ${PWD}:/app ghcr.io/vsfedorenko/go-arch-lint:latest check --project-path /app
```

Or grab a [binary from releases](https://github.com/vsfedorenko/go-arch-lint/releases).

## Configuration

The config is a Go file. Not YAML, not JSON — plain Go code with type safety and IDE autocomplete.

```bash
cd ~/code/my-project
go-arch-lint init
```

Creates `.go-arch-lint/` with `go.mod` and `main.go`:

```go
package main

import (
	"github.com/vsfedorenko/go-arch-lint"
	. "github.com/vsfedorenko/go-arch-lint/dsl"
)

var spec = Spec(func() {
	Version(1)
	Workdir("internal")

	Component("handler", "handlers/*")
	Component("service", "services/**")
	Component("repository", "domain/*/repository")

	CommonComponents("model")

	Deps("handler", func() {
		MayDependOn("service")
	})
	Deps("service", func() {
		MayDependOn("repository")
	})
})

func main() {
	archlint.MustRun(spec)
}
```

How this works:

— `Workdir` sets the root below which the linter scans for Go packages.
— `Component` maps a component name to a path glob pattern.
— `Deps` declares which components a component may depend on.
— `CommonComponents` — components available to everyone (utilities, models).
— `Vendor` and `CanUse` — third-party libraries allowed for a specific component.

Full DSL function reference: [syntax docs](docs/syntax/README.md) or `go doc github.com/vsfedorenko/go-arch-lint/dsl`.

## Check

```bash
go-arch-lint check
```

The linter builds an import graph from the actual code, compares it to the configured dependency graph, and reports violations:

![Check output](docs/images/check-example.png)

| Exit code | Meaning                    |
|-----------|----------------------------|
| 0         | No violations              |
| 1         | Violations found           |

Use `--json` for machine-readable output in CI pipelines.

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
	"github.com/vsfedorenko/go-arch-lint"
	. "github.com/vsfedorenko/go-arch-lint/dsl"
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

`archlint.MustRun(spec)` does the same but calls `os.Exit(1)` on error.

## Commands

| Command       | Purpose                                          |
|---------------|--------------------------------------------------|
| `init`        | Create `.go-arch-lint/` scaffold                 |
| `check`       | Check project architecture                       |
| `graph`       | Generate dependency graph                        |
| `mapping`     | Show package-to-component mapping                |
| `selfInspect` | Inspect go-arch-lint's own architecture          |
| `version`     | Print version                                    |

Global flags: `--project-path`, `--output-type` (`ascii`/`json`), `--json`, `--output-color`.

## Examples

The [`examples/`](examples/) directory contains three demo projects:

— **[basic](examples/basic/)** — layered architecture (handler → service → repository).
— **[ddd](examples/ddd/)** — domain-driven design (domain → application → infrastructure → interfaces).
— **[hexagonal](examples/hexagonal/)** — ports and adapters (core → adapters → domain).

Each example includes a `.go-arch-lint/main.go` with an arch-lint configuration for the corresponding pattern.

## How it works

![How is working](docs/images/how-is-working.png)

The linter maps Go packages to components via glob patterns, extracts imports from AST, builds the actual dependency graph, and compares it to the desired graph from the configuration. Mismatches are architecture violations.

Deep scan mode analyzes method calls and dependency injections — not just imports, but structural type usage between components.

## License

[MIT](LICENSE). Forked from [go-arch-lint](https://github.com/fe3dback/go-arch-lint) © [fe3dback](https://github.com/fe3dback).
