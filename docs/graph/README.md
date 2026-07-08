# Graph

Generate dependency graphs from your project architecture config.

## Usage

```
go-arch-lint graph [flags]

Flags:
      --arch-file string      arch file path (default ".go-arch-lint/main.go")
  -f, --format string         output format [svg,d2,plantuml,mermaid] (default "svg")
      --focus string          render only specified component and its dependencies
  -h, --help                  help for graph
  -r, --include-vendors       include vendor dependencies (from "canUse" block)?
      --out string            output file for SVG format (default "./go-arch-lint-graph.svg")
      --project-path string   path to project directory (default "./")
  -t, --type string           graph type [flow,di] (default "flow")
```

## Output formats

```bash
go-arch-lint graph                        # SVG file (default)
go-arch-lint graph --format=d2            # d2 source to stdout
go-arch-lint graph --format=plantuml      # PlantUML to stdout
go-arch-lint graph --format=mermaid       # Mermaid to stdout
```

| Format     | Description                                            |
|------------|--------------------------------------------------------|
| `svg`      | Rendered SVG image, written to `--out` file            |
| `d2`       | d2 source text for manual tweaking or custom rendering |
| `plantuml` | PlantUML component diagram text                        |
| `mermaid`  | Mermaid flowchart text for Markdown / GitHub / GitLab   |

Text formats (d2, plantuml, mermaid) output to stdout — redirect to a file if needed:

```bash
go-arch-lint graph --format=mermaid > graph.mmd
go-arch-lint graph --format=plantuml > graph.puml
```

## Simple FLOW graph

```bash
go-arch-lint graph
```

Shows dependency direction — which components depend on which.

![graph](../images/graph-flow-c.png)

### +focus

Focus displays a single component and all its recursive dependencies:

```bash
go-arch-lint graph --focus operations
```

![graph](../images/graph-flow-c-focus.png)

### +vendors

```bash
go-arch-lint graph --include-vendors
```

Adds vendor libraries to the graph.

![graph](../images/graph-flow-v.png)

## DI graph type

```bash
go-arch-lint graph --type di
```

DI graph is the reverse of flow — shows dependency injection direction.

![graph](../images/graph-di-c.png)
