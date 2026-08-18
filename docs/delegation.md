# The `go run .go-arch-lint/` Delegation Model

`go-arch-lint` is two programs, not one:

1. **The launcher** (`go install github.com/vsfedorenko/go-arch-lint/cmd/arch-lint@latest`)
   — a tiny, dependency-free binary. It handles `init`, `version`, `help`, and
   delegates everything else.
2. **The arch spec** (`.go-arch-lint/`) — *your* Go module, scaffolded by
   `init` and edited by you. It imports the `go-arch-lint` library and DSL,
   so your config is type-checked Go code with IDE completion, not a stringly
   YAML file.

For `check`, `mapping`, `graph`, and `selfInspect` the launcher runs:

```bash
go -C <project>/.go-arch-lint run . <command> [flags]
```

This document is an audit of what that means in practice: how flags flow,
what is cached and what a cold start costs, every failure mode and its exit
code, and the failure modes you should not be surprised by.

## Why delegation

- Your spec compiles **against the library**, so the DSL version you use is
  pinned by *your* `go.mod` — the launcher never needs to match it.
- The launcher binary stays dependency-free: `go install` builds it in
  seconds and there is no plugin/runtime bridge to break.
- Spec code runs with full Go tooling: compiler errors, `go vet`, debugger,
  refactoring, code review.

## What the launcher does to your flags

| Step | Behavior |
|------|----------|
| Command dispatch | `init`/`version`/`help` are local; any other command delegates |
| `--project-path` / `-p` | Rewritten to an **absolute** path before delegation (the spec module runs from inside `.go-arch-lint/`) |
| No `--project-path` given | `--project-path <abs>` is **appended** — the spec's default (`../`) is overridden deterministically |
| Other flags | Passed through verbatim (`--format json`, `--max-warnings`, `--no-colors`, `--output-type`, `--json`, `--output-json-one-line`, …) and parsed by the scaffolded runner (`archlint.OptionsFromFlags`) |
| stdout / stdin | Streamed through untouched (JSON output is pipe-safe) |
| stderr | **Teed**: shown to you *and* captured, because the child's exit code is only available there (see below) |

## Exit codes: how the child's code survives `go run`

`go run` does not propagate the child's exit code — any non-zero child exit
becomes `go run`'s exit `1`, with the real code mentioned only in the last
stderr line (`exit status N`). The launcher parses that line back, so the
linter convention survives delegation:

| Launcher exit | Meaning |
|---------------|---------|
| `0` | Child exited `0` (no violations) |
| `1` | Child exited `1` — architecture violations found (or usage error) |
| `2` | Config/system error: spec did not compile, `go run` itself failed, `go` missing, signal |

A non-zero exit **without** an `exit status N` line is always a build or
delegation failure → mapped to `2`, never mistaken for "violations found".

## Caching and cost

Delegation is `go run`, so the Go build cache does the work. Numbers below
are from a mid-range Linux box, go 1.25, this repo's own spec — treat them
as magnitudes, not benchmarks:

| Scenario | Cost | Why |
|----------|------|-----|
| First run ever (empty module **and** build cache) | ~45 s | downloads the module graph, compiles the full dependency tree |
| `go mod tidy` on a scaffold (modules already downloaded) | ~2 s | resolves versions, writes `go.sum`; no compiling yet |
| First `check` after tidy | ~10 s | compiles the dependency tree once into the build cache |
| Steady-state `check` (nothing changed) | **~2 s** | everything is cached; `go run` links and executes |
| After **editing the spec** | **~2 s** | only the tiny `.go-arch-lint/` module recompiles — the heavy dependency tree stays cached |

Practical consequences:

- **Day-to-day cost is ~2 s**, including after every spec edit. The scary
  numbers are one-time.
- The cache is keyed by content and lives in `GOCACHE` (build) and
  `GOMODCACHE` (modules). A Go toolchain upgrade or `go clean -cache`
  resets the build cache; CI runners without caching pay the cold cost on
  every job.
- **CI advice**: use `actions/setup-go` with `cache: true` (or cache
  `GOCACHE` + `GOMODCACHE` yourself) and run `go mod tidy` inside
  `.go-arch-lint/` as a cached step. A cached CI leg behaves like the
  steady-state row.
- `go-arch-lint check` in a pre-commit hook is fine; the first cold run is
  the only expensive one.

## Failure modes (audited)

Each row was reproduced against a real scaffold:

| # | Situation | What you see | Exit |
|---|-----------|--------------|------|
| 1 | `.go-arch-lint/` missing | `Error: .go-arch-lint/ directory not found at …` + `Run 'go-arch-lint init' first` | `1` |
| 2 | `go` not on PATH | explicit hint: delegation needs `go`, install link | `2` |
| 3 | Scaffold not tidied (`go.sum` absent) | go's own `go get …/dsl` suggestion + footer `The arch spec … did not build. Fix the compile errors above, or regenerate the scaffold with 'go-arch-lint init'.` | `2` |
| 4 | Spec does not compile (any Go error) | raw compiler errors, then the same footer pointing at `.go-arch-lint/arch.go` | `2` |
| 5 | Violations found | normal report | `1` |
| 6 | Child crashes / signal | no `exit status N` line → config/system error | `2` |

The two system footers (#2, #4) exist so neither failure mode is ever silent
or context-free; they are covered by `cmd/arch-lint/errors_ux_test.go`, and
exit-code mapping by `exitcode_test.go` plus `test/check/integration_test.go`
(which parses the child exit out of `go run`'s stderr).

## Known sharp edge: scaffold ahead of the published library

`go-arch-lint init` scaffolds `arch.go` (the spec) plus `main.go` (a stable runner) with flag passthrough — the split keeps the spec and the runner independently replaceable:

```go
archlint.MustRun(spec, archlint.OptionsFromFlags(os.Args[1:])...)
```

`OptionsFromFlags` is newer than the currently published library release, so
a **fresh scaffold tidied against the published version does not compile**
(`undefined: archlint.OptionsFromFlags`) until the next library release
ships. This repo dogfoods the scaffold through a `replace` directive, so CI
does not see it. Until the release: after `init`, either pin the module to
the upcoming version or replace the call with `archlint.MustRun(spec)` (flags
then fall back to their defaults).

## Library users bypass all of this

Delegation is a CLI concern. If you embed the check instead —
`archlint.Run(spec, archlint.WithProjectPath("."))` — there is no child
process, no stderr parsing, and no `go run` startup: the check runs
in-process. See `go doc github.com/vsfedorenko/go-arch-lint`.
