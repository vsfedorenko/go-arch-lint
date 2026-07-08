package graph

import (
	"strings"
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
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
				{from: "handler", to: "service"},
				{from: "service", to: "repository"},
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
				{from: "handler", to: "service"},
			},
			opts: models.CmdGraphIn{Type: models.GraphTypeDI},
			want: []string{
				"[service] --> [handler]",
			},
		},
		{
			name: "vendor deps use dotted arrow",
			edges: []graphEdge{
				{from: "handler", to: "service"},
				{from: "handler", to: "3rd-cobra", isVendor: true},
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
				if !strings.Contains(got, want) {
					t.Errorf("plantuml output missing %q\nGot:\n%s", want, got)
				}
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
				{from: "handler", to: "service"},
				{from: "service", to: "repository"},
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
				{from: "handler", to: "service"},
			},
			opts: models.CmdGraphIn{Type: models.GraphTypeDI},
			want: []string{
				"service --> handler",
			},
		},
		{
			name: "special chars get bracket notation",
			edges: []graphEdge{
				{from: "handler", to: "3rd-cobra"},
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
				{from: "handler", to: "service"},
				{from: "handler", to: "go-common", isVendor: true},
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
				if !strings.Contains(got, want) {
					t.Errorf("mermaid output missing %q\nGot:\n%s", want, got)
				}
			}
		})
	}
}

func TestRenderD2(t *testing.T) {
	t.Parallel()

	edges := []graphEdge{
		{from: "handler", to: "service"},
		{from: "handler", to: "go-common", isVendor: true},
	}

	op := &Operation{}
	got := op.renderD2(edges, models.CmdGraphIn{Type: models.GraphTypeFlow})

	if !strings.Contains(got, "handler -> service") {
		t.Errorf("d2 output missing component edge\nGot:\n%s", got)
	}

	if !strings.Contains(got, "source-arrowhead") {
		t.Errorf("d2 output missing vendor styling\nGot:\n%s", got)
	}
}
