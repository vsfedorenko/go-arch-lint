package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

// Component names used across render test cases.
const (
	cmpHandler    = "handler"
	cmpService    = "service"
	cmpRepository = "repository"
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
				{from: cmpHandler, to: "3rd-cobra", isVendor: true},
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
				{from: cmpHandler, to: "3rd-cobra"},
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

	assert.Contains(t, got, "handler -> service", "d2 output component edge")
	assert.Contains(t, got, "source-arrowhead", "d2 output vendor styling")
}
