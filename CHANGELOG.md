# Changelog

## [Unreleased]

## [3.1.4] — 2026-08-24

### Fixed
- The `help` header carried a hardcoded major (`go-arch-lint v3.0 — Go
  architectural linter`) while `version` reported the real release: the
  v3.1.3 binary greeted users with "v3.0". The header now resolves the
  version the same way `version` does (goreleaser ldflags, then module
  build info), so the two can never diverge again. Found by the
  post-release consumer probe of v3.1.3.

## [3.1.3] — 2026-08-23

### Fixed
- `init` silently ignored every token it did not know: `init --recipe`
  (a v1/v2 flag removed in v3, still advertised by the README) scaffolded
  the DEFAULT spec at the DEFAULT path with exit 0, and
  `init --project-path -p` took the next flag as its value and created the
  scaffold under a literal `-p/` directory. `parseInitArgs` now fails fast
  naming the token (`unknown flag or argument: …`) — init takes no
  positional arguments. README RU/EN: the stale `--recipe` section is
  replaced with the real `examples/` templates route, command tables
  updated. Found by the post-merge consumer probe of v3.1.2. (#116)

## [3.1.2] — 2026-08-23

### Fixed
- A flag-like first token was silently delegated as a command name:
  `go-arch-lint --version` outside a project answered with the misleading
  `.go-arch-lint/ directory not found` config error (told to run `init`
  when asking for the version), and any unknown leading flag degraded the
  same way instead of failing fast. The launcher now serves
  `--version`/`-v`/`-V` locally (same output as `version`) and rejects
  every other leading flag with `unknown flag or command: <token>` +
  exit 1. Found by the post-merge consumer probe of v3.1.1.

## [3.1.1] — 2026-08-22

### Fixed
- The default `init` scaffold was red on the canonical Go project shape:
  the module-root package importing an internal package (`main.go` →
  `internal/...`) failed the "3. Run check" next-step with
  `Component . shouldn't depend on ...` — declare-everything declared the
  paths but no `Use` rules, and everything is denied by default. `init`
  now scans non-test imports (module path from go.mod) and renders every
  existing import as a `Use` rule, dependencies-first (Kahn's order), with
  collision-safe identifiers (`app`/`app2`, keyword escapes). The fresh
  scaffold mirrors the code as-is and checks green on day one; tightening
  is deleting a `Use` line. Test files are not scanned (the checker does
  not flag test imports). Found by probing a fresh `init` as a consumer;
  pinned by a new integration test with a root→internal import fixture.

## [3.1.0] — 2026-08-22

### Added
- Package-level `Path` / `Use` / `Vendor` / `Exclude` sugar: specs can now be
  written receiverless with a dot import, and `Spec(fn)` accepts both
  closure shapes — `func()` (dot-import style) and `func(s *SpecBuilder)`
  (explicit builder style, unchanged). Routing is goroutine-local, so
  parallel Specs stay isolated; both styles may be mixed in one Spec.
  Panics and Use diagnostics keep pointing at the user's spec line (#107).
- The `init` scaffold, every doc example and the examples/ specs now use
  the sugar form; the explicit builder form stays documented as the
  equivalent alternative (#108).
- examples/*/.go-arch-lint: `replace` now points at the repo root with a
  portable relative path (#108).

### Fixed
- Fresh `init` scaffolds broke at `go mod tidy` time: the template used the
  sugar form before it existed in a published release (`Spec(func(){…})` did
  not compile against v3.0.4). Release v3.1.0 ships the sugar API, making
  every `go-arch-lint init` → `go mod tidy` → `check` run green again.
  Found by probing the scaffold as a consumer.

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [3.0.4] — 2026-08-22

### Fixed
- `version` printed `launcher dev (commit unknown, built unknown)` on
  `go install`-built binaries: the launcher printed the raw ldflags
  variables instead of asking the version operation, whose
  `debug.ReadBuildInfo` fallback already resolves the module version.
  `go install ...@v3.0.4` now reports the real version. Goreleaser
  builds (ldflags) are unchanged. Found by probing the released
  artifact as a consumer.
- `graph` polluted stdout with two `WARN missing slog.Logger in context`
  blocks carrying full goroutine stack traces (~80 lines) on every svg
  run: d2's internal logger warns whenever the context carries no
  `slog.Logger`, and the layout engine calls it twice per compile. The
  graph operation now attaches a WARN-capped logger to the context, so
  the debug chatter stays silent while real d2 errors keep flowing
  through the returned error. Found by probing the released v3.0.3
  artifact as a consumer.

## [3.0.3] — 2026-08-22

### Fixed
- The delegated `graph` command wrote its svg into `.go-arch-lint/`
  instead of the project root: the delegated process runs with
  `cwd=.go-arch-lint/`, so the command's default relative output path
  resolved against the wrong directory. The launcher now absolutizes
  every path-carrying flag (`--project-path`, `--baseline`, `--out`) in
  both `--flag value` and `--flag=value` forms and pins `graph`'s
  default output to the project root (#96).
- A path flag without a value (`go-arch-lint graph --out`) silently
  fell back to the default output location and exited 0 — the launcher's
  own appended defaults satisfied the delegated parser. Valueless path
  flags now fail fast with `Error: flag needs an argument: --out` and
  exit 1 (#96).
- Repo hygiene: a 26 MB launcher binary accidentally committed in #47
  is untracked and gitignored (#99).

## [3.0.1] — 2026-08-22

### Fixed
- `graph` failed to compile with `invalid text beginning unquoted key` on
  every spec declaring the module root (`Path(".")`) — d2 keys are now
  quoted, so path-shaped component names (`.`, `pkg.v1`, `internal/a`)
  survive d2 compilation in the default svg format. Regression of v3.0.0:
  v2 component aliases never collided with d2's reserved tokens.
- Self-dependency edges (the implicit "a component may import itself"
  rule) no longer render as self-loops in any graph format — they were
  noise, not architecture.

## [3.0.0] — v3: the Path DSL is the only DSL

### Removed (breaking)
- The first-generation DSL (`Version`/`Workdir`/`Component`/`Deps`/
  `CommonComponents`/`Allow`/`ExcludeFiles` and the `Naming`/`Tiers`/
  `Visibility`/`Interfaces` rule builders) — the `dsl` package now
  contains only `Spec`/`Path`/`Use`/`Vendor`/`Exclude`.
- `archlint.Run`/`RunCLI` over a v1 `SpecDef`; the YAML-free
  `GoSpecDecoder` (`decoder_go.go`) and the `--recipe` init flag with
  its `clean|hexagonal|ddd` recipes (candidates to return as v3
  preset functions).

### Changed (breaking)
- Module path: `github.com/vsfedorenko/go-arch-lint/v3` (imports,
  scaffold templates and fixtures migrated; `require` lines use
  `v3.0.0-dev` locally, `v3.0.0` once tagged).
- `RunV2`/`MustRunV2`/`RunCLIV2`/`MustRunCLIV2` renamed to
  `Run`/`MustRun`/`RunCLI`/`MustRunCLI` — the v2 pipeline is the only
  pipeline.
- `Vendor(name, import)` is variadic: `s.Vendor("pgx", imp1, imp2)`
  declares one vendor name over several import paths.
- Component names in output are path keys (`internal/a`), not the
  v1 free-form aliases.
- `WithOutputType`/`WithFormat` values are validated before the
  renderer runs — an unknown format is a config error (exit 2), not a
  panic (found by the v3 test migration).

### Added
- `Exclude(paths...)` — glob paths outside the architecture (test
  fixtures, examples, nested modules); declare-everything otherwise
  makes monorepos inexpressible.
- Component self-imports are always allowed: a `Path` may import its
  own subdirectories without a self-`Use` (decoder emits the self
  rule for every declared path).
- Migration guide: README → «Миграция с v2.x» / “Migrating from v2.x”.

## [3.0.2]

### Fixed
- `init` no longer declares nested Go modules (directories with their own `go.mod`) and `testdata` directories as components; both are scaffolded as explicit `s.Exclude` globs instead, so a fresh `init` produces a spec `check` agrees with on monorepos.
- README (RU/EN): the v2→v3 migration note no longer suggests re-running `init` over an existing `.go-arch-lint` (it refuses); the documented route is now `rm -rf .go-arch-lint && go-arch-lint init`.

## [2.6.0] — 2026-08-22

### Fixed
- `init --recipe <name>` scaffolds a compiling pair again: recipes write a
  first-generation spec (`var spec`), but the shared runner referenced the
  v2 `build` variable introduced in #89 — the generated `main.go` did not
  compile ("undefined: build"). Recipes now get the matching v1 runner
  (`MustRunCLI(spec, …)`); the default scaffold keeps the v2 runner. The
  recipe integration test now runs the launcher's OWN `main.go` (a
  hand-written runner used to hide this class of drift).

### Changed
- `init` scaffolds the v2 Path-based DSL (stage 3 of the v3 roadmap):
  the generated `arch.go` declares every directory containing Go code
  as a `Path()` (the declare-everything rule holds day one; the module
  root itself is declared as `.` when it has Go files), and the runner
  calls `archlint.MustRunCLIV2`. New public API: `RunCLIV2` /
  `MustRunCLIV2` — the delegated CLI surface (check, mapping, graph,
  self-inspect, version) over a v2 build. The v2 pipeline now excludes
  `.go-arch-lint/` and `vendor/` unconditionally (the linter's own
  scaffold is never part of the architecture), and `Path(".")` is the
  root package only — matching the README contract.
- README (RU/EN): the Configuration section now shows the real v2
  scaffold `init` writes; recipes are documented as first-generation
  specs; `RunCLIV2`/`MustRunCLIV2` documented in the v2 DSL section.
  EN translation caught up with the interface-placement and naming
  sections (structural parity: H2/H3/fence/table counts equal).

## [2.5.0] — 2026-08-22

### Added
- v2 Path-based DSL core (stage 1 of the v3 roadmap): the `dsl/v2`
  package — `Spec`/`Path`/`Use`/`Vendor`. A directory containing Go
  code IS a component; `Use` is the only rule ("this path uses these
  targets"); defaults deny everything until a `Use` says otherwise.
  Malformed specs panic at build time with file:line (duplicate
  paths, `Use` outside a `Path` fn, self-use, raw strings as
  targets, misplaced `**`). Forward references are a Go compile
  error — declaration order mirrors dependency direction.
- v2 DSL pipeline integration (stage 2 of the v3 roadmap):
  `V2SpecDocument` adapts a `dsl/v2.Build` to the checker's spec surface
  (paths → components with `/**` globs, Use rules → dependency rules with
  path/vendor targets split), `FSVerify` checks every declared path
  against the real filesystem (missing directories get a did-you-mean
  suggestion from sibling directories; empty `/**` subtrees are named),
  and `archlint.RunV2(build, opts...)` runs the full check pipeline over
  a v2 build — verified live: a conforming chain passes clean, an
  illegal import reports the violating component (and cycles).
  `Path(".")` now declares the module root.
- `archlint.MustRunV2` — the v2 counterpart of [MustRun] (exit-code
  plumbing for CLI-style runners), plus end-to-end coverage of the v2
  pipeline (`test/check/integration_v2_test.go` + the
  `test/check/project-v2` fixture module): clean spec → exit 0, upward
  import → exit 1 with the component named and the cycle reported,
  missing path → exit 2 with a did-you-mean hint.

### Fixed
- v2 root component: `Path(".")` produced a component with an EMPTY
  name — violations rendered as "Component  shouldn't depend on",
  `Use(root)` panicked with a misleading "never assigned from
  Path(...)" (the empty name collided with the zero-value PathID), and
  `Use(...)` inside `Path(".", func(){...})` panicked "must be called
  inside Path". The module root now uses the canonical key `"."`, and
  the dead `from == ""` guard is gone. Found by probing the freshly
  merged stage-2 pipeline live as a consumer; pinned by
  `TestPath_Root`, `TestUse_InsideRootPath`, and the v2 e2e suite.

## [2.4.3] — 2026-08-21

### Changed
- Synced with upstream fe3dback/go-arch-lint (through #87): Go 1.27
  support via golang.org/x dependency upgrades (x/mod v0.40.0, x/tools
  v0.49.0), spec-validator fixes, and upstream's stable unit tests for
  the holder/scanner. Fork-authoritative surfaces kept: v2 Go-DSL docs,
  the release pipeline, the testify-standalone test suites, and the
  v2 URL-archfile stub (upstream's http.Get loader intentionally not
  taken — v2 specs are local Go DSL). (#79)

## [2.4.2] — 2026-08-22

### Added
- Unit suites for every previously zero-coverage operation and service
  layer: check (0% → 61.5%), mapping (0% → 98.3%), selfInspect (0% →
  100%), the project-files resolver (0% → 100%), the embedded
  view-template map (0% → 100%), and version sourcing (0% → 72.7%).
  Pinned contracts include the exit-code trichotomy, the
  "[not attached]" bucket leading the grouped mapping, JSON-safe empty
  annotation lists, and scan-directory cleaning.

### Changed
- README (EN/RU): the GitHub Action example pins the real release tag
  (`@v2.4.1`) instead of `@main`, and CI snippets use `go-version:
  '1.25.13'` matching the repo's go directive.

## [2.4.1] — 2026-08-21

### Fixed
- Flag-pair validation now covers the delegated cobra path, not only the
  SDK entry point: since the scaffold ships `archlint.MustRunCLI` as its
  runner (#64), `check --baseline-update` without `--baseline` silently
  ran a plain check that recorded nothing (exit 0/1), and
  `--output-json-one-line` without json output was silently ignored on
  `check`/`mapping`/`graph` — while both READMEs promise "a config
  error, not a silent no-op". Both combinations now fail fast with an
  actionable config error (exit 2) on every command, via one shared
  rule set (`models.CheckOptions.ValidateFlagPairs`) used by BOTH entry
  paths. Also fixes the `max-warnings` range message typo
  ("should by" → "should be"). Found by probing the built launcher
  against a freshly scaffolded project (#72).

## [2.4.0] — 2026-08-21

### Fixed
- The default `init` scaffold checks green out of the box: it used to
  ship with every component commented out (spec invalid: "at least one
  component must be defined") and `Workdir("internal")` (invalid on
  projects without `internal/`) — the very next step init suggests
  ("run check") was guaranteed to fail. The template now ships one
  catch-all component, `Workdir(".")`, and excludes `.go-arch-lint/`
  itself. Pinned by black-box tests (scaffold → check on a real module
  and on an empty one).
- `check --help` outside a project prints usage instead of a config
  error (#68).
- Security/CI hardening: dependabot (gomod + github-actions), OSSF
  scorecard on main pushes, scorecard-action pinned (the upstream moving
  `v2` tag was unpublished).

## [2.3.0] — 2026-08-20

### Security
- CI security suite (`.github/workflows/security.yml`): govulncheck on
  every push/PR plus a daily cron (reachable vulnerabilities only),
  CodeQL analysis (Go), gitleaks over the full history, dependency
  review on PRs (fails on high/critical).
- `go` directive bumped 1.25.0 → 1.25.13: govulncheck reported 15
  reachable stdlib vulnerabilities (net/url, html/template,
  crypto/x509, …) — all fixed by the toolchain patch release.

### Fixed
- Scaffolded `.go-arch-lint` runners delegate subcommands (`check`,
  `graph`, …) to the installed CLI via `archlint.MustRunCLI` instead of
  failing on the first unknown flag (#64).
- Example trees no longer ship compiled `arch-lint-local` binaries
  (~58 MB off the repo); local rebuilds are gitignored (#62).

## [2.2.0] — 2026-08-20

### Fixed
- **Module path carries the mandatory `/v2` suffix** (go semver): before,
  `go install github.com/vsfedorenko/go-arch-lint@v2.1.0` failed with
  "module path must match major version", and `@latest` silently served
  the old v1.0.0. Install/import paths now end with `/v2`; the GitHub
  repo, releases, and Docker images are unchanged. Scaffolded projects:
  update imports or re-run `init`.

### Added
- golangci-lint expanded: testifylint, gocritic (diagnostics), thelper,
  intrange, unparam, noctx, contextcheck, nilerr, usestdlibvars, dogsled.
  Real findings fixed: two if/else chains → switch, a `%q` formatting bug
  in the mermaid renderer, two unguarded `strings.Index` slices, 30+
  non-idiomatic testify forms, missing `t.Helper()`.
- Unit suites for baseline (0→92%), code renderer (24→92%), spec decoder
  (37→87%) — the standing coverage rule.
- init flag DX: `init --help` no longer scaffolds; value-less
  `--recipe`/`-p` fail fast.
- Baseline-mode DX: `--baseline-update` without `--baseline` is a config
  error; a broken baseline no longer renders "OK" on stdout.

## [2.1.0] - 2026-08-18
v1.0 roadmap block complete: stabilization, docs, CI & release hygiene,
polish. 84 commits since v2.0.0.


### Changed
- `go-arch-lint init` now scaffolds the spec and the runner as separate
  files: `arch.go` (user-editable spec) + `main.go` (stable runner with
  flag passthrough). The runner can be regenerated or upgraded without
  touching the architecture description, and vice versa — guarded by
  `TestScaffoldSplit_RegenerateRunnerKeepsSpec`. The launcher's
  spec-did-not-build footer now points at `arch.go` (the file users
  edit); usage, README RU/EN, and docs/delegation.md updated to match.
  The migration guide already documented this layout — the scaffold now
  matches the docs instead of contradicting them.

### Added
- Official GitHub Actions action (`action.yml`, composite) with inline PR
  annotations: installs the released binary, runs
  `check --format github-actions`, propagates the linter exit codes
  (roadmap #3). The new `github-actions` format renders one workflow
  command per violation — `::error` for blocking kinds, `::notice` for the
  advisory unmatched-file kind — with `file`/`line`/`col`/`title`
  properties and percent-encoded reserved characters. CI gained an
  `action smoke` job dogfooding the action on this repo with a locally
  built binary (`install: false`).

### Fixed
- Global flag consistency (roadmap #3): `--output-color=false` (the
  cobra-style spelling the delegated layer documents) was silently ignored
  on the scaffold path — only `--no-colors` worked. `OptionsFromFlags` now
  honors both spellings (`--output-color=false|true`, `--output-color
  false`, bare `--output-color`, and `--no-colors`), verified end-to-end
  through the delegation launcher on a real fixture. Launcher `help`
  output updated to the actual flag surface (added `junit` to `--format`,
  `--max-warnings`, `-p`, both color spellings, `--output-json-one-line`);
  README RU/EN global-flags lines updated to match.
- Spec-validator messages: corrected typos and casing that reached users on
  every malformed spec — "at least one component should by defined" →
  "must be defined", "dublicated" → "duplicated" (components and vendors),
  "failed to resolv path" → "resolve", "miss configuration" →
  "misconfiguration", `AnyVendorDeps=true` → `anyVendorDeps=true` (casing
  now matches the DSL flag it names).

### Added
- Validator test suite (`internal/services/spec/validator`): 14 tests / 17
  cases covering every validator through the real Document implementation
  (dsl.SpecBuilder → GoSpecDocument) — version range, empty components,
  unresolvable globs (+ `ignoreNotFoundComponents` suppression), unknown and
  duplicated components/vendors in dep rules, `anyProjectDeps`/
  `anyVendorDeps` conflicts with non-empty lists, empty dep rules, common
  components/vendors, invalid `excludeFiles` regexps, and workdir
  existence. Every notice is asserted against its source reference
  (file/line), pinning the "source-referenced errors" contract.
- `//go-arch-lint:ignore` suppression directives for incremental adoption
  on legacy codebases (roadmap #3): annotate a known violation in source
  (on the line or the line above) and the check passes while new
  violations still fail it. Optional arguments filter by dependency
  target (component name or the last import-path segment);
  `//go-arch-lint:ignore-file` suppresses the whole file. Suppressed
  violations are counted and surfaced (`suppressed: N` footer in text
  output, `SuppressedCount` in JSON) so debt stays visible. Works
  uniformly across all output formats and the programmatic API.
- `--format sarif`: check results as a SARIF 2.1.0 log for GitHub Code
  Scanning and other code-scanning tools — rule IDs (GA001–GA004),
  severity levels, relative file URIs and line/column regions;
  `tool.driver.version` reports the build version (roadmap #3).
- CI workflow with GitHub Actions (build, test, lint).
- Self-lint job in CI: `go run ./cmd/arch-lint check` verifies the project's
  own architecture on every push and PR.
- Community health files: `CODE_OF_CONDUCT.md`, `SECURITY.md`, issue and PR templates.
- PlantUML and Mermaid graph output formats.
- Programmatic entry points `archlint.Run` / `archlint.MustRun` with `CheckOptions`.
- `SpecDef` type and `MergeSpecs` for multi-spec support.
- Examples directory with architecture pattern demos.
- Initial `CHANGELOG.md`.
- Delegation model audit: `docs/delegation.md` documents flag routing,
  exit-code survival through `go run`, cache costs (cold ~45 s / steady ~2 s),
  and every system failure mode.

### Changed
- Module renamed to fork `github.com/vsfedorenko/go-arch-lint`.
- DSL spec builder and context handling simplified.
- Scaffold now generates a single `main.go` using `Spec(...)` + `MustRun(spec)`.
- Project config migrated from YAML to Go DSL.
- Container path flattened and shared app bootstrap extracted.
- Replaced `sort.Slice` with the stdlib `slices` package.

### Fixed
- DeepScan concurrency: `workersCount()` now scales with `runtime.NumCPU()`
  (capped at 8) instead of hard-coding a single worker. Result appends are
  protected by a dedicated mutex so concurrent component scanning is safe.
- Scanner no longer descends into excluded directories (fixes permission-denied
  errors on root-owned local volumes).
- Broken import path in `scanner_test.go` (`fe3dback` → `vsfedorenko`).
- Docker image links and goreleaser main package path.
- Various `golangci-lint` warnings resolved.

## [1.0.0] - 2026-07-09

### Added
- Go DSL configuration format with single-file scaffold.
- Graph rendering (d2) and code highlighting (chroma).
- Integration test suite.
