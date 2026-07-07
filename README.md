![Logo image](./docs/images/logo.png)

Linter used to enforce some good project structure and validate top level architecture (code layers) 

[![Go Report Card](https://goreportcard.com/badge/github.com/fe3dback/go-arch-lint)](https://goreportcard.com/report/github.com/fe3dback/go-arch-lint)
[![go-recipes](https://raw.githubusercontent.com/nikolaydubina/go-recipes/main/badge.svg?raw=true)](https://github.com/nikolaydubina/go-recipes)

## Quick start

### What exactly is project architecture?

You can imagine some simple architecture, for example classic onion part from "clean architecture":

![Layouts example](./docs/images/layout_example.png)

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

see [config syntax](docs/syntax/README.md) for details.

And now linter will check all project code inside `internal` workdir
and show warnings, when code violate this rules.

For best experience you can add linter into CI workflow

### Example of broken code

Imagine some `main.go`, when we provide `repository` into `handler` and get some bad
flow:

```go
func main() {
  // ..
  repository := booksRepository.NewRepository()
  handler := booksHandler.NewHandler(
    service,
    repository, // !!!
  )
  // ..
}
```

Linter will easily found this issue:

![Check stdout example](./docs/images/check-example.png)

### Install/Run

#### Wia Docker

```bash
docker run --rm -v ${PWD}:/app fe3dback/go-arch-lint:latest-stable-release check --project-path /app
```

[other docker tags and versions](https://hub.docker.com/r/fe3dback/go-arch-lint/tags)

#### From sources
It requires go 1.25+

```bash
go install github.com/fe3dback/go-arch-lint@latest
```

Scaffold the arch config in your project:

```bash
cd ~/code/my-project
go-arch-lint init
```

This creates a `.go-arch-lint/` directory with `go.mod`, a generated `main.go`,
and a starter `arch.go` for you to edit. Then run the linter:

```bash
go-arch-lint check
```

#### Precompiled binaries

[see on releases page](https://github.com/fe3dback/go-arch-lint/releases)

## Usage

### How to add linter to existing project?

![Adding linter steps](./docs/images/add-linter-steps.png)

Adding a linter to a project takes several steps:

1. Current state of the project
2. Run `go-arch-lint init` to scaffold `.go-arch-lint/`, then edit `arch.go` to describe the ideal project architecture
3. Linter find some issues in the project. Don't fix them right now, but "legalize" them by adding them to the config and marking `todo` with the label
4. In your free time, technical debt, etc. fix the code
5. After fixes, clean up config to target state

### Execute

```
Usage:
  go-arch-lint check [flags]

Flags:
      --arch-file string      arch file path (default ".go-arch-lint/arch.go")
  -h, --help                  help for check
      --max-warnings int      max number of warnings to output (default 512)
      --project-path string   absolute path to project directory (where '.go-arch-lint/' is located) (default "./")

Global Flags:
      --json                   (alias for --output-type=json)
      --output-color           use ANSI colors in terminal output (default true)
      --output-json-one-line   format JSON as single line payload (without line breaks), only for json output type
      --output-type string     type of command output, variants: [ascii, json] (default "default")
```

This linter will return:

| Status Code | Description                      |
|-------------|----------------------------------|
| 0           | Project has correct architecture |
| 1           | Found warnings                   |


### How is working?

![How is working](./docs/images/how-is-working.png)

Linter will:
- match/mark **go packages** with **components**
- finds all dependencies between components
- build a dependency graph
- compares the actual (code) and desired (config) dependency graph
- if it got a non-empty DIFF, then project has some issues

## Graph

Example config of this repository: [.go-arch-lint/arch.go](.go-arch-lint/arch.go)

![graph](./docs/images/graph-example.png)

You can generate dependencies graph with command `graph`:

```bash
go-arch-lint graph
```

See full [graph documentation](docs/graph/README.md) for details.