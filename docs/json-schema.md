# JSON Output for CI Integrations

`go-arch-lint check --format json` prints a **flat JSON array of
violations** to stdout — one object per violation, no wrapper, stable
order (grouped by kind, sorted by file then line). The process exit code
follows the linter convention: `0` no violations, `1` violations found,
`2` configuration/system error (broken spec, unreadable project).

```bash
go-arch-lint check --format json --project-path .
```

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

## GitHub Actions

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
        run: go install github.com/vsfedorenko/go-arch-lint/cmd/arch-lint@latest
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
    - go install github.com/vsfedorenko/go-arch-lint/cmd/arch-lint@latest
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

## Notes

- `--format json` affects only `check`; other commands keep the
  `--output-type` (`ascii`/`json`) wrapper format.
- The scaffold `main()` must pass through CLI flags — the modern scaffold
  (`go-arch-lint init`) calls `archlint.MustRun(spec,
  archlint.OptionsFromFlags(os.Args[1:])...)`. If your scaffold predates
  this, add `archlint.OptionsFromFlags(os.Args[1:])...` to `MustRun`.
- Exit code `2` means the check did not run: fix the spec first (a broken
  config lints nothing — don't treat it as "clean").
