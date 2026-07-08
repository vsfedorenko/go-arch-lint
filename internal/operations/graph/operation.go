package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/lib/textmeasure"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/models/arch"
)

// graphEdge represents a directed dependency between two components.
type graphEdge struct {
	from     string
	to       string
	isVendor bool
}

type Operation struct {
	specAssembler        specAssembler
	projectInfoAssembler projectInfoAssembler
}

func NewOperation(
	specAssembler specAssembler,
	projectInfoAssembler projectInfoAssembler,
) *Operation {
	return &Operation{
		specAssembler:        specAssembler,
		projectInfoAssembler: projectInfoAssembler,
	}
}

func (o *Operation) Behave(ctx context.Context, in models.CmdGraphIn) (models.CmdGraphOut, error) {
	projectInfo, err := o.projectInfoAssembler.ProjectInfo(in.ProjectPath, in.ArchFile)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed to assemble project info: %w", err)
	}

	spec, err := o.specAssembler.Assemble(projectInfo)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed to assemble spec: %w", err)
	}

	whiteList, err := o.populateGraphWhitelist(spec, in)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed build graph whitelist: %w", err)
	}

	edges := o.collectEdges(spec, in, whiteList)

	format := in.Format
	if format == "" {
		if in.ExportD2 {
			format = models.GraphFormatD2
		} else {
			format = models.GraphFormatSVG
		}
	}

	out := models.CmdGraphOut{
		ProjectDirectory: spec.RootDirectory.Value,
		ModuleName:       spec.ModuleName.Value,
		Format:           format,
		ExportD2:         in.ExportD2,
	}

	switch format {
	case models.GraphFormatSVG:
		d2Code := o.renderD2(edges, in)
		svg, err := o.compileGraph(ctx, d2Code)
		if err != nil {
			return models.CmdGraphOut{}, fmt.Errorf("failed to compile graph: %w", err)
		}

		outFile, err := filepath.Abs(in.OutFile)
		if err != nil {
			return models.CmdGraphOut{}, fmt.Errorf("failed get abs path from '%s': %w", in.OutFile, err)
		}

		if isFileShouldBeWritten(format, in.OutputType) {
			if err := os.WriteFile(outFile, svg, 0o600); err != nil {
				return models.CmdGraphOut{}, fmt.Errorf("failed write graph into '%s' file: %w", in.OutFile, err)
			}
		}

		out.OutFile = outFile
		out.D2Definitions = d2Code

	case models.GraphFormatD2:
		source := o.renderD2(edges, in)
		out.D2Definitions = source
		out.GraphSource = source
		out.IsTextOutput = true

	case models.GraphFormatPlantUML:
		out.GraphSource = o.renderPlantUML(edges, in)
		out.IsTextOutput = true

	case models.GraphFormatMermaid:
		out.GraphSource = o.renderMermaid(edges, in)
		out.IsTextOutput = true

	default:
		return models.CmdGraphOut{}, fmt.Errorf("unknown graph format: %s", format)
	}

	return out, nil
}

func isFileShouldBeWritten(format models.GraphFormat, outputType models.OutputType) bool {
	if outputType == models.OutputTypeJSON {
		return false
	}

	return format == models.GraphFormatSVG
}

func (o *Operation) collectEdges(spec arch.Spec, opts models.CmdGraphIn, whiteList map[string]struct{}) []graphEdge {
	edges := make([]graphEdge, 0, 64)

	for _, cmp := range spec.Components {
		if _, visible := whiteList[cmp.Name.Value]; !visible {
			continue
		}

		for _, dep := range cmp.MayDependOn {
			if _, visible := whiteList[dep.Value]; !visible {
				continue
			}
			edges = append(edges, graphEdge{from: cmp.Name.Value, to: dep.Value})
		}

		if opts.IncludeVendors {
			for _, vnd := range cmp.CanUse {
				edges = append(edges, graphEdge{from: cmp.Name.Value, to: vnd.Value, isVendor: true})
			}
		}
	}

	return edges
}

func (o *Operation) populateGraphWhitelist(spec arch.Spec, opts models.CmdGraphIn) (map[string]struct{}, error) {
	if opts.Focus == "" {
		return o.populateGraphWhitelistAll(spec)
	}

	return o.populateGraphWhitelistFocused(spec, opts.Focus)
}

func (o *Operation) populateGraphWhitelistAll(spec arch.Spec) (map[string]struct{}, error) {
	whiteList := make(map[string]struct{}, len(spec.Components))

	for _, cmp := range spec.Components {
		whiteList[cmp.Name.Value] = struct{}{}
	}

	return whiteList, nil
}

func (o *Operation) populateGraphWhitelistFocused(spec arch.Spec, focusCmpName string) (map[string]struct{}, error) {
	cmpMap := make(map[string]arch.Component)
	rootExist := false

	for _, cmp := range spec.Components {
		cmpMap[cmp.Name.Value] = cmp

		if focusCmpName == cmp.Name.Value {
			rootExist = true
		}
	}

	if !rootExist {
		return nil, fmt.Errorf("focused cmp %s is not defined", focusCmpName)
	}

	whiteList := make(map[string]struct{}, len(spec.Components))
	resolved := make(map[string]struct{}, 64)
	resolveList := make([]string, 0, 64)
	resolveList = append(resolveList, focusCmpName)

	for len(resolveList) > 0 {
		cmp := cmpMap[resolveList[0]]
		resolveList = resolveList[1:]

		if _, alreadyResolved := resolved[cmp.Name.Value]; alreadyResolved {
			continue
		}

		whiteList[cmp.Name.Value] = struct{}{}

		for _, dep := range cmp.MayDependOn {
			whiteList[dep.Value] = struct{}{}
			resolveList = append(resolveList, dep.Value)
		}

		resolved[cmp.Name.Value] = struct{}{}
	}

	return whiteList, nil
}

func (o *Operation) compileGraph(ctx context.Context, d2Code string) ([]byte, error) {
	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, fmt.Errorf("failed create ruler: %w", err)
	}

	sketch := true
	renderOpts := &d2svg.RenderOpts{
		Sketch: &sketch,
	}

	diagram, _, err := d2lib.Compile(ctx, d2Code, &d2lib.CompileOptions{
		Ruler: ruler,
		LayoutResolver: func(engine string) (d2graph.LayoutGraph, error) {
			if engine != "dagre" {
				return nil, fmt.Errorf("unsupported layout engine: %s", engine)
			}
			return func(ctx context.Context, g *d2graph.Graph) error {
				return d2dagrelayout.Layout(ctx, g, nil)
			}, nil
		},
	}, renderOpts)
	if err != nil {
		return nil, fmt.Errorf("failed compile d2 graph: %w", err)
	}

	out, err := d2svg.Render(diagram, renderOpts)
	if err != nil {
		return nil, fmt.Errorf("svg render failed: %w", err)
	}

	return out, nil
}

// directedEdge returns the from/to pair adjusted for graph type.
// Flow: component → dependency. DI: dependency → component.
func directedEdge(e graphEdge, graphType models.GraphType) (from, to string) {
	if graphType == models.GraphTypeDI {
		return e.to, e.from
	}

	return e.from, e.to
}
