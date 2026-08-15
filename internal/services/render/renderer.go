package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

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

func newRenderer(
	colorPrinter colorPrinter,
	referenceRender referenceRender,
	outputType models.OutputType,
	outputJSONOneLine bool,
	format models.Format,
	asciiTemplates map[string]string,
	out io.Writer,
) *Renderer {
	return &Renderer{
		colorPrinter:      colorPrinter,
		referenceRender:   referenceRender,
		outputType:        outputType,
		outputJSONOneLine: outputJSONOneLine,
		format:            format,
		asciiTemplates:    asciiTemplates,
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
			fmt.Printf("ERR: %s\n", err.Error())
			fmt.Printf("------------\n")
			fmt.Printf("%s\n", codePreview)
		}

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
