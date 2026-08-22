package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/arch"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

// Component names used across render test cases.
const (
	cmpHandler    = "handler"
	cmpService    = "service"
	cmpRepository = "repository"

	cmpVendorCobra = "3rd-cobra"
	cmpRoot        = "."
	cmpOrder       = "internal/order"
	cmpUser        = "internal/user"
)

func TestRenderPlantUML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		edges []graphEdge
		opts  models.CmdGraphIn
		want  []string
	}{
		{
			name: "flow type basic deps",
			edges: []graphEdge{
				{from: cmpHandler, to: cmpService},
				{from: cmpService, to: cmpRepository},
			},
			opts: models.CmdGraphIn{Type: models.GraphTypeFlow},
			want: []string{
				"@startuml",
				"component [\"handler\"]",
				"component [\"service\"]",
				"component [\"repository\"]",
				"[handler] --> [service]",
				"[service] --> [repository]",
				"@enduml",
			},
		},
		{
			name: "di type reverses arrows",
			edges: []graphEdge{
				{from: cmpHandler, to: cmpService},
			},
			opts: models.CmdGraphIn{Type: models.GraphTypeDI},
			want: []string{
				"[service] --> [handler]",
			},
		},
		{
			name: "vendor deps use dotted arrow",
			edges: []graphEdge{
				{from: cmpHandler, to: cmpService},
				{from: cmpHandler, to: cmpVendorCobra, isVendor: true},
			},
			opts: models.CmdGraphIn{Type: models.GraphTypeFlow},
			want: []string{
				"[handler] --> [service]",
				"[handler] ..> [3rd-cobra]",
			},
		},
	}

	op := &Operation{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := op.renderPlantUML(tt.edges, tt.opts)

			for _, want := range tt.want {
				assert.Contains(t, got, want, "plantuml output")
			}
		})
	}
}

func TestRenderMermaid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		edges []graphEdge
		opts  models.CmdGraphIn
		want  []string
	}{
		{
			name: "flow type basic deps",
			edges: []graphEdge{
				{from: cmpHandler, to: cmpService},
				{from: cmpService, to: cmpRepository},
			},
			opts: models.CmdGraphIn{Type: models.GraphTypeFlow},
			want: []string{
				"graph LR",
				"handler --> service",
				"service --> repository",
			},
		},
		{
			name: "di type reverses arrows",
			edges: []graphEdge{
				{from: cmpHandler, to: cmpService},
			},
			opts: models.CmdGraphIn{Type: models.GraphTypeDI},
			want: []string{
				"service --> handler",
			},
		},
		{
			name: "special chars get bracket notation",
			edges: []graphEdge{
				{from: cmpHandler, to: cmpVendorCobra},
			},
			opts: models.CmdGraphIn{Type: models.GraphTypeFlow},
			want: []string{
				`["3rd-cobra"]`,
				"handler --> ",
			},
		},
		{
			name: "vendor deps use dotted arrow",
			edges: []graphEdge{
				{from: cmpHandler, to: cmpService},
				{from: cmpHandler, to: "go-common", isVendor: true},
			},
			opts: models.CmdGraphIn{Type: models.GraphTypeFlow},
			want: []string{
				"handler --> service",
				`["go-common"]`,
				"-.->",
			},
		},
	}

	op := &Operation{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := op.renderMermaid(tt.edges, tt.opts)

			for _, want := range tt.want {
				assert.Contains(t, got, want, "mermaid output")
			}
		})
	}
}

func TestRenderD2(t *testing.T) {
	t.Parallel()

	edges := []graphEdge{
		{from: cmpHandler, to: cmpService},
		{from: cmpHandler, to: "go-common", isVendor: true},
	}

	op := &Operation{}
	got := op.renderD2(edges, models.CmdGraphIn{Type: models.GraphTypeFlow})

	assert.Contains(t, got, `"handler" -> "service"`, "d2 output component edge")
	assert.Contains(t, got, "source-arrowhead", "d2 output vendor styling")
}

// TestRenderD2KeysPin pins the exact key quoting rules: component names are
// directory paths, and d2 reserves "." while dots nest — both would corrupt
// the diagram if keys were emitted unquoted.
func TestRenderD2KeysPin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		edges []graphEdge
		want  []string
	}{
		{
			name: "root component is quoted",
			edges: []graphEdge{
				{from: cmpRoot, to: cmpHandler},
			},
			want: []string{
				`"." -> "handler"`,
			},
		},
		{
			name: "dotted path segment is quoted",
			edges: []graphEdge{
				{from: cmpHandler, to: "pkg.v1"},
			},
			want: []string{
				`"handler" -> "pkg.v1"`,
			},
		},
		{
			name: "vendor styling quotes style target",
			edges: []graphEdge{
				{from: cmpRoot, to: cmpVendorCobra, isVendor: true},
			},
			want: []string{
				`"3rd-cobra".style.font-size: 12`,
				`"." -> "3rd-cobra"`,
			},
		},
	}

	op := &Operation{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := op.renderD2(tt.edges, models.CmdGraphIn{Type: models.GraphTypeFlow})

			for _, want := range tt.want {
				assert.Contains(t, got, want, "d2 output key quoting")
			}
		})
	}
}

// TestRenderD2Compiles guards the whole d2 pipeline: the text produced for
// path-shaped component names (including the "." module root every scaffold
// declares) must survive d2 compilation used by the svg format.
func TestRenderD2Compiles(t *testing.T) {
	t.Parallel()

	edges := []graphEdge{
		{from: cmpRoot, to: cmpOrder},
		{from: cmpRoot, to: cmpUser},
		{from: cmpOrder, to: cmpUser},
		{from: cmpOrder, to: cmpVendorCobra, isVendor: true},
	}

	op := &Operation{}
	src := op.renderD2(edges, models.CmdGraphIn{Type: models.GraphTypeFlow})

	svg, err := op.compileGraph(t.Context(), src)

	require.NoError(t, err, "d2 source must compile for path-shaped component names")
	assert.Contains(t, string(svg), "<svg", "compiled output is an svg document")
}

// TestCollectEdgesSkipsSelfLoops pins the implicit self-dependency rule out
// of the graph: the decoder grants every component a MayDependOn entry for
// itself, which must not surface as an edge in any format.
func TestCollectEdgesSkipsSelfLoops(t *testing.T) {
	t.Parallel()

	spec := arch.Spec{
		Components: []arch.Component{
			{Name: domain.NewReferable(cmpRoot, domain.NewEmptyReference()), MayDependOn: []domain.Referable[string]{
				domain.NewReferable(cmpRoot, domain.NewEmptyReference()),
				domain.NewReferable(cmpOrder, domain.NewEmptyReference()),
			}},
			{Name: domain.NewReferable(cmpOrder, domain.NewEmptyReference()), MayDependOn: []domain.Referable[string]{
				domain.NewReferable(cmpOrder, domain.NewEmptyReference()),
			}},
		},
	}

	op := &Operation{}
	whiteList := map[string]struct{}{cmpRoot: {}, cmpOrder: {}}
	edges := op.collectEdges(spec, models.CmdGraphIn{}, whiteList)

	want := []graphEdge{{from: cmpRoot, to: cmpOrder}}
	assert.Equal(t, want, edges, "only the cross-component edge remains")
}
