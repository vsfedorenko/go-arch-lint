package render

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models"
)

// driverVersionDefault is reported as tool.driver.version in SARIF output
// when no explicit build version was injected (local `go run` builds).
const driverVersionDefault = "dev"

// violationKindMatch is the advisory Violation.Type value (file matched no
// component): those annotate as ::notice instead of ::error in
// github-actions format. Kept in sync with models.ToViolations.
const violationKindMatch = "match"

const (
	fnColorize   = "colorize"
	fnTrimPrefix = "trimPrefix"
	fnTrimSuffix = "trimSuffix"
	fnTrimDef    = "def"
	fnPadLeft    = "padLeft"
	fnPadRight   = "padRight"
	fnLinePrefix = "linePrefix"
	fnDir        = "dir"
	fnPlus       = "plus"
	fnConcat     = "concat"
)

type (
	Renderer struct {
		colorPrinter      colorPrinter
		referenceRender   referenceRender
		outputType        models.OutputType
		outputJSONOneLine bool
		format            models.Format
		asciiTemplates    map[string]string
		// driverVersion is reported as tool.driver.version in SARIF
		// output. Defaults to "dev" (unknown build).
		driverVersion string
		// out receives rendered output. Defaults to os.Stdout; injected in
		// tests so renderers become pure and pipeable without swapping the
		// process-wide os.Stdout.
		out io.Writer
	}
)

// NewRenderer constructs a Renderer writing to os.Stdout.
func NewRenderer(
	colorPrinter colorPrinter,
	referenceRender referenceRender,
	outputType models.OutputType,
	outputJSONOneLine bool,
	format models.Format,
	asciiTemplates map[string]string,
) *Renderer {
	return newRenderer(colorPrinter, referenceRender, outputType, outputJSONOneLine, format, asciiTemplates, os.Stdout)
}

// NewRendererTo is NewRenderer with an explicit output writer (tests,
// programmatic API embedding).
func NewRendererTo(
	out io.Writer,
	colorPrinter colorPrinter,
	referenceRender referenceRender,
	outputType models.OutputType,
	outputJSONOneLine bool,
	format models.Format,
	asciiTemplates map[string]string,
) *Renderer {
	return newRenderer(colorPrinter, referenceRender, outputType, outputJSONOneLine, format, asciiTemplates, out)
}

// WithDriverVersion sets the tool version reported in SARIF output
// (tool.driver.version). Call after construction, before RenderModel.
func (r *Renderer) WithDriverVersion(version string) *Renderer {
	r.driverVersion = version
	return r
}

func newRenderer(
	colorPrinter colorPrinter,
	referenceRender referenceRender,
	outputType models.OutputType,
	outputJSONOneLine bool,
	format models.Format,
	asciiTemplates map[string]string,
	out io.Writer,
) *Renderer {
	version := driverVersionDefault
	return &Renderer{
		colorPrinter:      colorPrinter,
		referenceRender:   referenceRender,
		outputType:        outputType,
		outputJSONOneLine: outputJSONOneLine,
		format:            format,
		asciiTemplates:    asciiTemplates,
		driverVersion:     version,
		out:               out,
	}
}

// emit writes one rendered document (ASCII buffer or JSON blob) to the
// renderer's output writer, newline-terminated.
func (r *Renderer) emit(doc string) {
	fmt.Fprintln(r.out, doc)
}

// marshalJSON serialises v compactly (one-line mode) or indented.
func (r *Renderer) marshalJSON(v interface{}) ([]byte, error) {
	if r.outputJSONOneLine {
		return json.Marshal(v)
	}
	return json.MarshalIndent(v, "", "  ")
}

func (r *Renderer) RenderModel(model interface{}, err error) error {
	userSpace := models.IsUserSpaceError(err)
	configErr := models.IsConfigError(err)
	if err != nil && !userSpace && !configErr {
		var referableErr models.ReferableError
		if errors.As(err, &referableErr) {
			codePreview := r.referenceRender.SourceCode(referableErr.Reference().ExtendRange(1, 1), true, true)
			r.emit("ERR: " + err.Error())
			r.emit("------------")
			r.emit(string(codePreview))
		}

		return err
	}

	// A config error means the check did NOT run. Rendering the (empty)
	// check model would print "OK - No warnings found" (ASCII) or an
	// empty array (json) — actively misleading on a failed run. Formats
	// with dedicated config-error branches (github-actions, html, sarif)
	// keep them; everything else gets the plain error text.
	if configErr {
		switch r.format {
		case models.FormatGitHubActions:
			if renderErr := r.renderGitHubActions(model, err); renderErr != nil {
				return fmt.Errorf("failed to render model: %w", renderErr)
			}
			return err
		case models.FormatHTML:
			if renderErr := r.renderHTML(model, err); renderErr != nil {
				return fmt.Errorf("failed to render model: %w", renderErr)
			}
			return err
		case models.FormatSARIF:
			if renderErr := r.renderSARIF(model); renderErr != nil {
				return fmt.Errorf("failed to render model: %w", renderErr)
			}
			return err
		}

		// The check model may carry document notices (spec validation
		// details like "not found directories for 'x'"); surface them —
		// they are the actionable part of a config error.
		if checkOut, ok := model.(models.CmdCheckOut); ok {
			for _, notice := range checkOut.DocumentNotices {
				r.emit(fmt.Sprintf("Error: %s", notice.Text))
			}
		}
		r.emit(models.CmdErrorOut{Error: err.Error()}.Error)
		return err
	}

	// Fast path: --format json renders check results as a flat JSON array of
	// violations (one object per violation), which is easier for CI pipelines
	// and editor integrations to consume than the wrapped {Type, Payload} model.
	if r.format == models.FormatJSON {
		if renderErr := r.renderViolationsJSON(model); renderErr != nil {
			return fmt.Errorf("failed to render model: %w", renderErr)
		}
		return err
	}

	// Fast path: --format sarif renders check results as a SARIF 2.1.0 log
	// for GitHub Code Scanning / code-scanning tools.
	if r.format == models.FormatSARIF {
		if renderErr := r.renderSARIF(model); renderErr != nil {
			return fmt.Errorf("failed to render model: %w", renderErr)
		}
		return err
	}

	// Fast path: --format junit renders check results as a JUnit-style XML
	// report for CI test dashboards (GitLab/Jenkins/Buildkite).
	if r.format == models.FormatJUnit {
		if renderErr := r.renderJUnit(model); renderErr != nil {
			return fmt.Errorf("failed to render model: %w", renderErr)
		}
		return err
	}

	// Fast path: --format github-actions renders check results as GitHub
	// Actions workflow commands (::error/::notice annotations).
	if r.format == models.FormatGitHubActions {
		if renderErr := r.renderGitHubActions(model, err); renderErr != nil {
			return fmt.Errorf("failed to render model: %w", renderErr)
		}
		return err
	}

	// Fast path: --format html renders check results as a standalone HTML
	// report for humans and CI artifact archives.
	if r.format == models.FormatHTML {
		if renderErr := r.renderHTML(model, err); renderErr != nil {
			return fmt.Errorf("failed to render model: %w", renderErr)
		}
		return err
	}

	var renderErr error

	switch r.outputType {
	case models.OutputTypeJSON:
		renderErr = r.renderJSON(model)
	case models.OutputTypeASCII:
		renderErr = r.renderASCII(model)
	default:
		panic(fmt.Sprintf("failed to render: unknown output type: %s", r.outputType))
	}

	if renderErr != nil {
		return fmt.Errorf("failed to render model: %w", renderErr)
	}

	return err
}

func (r *Renderer) renderASCII(model interface{}) error {
	templateName := fmt.Sprintf("%T", model)
	templateBuffer, exist := r.asciiTemplates[templateName]

	if !exist {
		return fmt.Errorf("ascii template for model '%s' not exist", templateName)
	}

	tpl, err := template.
		New(templateName).
		Funcs(map[string]interface{}{
			fnColorize:   r.asciiColorize,
			fnTrimPrefix: r.asciiTrimPrefix,
			fnTrimSuffix: r.asciiTrimSuffix,
			fnTrimDef:    r.asciiDefaultValue,
			fnPadLeft:    r.asciiPadLeft,
			fnPadRight:   r.asciiPadRight,
			fnLinePrefix: r.asciiLinePrefix,
			fnDir:        r.asciiPathDirectory,
			fnPlus:       r.asciiPlus,
			fnConcat:     r.asciiConcat,
		}).
		Parse(
			preprocessRawASCIITemplate(templateBuffer),
		)
	if err != nil {
		return fmt.Errorf("failed to render ascii view '%s': %w", templateName, err)
	}

	var buffer bytes.Buffer
	err = tpl.Execute(&buffer, model)
	if err != nil {
		return fmt.Errorf("failed to execute template '%s': %w", templateName, err)
	}

	r.emit(buffer.String())
	return nil
}

func (r *Renderer) renderJSON(model interface{}) error {
	modelType, err := r.extractModelType(model)
	if err != nil {
		return fmt.Errorf("failed extract model type from '%T' (maybe not matched pattern: 'CmdXXXOut') : %w", model, err)
	}

	wrapperModel := struct {
		Type    string      `json:"Type"`
		Payload interface{} `json:"Payload"`
	}{
		Type:    modelType,
		Payload: model,
	}

	jsonBuffer, marshalErr := r.marshalJSON(wrapperModel)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal payload '%v' to json: %w", model, marshalErr)
	}

	r.emit(string(jsonBuffer))
	return nil
}

// renderViolationsJSON renders check results as a flat JSON array of Violation
// objects (the --format json output). Non-check models fall back to the
// generic wrapped JSON so that `--format json` is still safe on other commands.
func (r *Renderer) renderViolationsJSON(model interface{}) error {
	checkOut, ok := model.(models.CmdCheckOut)
	if !ok {
		return r.renderJSON(model)
	}

	violations := checkOut.ToViolations()

	jsonBuffer, marshalErr := r.marshalJSON(violations)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal violations to json: %w", marshalErr)
	}

	r.emit(string(jsonBuffer))
	return nil
}

// renderSARIF renders check results as a SARIF 2.1.0 log (the --format
// sarif output). Non-check models fall back to the generic wrapped JSON
// so that `--format sarif` is still safe on other commands.
func (r *Renderer) renderSARIF(model interface{}) error {
	checkOut, ok := model.(models.CmdCheckOut)
	if !ok {
		return r.renderJSON(model)
	}

	version := r.driverVersion
	if version == "" {
		version = driverVersionDefault
	}

	sarif := checkOut.ToSARIF(version)

	jsonBuffer, marshalErr := r.marshalJSON(sarif)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal SARIF to json: %w", marshalErr)
	}

	r.emit(string(jsonBuffer))
	return nil
}

// renderJUnit renders check results as a JUnit-style XML report (the
// --format junit output). Non-check models fall back to the generic
// wrapped JSON so that `--format junit` is still safe on other commands.
func (r *Renderer) renderJUnit(model interface{}) error {
	checkOut, ok := model.(models.CmdCheckOut)
	if !ok {
		return r.renderJSON(model)
	}

	report := checkOut.ToJUnitXML()

	xmlBuffer, marshalErr := xml.MarshalIndent(report, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal JUnit report to xml: %w", marshalErr)
	}

	var doc strings.Builder
	doc.WriteString(xml.Header)
	doc.Write(xmlBuffer)
	doc.WriteByte('\n')
	r.emit(doc.String())

	return nil
}

// renderGitHubActions renders check results as GitHub Actions workflow
// commands (the --format github-actions output): one `::error` annotation
// per blocking violation and one `::notice` per advisory, each pointing at
// the offending file and line so violations show up inline on the PR diff.
// A config error gets a single `::error` instead — a "no violations"
// notice would wrongly read as a green check when nothing was linted.
// Non-check models fall back to the generic wrapped JSON so the flag stays
// safe on other commands.
func (r *Renderer) renderGitHubActions(model interface{}, err error) error {
	checkOut, ok := model.(models.CmdCheckOut)
	if !ok {
		return r.renderJSON(model)
	}

	if models.IsConfigError(err) {
		r.emit("::error title=go-arch-lint::configuration error — the check did not run: " +
			githubActionsEscape(err.Error()))
		return nil
	}

	violations := checkOut.ToViolations()
	for _, v := range violations {
		r.emit(githubActionsCommand(v))
	}

	if len(violations) == 0 {
		r.emit("::notice ::go-arch-lint: no architecture violations found")
	}

	return nil
}

// githubActionsCommand renders one violation as a workflow command:
//
//	::error file=internal/handler/user.go,line=10,col=2,title=go-arch-lint (component: handler)::component "handler" may not depend on "…"
//
// Property values are escaped per the workflow-command spec (percent-encode
// reserved characters); the message keeps newlines legal via %0A.
func githubActionsCommand(v models.Violation) string {
	var b strings.Builder

	// Advisory kinds annotate as notices; everything that fails the check
	// (dependency, deepscan, naming) is an error annotation. Mirrors
	// models.Violation.Type == "match" (unmatched file), which is advisory.
	command := "error"
	if v.Type == violationKindMatch {
		command = "notice"
	}

	b.WriteString("::")
	b.WriteString(command)

	b.WriteString(" file=")
	b.WriteString(githubActionsEscape(models.RelativeFilePath(v.File)))
	if v.Line > 0 {
		b.WriteString(",line=")
		b.WriteString(strconv.Itoa(v.Line))
		if v.Column > 0 {
			b.WriteString(",col=")
			b.WriteString(strconv.Itoa(v.Column))
		}
	}
	b.WriteString(",title=go-arch-lint")
	if v.Component != "" {
		b.WriteString(" ")
		b.WriteString(githubActionsEscape(v.Component))
	}

	b.WriteString("::")
	b.WriteString(githubActionsEscape(v.Rule))

	return b.String()
}

// githubActionsEscape percent-encodes the characters the workflow-command
// grammar reserves in property values and messages: `:` and `,` separate
// properties, `=` separates keys from values, and `%` is the escape prefix.
// Newlines are encoded as %0A (a raw newline would terminate the command).
func githubActionsEscape(s string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
		":", "%3A",
		",", "%2C",
		"=", "%3D",
	)
	return replacer.Replace(s)
}

// Rename "anypackage.CmdXXXXOut" to "models.XXXX"
// for back compatible with previous response version
func (r *Renderer) extractModelType(model any) (string, error) {
	const expectedPrefix = "Cmd"
	const expectedSuffix = "Out"

	alias := fmt.Sprintf("%T", model)
	dotIndex := strings.Index(alias, ".")

	if dotIndex == -1 {
		return "", fmt.Errorf("DTO type '%s' without package name", alias)
	}

	dtoName := alias[dotIndex+1:]

	if !strings.HasPrefix(dtoName, expectedPrefix) {
		return "", fmt.Errorf("DTO name '%s' alias '%s' should has prefix '%s'", dtoName, alias, expectedPrefix)
	}

	if !strings.HasSuffix(dtoName, expectedSuffix) {
		return "", fmt.Errorf("DTO name '%s' alias '%s' should has suffix '%s'", dtoName, alias, expectedSuffix)
	}

	return fmt.Sprintf(
		"models.%s",
		strings.TrimPrefix(strings.TrimSuffix(dtoName, expectedSuffix), expectedPrefix),
	), nil
}
