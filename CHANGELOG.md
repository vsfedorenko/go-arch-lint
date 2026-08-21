# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
