package render

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
)

// htmlReportTpl is the standalone HTML document template for --format
// html. It uses html/template, so every interpolated value is
// context-escaped automatically — file paths, package names and rule text
// containing <, >, &, quotes or spaces cannot break out of their element.
// The document is self-contained: inline CSS, no scripts, no external
// assets, so an archived CI artifact renders identically offline.
const htmlReportTpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.ToolName}} report — {{.ModuleName}}</title>
<style>
  :root { color-scheme: light dark; }
  body { font: 14px/1.5 -apple-system, "Segoe UI", Roboto, sans-serif;
         margin: 0; padding: 2rem; background: #f6f8fa; color: #1f2328; }
  @media (prefers-color-scheme: dark) {
    body { background: #0d1117; color: #e6edf3; }
    .card, .wrap { background: #161b22; }
    table, th, td { border-color: #30363d !important; }
    tr:nth-child(even) { background: #1c2128; }
  }
  .wrap { max-width: 72rem; margin: 0 auto; }
  header { display: flex; flex-wrap: wrap; gap: .5rem 2rem;
           align-items: baseline; margin-bottom: 1.25rem; }
  h1 { font-size: 1.25rem; margin: 0; }
  header p { margin: 0; color: #656d76; font-size: .85rem; }
  .cards { display: flex; flex-wrap: wrap; gap: .75rem; margin-bottom: 1.5rem; }
  .card { background: #fff; border: 1px solid #d0d7de; border-radius: 8px;
          padding: .75rem 1.25rem; min-width: 8rem; }
  .card .n { font-size: 1.5rem; font-weight: 600; }
  .card .l { font-size: .8rem; color: #656d76; }
  .card.bad .n { color: #cf222e; }
  .card.ok .n { color: #1a7f37; }
  table { border-collapse: collapse; width: 100%; background: #fff;
          border: 1px solid #d0d7de; border-radius: 8px; overflow: hidden; }
  th, td { text-align: left; padding: .45rem .75rem;
           border-bottom: 1px solid #d0d7de; vertical-align: top; }
  th { font-size: .8rem; text-transform: uppercase; letter-spacing: .03em;
       color: #656d76; background: #f6f8fa; }
  tr:nth-child(even) { background: #fbfcfd; }
  code, td.file { font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
                  font-size: .85rem; }
  .tag { display: inline-block; font-size: .75rem; padding: .1rem .5rem;
         border-radius: 999px; font-weight: 500; }
  .tag.dependency, .tag.deepscan, .tag.naming { background: #ffebe9; color: #cf222e; }
  .tag.match { background: #ddf4ff; color: #0969da; }
  .details { color: #656d76; font-size: .85rem; }
  .foot { margin-top: 1rem; color: #656d76; font-size: .85rem; }
</style>
</head>
<body>
<div class="wrap">
<header>
  <h1>{{.ToolName}} — architecture report</h1>
  <p>module <code>{{.ModuleName}}</code></p>
  <p>{{.ToolName}} {{.ToolVersion}}</p>
</header>

<div class="cards">
  <div class="card {{if .Rows}}bad{{else}}ok{{end}}">
    <div class="n">{{.Total}}</div>
    <div class="l">violations</div>
  </div>
  {{range .ByType}}
  <div class="card">
    <div class="n">{{.Count}}</div>
    <div class="l">{{.Label}}</div>
  </div>
  {{end}}
  {{if .OmittedCount}}
  <div class="card">
    <div class="n">{{.OmittedCount}}</div>
    <div class="l">omitted (display cap)</div>
  </div>
  {{end}}
  {{if .SuppressedCount}}
  <div class="card">
    <div class="n">{{.SuppressedCount}}</div>
    <div class="l">suppressed</div>
  </div>
  {{end}}
</div>

{{if .Rows}}
<table>
  <thead>
  <tr>
    <th>Rule</th><th>File</th><th>Component</th><th>Dependency</th><th>Details</th>
  </tr>
  </thead>
  <tbody>
  {{range .Rows}}
  <tr>
    <td><span class="tag {{.Type}}">{{.TypeLabel}}</span></td>
    <td class="file">{{.File}}{{if .Line}}:{{.Line}}{{end}}</td>
    <td>{{.Component}}</td>
    <td class="file">{{.Dependency}}</td>
    <td>{{.Rule}}{{if .Details}}<div class="details">{{.Details}}</div>{{end}}</td>
  </tr>
  {{end}}
  </tbody>
</table>
{{else}}
<p>No architecture violations found.</p>
{{end}}

{{if .OmittedCount}}<p class="foot">{{.OmittedCount}} violation(s) omitted from this table by the --max-warnings display cap; the exit code reflects the full count.</p>{{end}}
{{if .SuppressedCount}}<p class="foot">{{.SuppressedCount}} violation(s) suppressed by //go-arch-lint:ignore directives.</p>{{end}}
</div>
</body>
</html>
`

// htmlTpl is parsed once at package init; a parse failure is a programmer
// error and panics loudly instead of failing every check at runtime.
var htmlTpl = template.Must(template.New("html-report").Parse(htmlReportTpl))

// renderHTML renders check results as a standalone HTML document (the
// --format html output). Non-check models fall back to the generic
// wrapped JSON so the flag stays safe on other commands. Config errors
// render as an error-styled single-card document — an empty "no
// violations" report would wrongly read as green when nothing was linted.
func (r *Renderer) renderHTML(model interface{}, err error) error {
	checkOut, ok := model.(models.CmdCheckOut)
	if !ok {
		return r.renderJSON(model)
	}

	if models.IsConfigError(err) {
		r.emit(renderHTMLConfigError(err))
		return nil
	}

	report := checkOut.ToHTMLReport(r.htmlDriverVersion())

	var buf strings.Builder
	if execErr := htmlTpl.Execute(&buf, report); execErr != nil {
		return fmt.Errorf("failed to render html report: %w", execErr)
	}

	r.emit(buf.String())
	return nil
}

// renderHTMLConfigError renders a configuration error as a minimal HTML
// document with an error banner, so a failed check never looks green.
func renderHTMLConfigError(err error) string {
	return `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>go-arch-lint — configuration error</title>
<style>body{font:14px/1.5 -apple-system,"Segoe UI",Roboto,sans-serif;margin:0;padding:2rem;background:#f6f8fa;color:#1f2328}
.banner{background:#ffebe9;color:#cf222e;border:1px solid #cf222e;border-radius:8px;padding:1rem 1.5rem;max-width:72rem;margin:0 auto}
code{font-family:ui-monospace,Menlo,monospace;font-size:.85rem}</style></head>
<body><div class="banner"><strong>go-arch-lint: configuration error — the check did not run.</strong><br>
<code>` + template.HTMLEscapeString(err.Error()) + `</code></div></body>
</html>
`
}

// htmlDriverVersion returns the version reported in the report header,
// defaulting like SARIF output.
func (r *Renderer) htmlDriverVersion() string {
	if r.driverVersion == "" {
		return driverVersionDefault
	}
	return r.driverVersion
}
