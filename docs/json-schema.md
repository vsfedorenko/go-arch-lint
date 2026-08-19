# CI Output Formats: json, sarif, junit, github-actions, html

One page for everything a pipeline consumes: the five `check --format`
outputs, the baseline mode for incremental adoption, exit codes, and the
official GitHub Action.

`--format json` prints a **flat JSON array of violations** to stdout — one
object per violation, no wrapper, stable order (grouped by kind, sorted by
file then line).

```bash
go-arch-lint check --format json --project-path .
```

## Output cap (`--max-warnings`)

Every output format — text, JSON, SARIF, JUnit, GitHub Actions — is capped
at `--max-warnings` violations (**default 512**). The exit code reflects
the FULL count: `1` fires as soon as the project has any violation, even
when the displayed array is truncated.

- The **text** format marks the truncation explicitly (`omitted: N (too
  big to display)`).
- The **machine formats** (JSON/SARIF/JUnit) contain no truncation marker:
  a pipeline that counts array elements under-reports the violation count
  on heavily-violating projects. Raise the cap when the count matters:

```bash
go-arch-lint check --format json --max-warnings=100000 --project-path .
```

Programmatic consumers get the same behaviour through
`archlint.WithMaxWarnings(n)`.

## Baseline mode (`--baseline` / `--baseline-update`)

`--baseline <file>` switches the check to incremental adoption:
violations whose fingerprints are recorded in the baseline file are
tolerated as known debt, and the exit code (and the rendered output)
reflect only NEW violations. `--baseline-update` (together with
`--baseline`) records the current full violation set instead of
comparing.

```bash
# record (commit the file):
go-arch-lint check --baseline .go-arch-lint/baseline.json --baseline-update
# compare (CI):
go-arch-lint check --baseline .go-arch-lint/baseline.json
```

Contract:

- The baseline file is a JSON document:
  `{ "schemeVersion": 1, "fingerprints": { "<kind>|<rule>|<file>":
  "<annotation>" } }`. Fingerprints deliberately exclude line numbers:
  an edit that only shifts lines never turns known debt "new".
- The flat violation array contains ONLY the new violations; the
  `--max-warnings` cap applies to that filtered set.
- The **text** summary prints `baseline: N new, M known (tolerated)`.
- Programmatic API: `archlint.WithBaseline(path)` +
  `archlint.WithBaselineUpdate()`.
- A missing baseline file in compare mode is a **configuration error
  (exit 2)** — a missing baseline must never silently pass the check.
  A wrong `schemeVersion` is likewise a config error; re-record the
  baseline after upgrading the tool.

## Violation schema

| Field        | Type   | Always | Description                                                        |
|--------------|--------|--------|--------------------------------------------------------------------|
| `type`       | string | yes    | `dependency` \| `match` \| `deepscan` \| `naming`                  |
| `file`       | string | yes    | Project-relative path of the offending file                        |
| `line`       | number | yes    | 1-based line (`0` when unknown)                                    |
| `column`     | number | no     | 1-based column (`0`/absent when unknown)                           |
| `component`  | string | no     | Source component owning the file                                   |
| `dependency` | string | no     | Target component/import that was used                              |
| `package`    | string | no     | Resolved import path (dependency kind) or package name (naming)    |
| `rule`       | string | yes    | Human-readable description of the violated rule                    |
| `details`    | string | no     | Extra context (e.g. injection AST for deepscan)                    |

Example:

```json
[
  {
    "type": "dependency",
    "file": "/internal/alpha/a.go",
    "line": 3,
    "column": 8,
    "component": "alpha",
    "dependency": "example.com/app/internal/beta",
    "package": "example.com/app/internal/beta",
    "rule": "component \"alpha\" may not depend on \"example.com/app/internal/beta\""
  }
]
```

### Violation kinds

- `dependency` — a component imports something it may not (imports
  checker, cycles, tier rules): `component`, `dependency`, `package` set.
- `match` — a file matched no component: only `file` + `rule`.
- `deepscan` — constructor-injected dependency not allowed by the spec:
  `component`, `dependency` (type name), `details` (injection AST),
  injection file/line in `file`/`line`.
- `naming` — forbidden package name: `package` set, `rule` carries the
  banned name, package path and file count.

## GitHub Action with inline annotations

The official composite action renders violations as `::error` / `::notice`
workflow commands anchored to the offending file and line — they appear
inline on the PR diff and in the Checks summary, no JS glue required:

```yaml
name: arch
on: [pull_request]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: '1.25'
      - uses: vsfedorenko/go-arch-lint@main   # pin to a tag once released
```

Inputs: `version` (release to install, default `latest`), `project-path`
(default `.`), `max-warnings` (default `0` = tool default), `install`
(`false` skips the install when a `go-arch-lint` binary is already on
PATH — useful for smoke-testing a locally built one). The action requires
`go` on PATH (the check delegates to `go run .go-arch-lint/`, see
[delegation](delegation.md)) and a committed `.go-arch-lint/` scaffold
(create it once with `go-arch-lint init`).

Exit codes propagate unchanged: `0` green, `1` violations (annotated),
`2` configuration error (surfaced as a top-level `::error`).

Under the hood the action runs `check --format github-actions`:

```
::error file=internal/handler/user.go,line=10,col=2,title=go-arch-lint handler::component "handler" may not depend on "github.com/x/proj/internal/repository"
::notice file=internal/orphan/x.go,title=go-arch-lint::file is not attached to any component
```

Blocking kinds (dependency, deepscan, naming) annotate as `::error`;
the advisory "file matched no component" kind annotates as `::notice`.
Reserved characters in messages are percent-encoded per the
[workflow-command spec](https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions),
so one violation is always exactly one command.

## Manual recipe (no action)

If you prefer not to use the action, the same annotations can be produced
from the JSON format directly:

```yaml
name: arch
on: [pull_request]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: Install go-arch-lint
        run: go install github.com/vsfedorenko/go-arch-lint/v2/cmd/arch-lint@latest
      - name: Init scaffold (once; commit the .go-arch-lint/ dir)
        run: arch-lint init && cd .go-arch-lint && go mod tidy
      - name: Architecture check
        run: arch-lint check --format json --project-path . > arch-violations.json || true
      - name: Annotate PR
        if: hashFiles('arch-violations.json') != ''
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const violations = JSON.parse(fs.readFileSync('arch-violations.json', 'utf8'));
            for (const v of violations) {
              core.error(`${v.rule}`, {
                file: v.file.replace(/^\//, ''),
                startLine: v.line || 1,
              });
            }
            if (violations.length) core.setFailed(`${violations.length} architecture violations`);
```

The exit code is authoritative — `|| true` only keeps the annotation step
running; `setFailed` fails the job after annotating.

## GitLab CI

```yaml
arch-check:
  stage: test
  image: golang:1.25
  script:
    - go install github.com/vsfedorenko/go-arch-lint/v2/cmd/arch-lint@latest
    - arch-lint init && cd .go-arch-lint && go mod tidy && cd ..
    - |
      arch-lint check --format json --project-path . > arch-violations.json
      code=$?
      cat arch-violations.json
      if [ $code -eq 2 ]; then
        echo "go-arch-lint configuration error"; exit 2
      fi
      if [ $code -eq 1 ]; then
        echo "architecture violations found (see JSON above)"; exit 1
      fi
  artifacts:
    when: always
    paths:
      - arch-violations.json
```

## SARIF output for GitHub Code Scanning

`go-arch-lint check --format sarif` prints a [SARIF 2.1.0](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html)
log — the native ingestion format for GitHub Code Scanning, DefectDojo and
other code-scanning dashboards. The exit-code convention is unchanged
(`0`/`1`/`2`); only the stdout payload differs.

```bash
go-arch-lint check --format sarif --project-path . > arch-lint.sarif
```

Rule IDs are stable across releases:

| Rule ID  | Name           | Level | Violation kind                        |
|----------|----------------|-------|---------------------------------------|
| `GA001`  | `ArchDependency` | `error` | disallowed import / cycle / tier break |
| `GA002`  | `ArchMatch`      | `note`  | file attached to no component          |
| `GA003`  | `ArchDeepScan`   | `error` | disallowed constructor injection       |
| `GA004`  | `ArchNaming`     | `error` | forbidden package name                 |

Every result carries `ruleId`, `level`, a `message.text` (the same
human-readable rule text as `--format json`, plus component/dependency
context), and a `locations[0].physicalLocation` with a project-relative
`artifactLocation.uri` and a `region.startLine`/`startColumn` (line `0` is
clamped to `1` per the SARIF spec). `tool.driver.version` reports the
build version (`dev` for local `go run` builds).

Upload to GitHub Code Scanning:

```yaml
name: arch
on: [pull_request]
jobs:
  check:
    runs-on: ubuntu-latest
    permissions:
      security-events: write  # upload-sarif requirement
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: Install go-arch-lint
        run: go install github.com/vsfedorenko/go-arch-lint/v2/cmd/arch-lint@latest
      - name: Init scaffold (once; commit the .go-arch-lint/ dir)
        run: arch-lint init && cd .go-arch-lint && go mod tidy && cd ..
      - name: Architecture check (SARIF)
        run: arch-lint check --format sarif --project-path . > arch-lint.sarif || [ $? -eq 1 ]
      - uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: arch-lint.sarif
        if: always()
```

`|| [ $? -eq 1 ]` keeps the job alive when violations are found (exit 1)
so the SARIF still uploads; exit 2 (broken config) still fails the job.

Real output (fixture project, one dependency violation + three unattached
files):

```json
{
  "version": "2.1.0",
  "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "go-arch-lint",
          "version": "dev",
          "informationUri": "https://github.com/vsfedorenko/go-arch-lint",
          "rules": [ { "id": "GA001", "name": "ArchDependency", "default": { "level": "error" } } ]
        }
      },
      "results": [
        {
          "ruleId": "GA001",
          "level": "error",
          "message": {
            "text": "component \"c\" may not depend on \"github.com/x/proj/internal/b\" (component: c, dependency: github.com/x/proj/internal/b)"
          },
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": { "uri": "internal/c/c1.go" },
                "region": { "startLine": 3, "startColumn": 8 }
              }
            }
          ]
        }
      ]
    }
  ]
}
```


## JUnit output for CI test dashboards

`go-arch-lint check --format junit` prints a JUnit-style XML report — the
native ingestion format for CI test dashboards: GitLab CI
[`artifacts:reports:junit`](https://docs.gitlab.com/ee/ci/testing/unit_test_reports.html),
the Jenkins JUnit plugin, Buildkite test annotations, Azure Pipelines.
One violation = one failed `testcase`; a clean check emits a single green
`arch-check` testcase so dashboards that require at least one case keep
rendering. The exit-code convention is unchanged (`0`/`1`/`2`).

```bash
go-arch-lint check --format junit --project-path . > arch-lint-junit.xml
```

Field mapping:

| JUnit field        | Value                                                    |
|--------------------|----------------------------------------------------------|
| `testsuite/@name`  | `go-arch-lint check`                                     |
| `testcase/@classname` | `go-arch-lint.GA001`…`GA004` (rule IDs shared with SARIF) |
| `testcase/@name`   | project-relative `file:line` of the violation            |
| `failure/@message` | the same human-readable rule text as `--format json`     |
| `failure/@type`    | rule ID (`GA001`…`GA004`)                                |
| `failure` body     | rule text + `component:` / `dependency:` / deepscan AST context |

GitLab CI recipe:

```yaml
arch-check:
  script:
    - arch-lint check --format junit --project-path . > arch-lint-junit.xml || [ $? -eq 1 ]
  artifacts:
    when: always
    reports:
      junit: arch-lint-junit.xml
```

`|| [ $? -eq 1 ]` keeps the job alive when violations are found (exit 1)
so the report is still uploaded; exit 2 (broken config) still fails the job.

Real output (fixture project, one dependency violation + three unattached
files):

```xml
<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="go-arch-lint" tests="4" failures="4">
  <testsuite name="go-arch-lint check" tests="4" failures="4">
    <testcase classname="go-arch-lint.GA001" name="internal/c/c1.go:3">
      <failure message="component &#34;c&#34; may not depend on &#34;github.com/x/proj/internal/a&#34;" type="GA001">component &#34;c&#34; may not depend on &#34;github.com/x/proj/internal/a&#34;&#xA;component: c&#xA;dependency: github.com/x/proj/internal/a</failure>
    </testcase>
    <testcase classname="go-arch-lint.GA002" name="internal/c/not_covered/c1nc.go">
      <failure message="file is not attached to any component" type="GA002">file is not attached to any component</failure>
    </testcase>
  </testsuite>
</testsuites>
```

(Rows for `internal/d/not_covered.go` and `internal/not_covered/nc.go`
omitted for brevity — same shape as the `GA002` case above.)

## HTML report for humans and archives

`go-arch-lint check --format html` prints a standalone HTML document —
one file, no scripts, no external assets (inline CSS). CI pipelines
archive it as an artifact; humans open it directly in a browser. Content:

- header: tool name/version and the checked module;
- summary cards: total violations plus per-rule-class counts
  (Dependency / Not matched / DeepScan / Naming), and — when non-zero —
  omitted (display cap) and suppressed counts;
- violation table: one row per violation — rule tag, `file:line`,
  component, dependency, rule text (+ details for DeepScan injections);
- footer notes repeating the display-cap and suppression semantics.

Guarantees (same contract as the other machine formats):

- The `--max-warnings` display cap never changes pass/fail semantics:
  the exit code reflects the FULL violation count; omitted rows are
  announced in a card and a footer line (see
  [Output cap](#output-cap---max-warnings)).
- All dynamic values are context-escaped via `html/template` — file paths
  and package names containing `<`, `>`, `&`, quotes, spaces or `::`
  render as text, never markup (pinned by the weird-path integration
  test). The document contains no scripts.
- A configuration error renders an error banner document — never an
  empty "no violations" page that would read as green (exit code `2`).
- Non-check commands fall back to the wrapped JSON model, so the flag is
  safe everywhere.

Example (fragment):

```html
<!DOCTYPE html>
<html lang="en">
...
  <div class="card bad">
    <div class="n">4</div>
    <div class="l">violations</div>
  </div>
...
  <tr>
    <td><span class="tag dependency">Dependency</span></td>
    <td class="file">internal/c/c1.go:3</td>
    <td>c</td>
    <td class="file">github.com/x/proj/internal/a</td>
    <td>component "c" may not depend on "github.com/x/proj/internal/a"</td>
  </tr>
```

GitLab CI artifact recipe:

```yaml
arch-report:
  script: go-arch-lint check --format html > arch-report.html
  artifacts:
    paths: [arch-report.html]
    when: always   # archive the report even when the check fails
```

## Notes

- `--format json`, `--format sarif`, `--format junit`, `--format github-actions` and `--format html` affect only `check`; other commands
  keep the `--output-type` (`ascii`/`json`) wrapper format (all formats
  fall back to it for non-check models).
- The scaffold `main()` must pass through CLI flags — the modern scaffold
  (`go-arch-lint init`) calls `archlint.MustRun(spec,
  archlint.OptionsFromFlags(os.Args[1:])...)`. If your scaffold predates
  this, add `archlint.OptionsFromFlags(os.Args[1:])...` to `MustRun`.
- Exit code `2` means the check did not run. Fix the spec first — a broken
  config lints nothing, so `2` is never "clean".
