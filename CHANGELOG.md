# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Unit suite for the spec decoder (`internal/services/spec/decoder`):
  coverage 37.3% → 86.6%. The new tests build the SpecBuilder through the
  REAL DSL functions (same path as a user's arch.go) and decode it back:
  every block round-trips — options, excludes, vendors, common lists,
  tiers, deps, interface placement, naming, visibility. Pins the
  documented defaults (global DeepScan=true, per-component DeepScan=false,
  IgnoreNotFoundComponents=false) and the defensive-copy contract of
  Tiers().

### Added
- Unit suite for the code-snippet renderer
  (`internal/services/render/code`): coverage 24.3% → 91.9%. Pins the
  rendering contracts: empty output for invalid references and missing
  files, exact range extraction with line numbers, a single pointer
  marker on the violation line, column caret, tab expansion, clamping
  of out-of-range references, and chroma highlighting (ANSI-safe
  assertion — the highlighter interleaves escapes inside tokens).

### Added
- Unit suite for the baseline service (`internal/services/baseline`):
  14 tests, coverage 0% → 92.1%. Pins the contracts the e2e tests exercise
  only indirectly: fingerprint stability, per-kind constructors, sorted
  deterministic output, save/load round-trip (incl. missing parent dir and
  bare filenames), human-readable annotations for every violation kind,
  missing/broken/unreadable/future-scheme baseline files, known-vs-new
  comparison, and result filtering with SuppressedCount passthrough.

### Documentation
- Refreshed and de-bureaucratized the docs to match the code:
  - README RU/EN scaffold example updated for the arch.go/main.go split
    (the old example still showed `func main()` inside arch.go and a
    bare `MustRun(spec)` without flag passthrough)
  - Exit-code tables now include `2` — "the check did not run" — with a
    one-line explanation, in both languages
  - migration-v2.md: plain-language rewrite of the intro and
    troubleshooting; the "after" example completed with the generated
    runner note
  - docs/README.md index now lists all five documents (cookbook,
    json-schema, delegation were missing)
  - cookbook/syntax examples migrated from the legacy `var _ = Spec`
    idiom to the scaffold's `var spec = Spec` (both work; docs now match
    what `init` writes)
  - json-schema.md retitled: the page covers five formats + baseline +
    the GitHub Action, not just JSON
  - dogfood `.go-arch-lint/` itself split into arch.go + main.go — the
    repo's own spec now matches the layout `init` scaffolds (the
    syntax docs link to `.go-arch-lint/arch.go` was broken until now)
  - all internal doc links verified (0 broken across 15 files)

### Fixed
- `init` flag DX (found by synthetic probing of the fresh --recipe
  feature): `init --help` scaffolded a project instead of showing help;
  `--recipe` (and `-p`/`--project-path`) without a value silently fell
  back to the DEFAULT spec — a user who typo'd their recipe invocation
  got a generic scaffold while believing they had chosen a pattern.
  Now: `init --help` prints usage (recipes listed, nothing written),
  value-less flags fail fast with exit 1 naming the flag.

### Fixed
- Output flags are honored on the scaffold (delegated) path — found by
  probing `check --output-type=json` against the real binary: the flag
  was silently dropped and the check kept printing ASCII. The scaffolded
  runner (`archlint.OptionsFromFlags`) now parses `--output-type`,
  `--json` (alias), and `--output-json-one-line`, with two fail-fast
  validations instead of silent no-ops (the confusion class reported
  against the CLI upstream, fe3dback/go-arch-lint#62): unknown
  `--output-type` values are config errors echoing the bad value, and
  `--output-json-one-line` without json output (`--output-type=json`,
  `--json`, or `--format json`) is rejected with the fix spelled out.
  New public API options `WithOutputType` / `WithOutputJSONOneLine`
  expose the same control to library users.
- The launcher's "arch spec did not build" footer is now printed only
  when the delegated process actually failed to build (no `exit status N`
  in `go run`'s stderr). Previously ANY exit 2 — including config errors
  the renderer had already explained (e.g. the baseline and one-line
  validations above) — was followed by the misleading footer blaming a
  spec that compiled fine.
- Baseline-mode DX (found by synthetic probing of the fresh --baseline
  feature): `--baseline-update` without `--baseline` silently recorded
  nothing while ordinary violations still failed the run — now a config
  error naming the missing flag (`--baseline-update requires --baseline
  <file> ...`, exit 2). And a broken or missing baseline file rendered
  the empty check model — stdout said `OK - No warnings found` (or `[]`
  for json) while the process exited 2: config errors now surface the
  real diagnostic in every format (github-actions/html/sarif keep their
  dedicated banners; others print the error text plus any document
  notices such as "not found directories"), and `MustRun` prints
  pre-renderer errors to stderr so no failure path is silent anymore.

### Added
- `go-arch-lint init --recipe clean|hexagonal|ddd`: scaffold `arch.go`
  pre-filled for a known architecture pattern instead of a blank page
  (clean: domain <- usecase <- delivery; hexagonal: ports & adapters;
  ddd: bounded contexts). Every recipe sets
  `IgnoreNotFoundComponents(true)` so layer directories can be created
  gradually. Unknown recipe names fail with the recipe list, exit 1.
- Baseline / incremental adoption mode: `--baseline <file>` +
  `--baseline-update`. Record existing violations into a committed
  baseline file (`go-arch-lint check --baseline
  .go-arch-lint/baseline.json --baseline-update`), then CI compares
  against it — known debt is tolerated, only NEW violations fail the
  build (exit 1). Fingerprints (`kind|rule|file`) exclude line numbers,
  so edits above a violation never resurrect it as new. A missing
  baseline in compare mode is a config error (exit 2), never a silent
  pass. Programmatic API: `archlint.WithBaseline(path)` /
  `archlint.WithBaselineUpdate()`. Text summary prints
  `baseline: N new, M known (tolerated)`; JSON carries
  `BaselineNewCount` / `BaselineKnownCount`.

### Fixed
- Segmentation fault in the DSL: calling any builder function AFTER a
  nested `Spec()` callback crashed with a raw nil-pointer dereference
  (the nested `Spec` resets the package-level builder context on exit;
  subsequent builders dereferenced the nil `*SpecBuilder`). Found by
  synthetic DSL probing. Every builder now guards with an actionable
  panic: "this DSL function must be called inside Spec(func(){...}) —
  the context was reset (a nested Spec() finished)". Normal nested
  callbacks (Deps/Allow/Naming/...) are unaffected — pinned by tests.

### Added
- Weird-path format integration tests: a project with spaces and `::`
  in directory names (legal on unix, hostile to URI encoders,
  workflow-command parsers, and XML escapers) run through all three
  machine formats end-to-end. SARIF `artifactLocation.uri` must carry
  the raw path (it is a string, not URL-encoded), JUnit testcase names
  and failure messages must survive the XML round-trip, and the
  GitHub Actions `file=` property must percent-encode `::` (unescaped
  colons would break workflow-command parsing) while still decoding
  back to the real path. Found by hand-probing; all formats passed —
  now pinned so a renderer refactor cannot regress it.

### Added
- Documentation for the `--max-warnings` display cap (found by the
  540-file synthetic stress run): every output format is capped at the
  limit (default 512); text marks truncation (`omitted: N`) while the
  machine formats (JSON/SARIF/JUnit) truncate silently — pipelines that
  count array elements under-report on heavily-violating projects. The
  exit code reflects the FULL violation count either way. Documented in
  docs/json-schema.md (new "Output cap" section) and the README RU/EN
  global-flags lines; pinned by tests that the cap never flips
  pass/fail semantics (`WithMaxWarnings(1)` on a violating project
  still maps to exit code 1).

### Fixed
- Equals-form CLI flags (`--format=json`, `--project-path=/x`,
  `--max-warnings=0`) were silently dropped by the scaffold flag parser —
  the same class as the `--output-color=false` bug fixed earlier. Found
  by synthetic flag probing: a CI pipeline writing the cobra-style
  spelling got text output from a "json" invocation and linted the wrong
  directory from a "project-path" one. `stringFlag` now recognizes both
  `--name value` and `--name=value`; first occurrence wins.

### Added
- CRLF line-ending tests for the suppress directive scanner (Windows
  checkouts must not glue `\r` into directive arguments).
- Synthetic hostile-input suite for `OptionsFromFlags` (empty values,
  value-less trailing flags, overflow numbers, duplicate and conflicting
  flags, `--` terminator): no panics, sane ignore semantics.

### Fixed
- Nil-pointer panic in project-info assembly when `go.mod` contains a
  syntax error or no `module` directive: `modfile.ParseLax` results are
  now nil-checked before dereferencing, producing a clear
  "should contain valid go.mod syntax and 'module' directive" error
  instead of a segmentation fault. Found by the new test suite.

### Added
- Test coverage for six previously untested packages (found by the
  coverage audit after v2.1.0):
  `internal/services/suppress` 0%→95% (14 tests: directive parsing edge
  cases — trailing vs stacked, argument filters, ignore-vs-ignore-file
  prefix separation, URL-after-comment non-matching, blank-line
  boundaries, missing files);
  `internal/services/checker` suppress filter + rule-target extraction
  59%→68% (12 tests: per-category suppression semantics, cycle/tier/
  visibility/placement target matching);
  `internal/services/common/path` 0%→86% (glob resolver: doublestar
  anchor inclusion and trailing-slash contracts pinned);
  `internal/services/common/ast` 0%→100%;
  `internal/services/render/printer` 0%→100% (colors on/off contract);
  `internal/services/project/info` 0%→87% (7 tests: relative roots,
  missing/broken/moduleless go.mod, URL arch-file rejection).

### Fixed
- `go-arch-lint version` now reports the real release build metadata
  (version, commit, build time via goreleaser ldflags) instead of the
  hard-coded `v2.0.0-dev` string. Local builds fall back to `dev`.

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
