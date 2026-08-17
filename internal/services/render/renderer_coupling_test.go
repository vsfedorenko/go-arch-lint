package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
	"github.com/vsfedorenko/go-arch-lint/internal/view"
)

/**
 * Coupling metrics rendering: the grouped mapping template prints the
 * coupling line (out/in/stability) for components with metrics and stays
 * silent for entries without them; the JSON output carries the structured
 * fields.
 */

const tcSvcName = "services"

func couplingMappingModel() models.CmdMappingOut {
	return models.CmdMappingOut{
		ProjectDirectory: "/p",
		ModuleName:       "example.com/m",
		Scheme:           models.MappingSchemeGrouped,
		MappingGrouped: []models.CmdMappingOutGrouped{
			{
				ComponentName: tcSvcName,
				FileNames:     []string{"/p/services/a/a.go"},
				Coupling: &models.ComponentCoupling{
					Name:         tcSvcName,
					OutboundDeps: 2,
					InboundDeps:  1,
					Stability:    0.6667,
				},
			},
			{
				ComponentName: "models",
				FileNames:     []string{"/p/models/m.go"},
			},
		},
	}
}

func TestRenderModel_Text_MappingCouplingLine(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatText)
	r.asciiTemplates = view.Templates

	if err := r.RenderModel(couplingMappingModel(), nil); err != nil {
		t.Fatalf("render: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "coupling: out 2 | in 1 | stability 0.67") {
		t.Errorf("expected coupling line in text output, got:\n%s", out)
	}
	if strings.Count(out, "coupling:") != 1 {
		t.Errorf("components without metrics must not print a coupling line, got:\n%s", out)
	}
	if !strings.Contains(out, tcSvcName) || !strings.Contains(out, "models") {
		t.Errorf("expected both components listed, got:\n%s", out)
	}
}

func TestRenderModel_JSON_MappingCouplingFields(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatJSON)
	r.asciiTemplates = view.Templates

	if err := r.RenderModel(couplingMappingModel(), nil); err != nil {
		t.Fatalf("render: %v", err)
	}

	var parsed struct {
		Payload struct {
			MappingGrouped []struct {
				ComponentName string `json:"ComponentName"`
				Coupling      *struct {
					OutboundDeps int     `json:"OutboundDeps"`
					InboundDeps  int     `json:"InboundDeps"`
					Stability    float64 `json:"Stability"`
				} `json:"Coupling"`
			} `json:"MappingGrouped"`
		} `json:"Payload"`
	}

	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("json parse: %v\nraw: %s", err, buf.String())
	}

	if len(parsed.Payload.MappingGrouped) != 2 {
		t.Fatalf("expected 2 grouped entries, got %d", len(parsed.Payload.MappingGrouped))
	}

	withMetrics := parsed.Payload.MappingGrouped[0]
	if withMetrics.ComponentName != tcSvcName || withMetrics.Coupling == nil {
		t.Fatalf("services must carry coupling, got %+v", withMetrics)
	}
	if withMetrics.Coupling.OutboundDeps != 2 ||
		withMetrics.Coupling.InboundDeps != 1 ||
		withMetrics.Coupling.Stability < 0.66 || withMetrics.Coupling.Stability > 0.67 {
		t.Errorf("wrong coupling values: %+v", withMetrics.Coupling)
	}

	withoutMetrics := parsed.Payload.MappingGrouped[1]
	if withoutMetrics.Coupling != nil {
		t.Errorf("models must have no coupling (omitempty), got %+v", withoutMetrics.Coupling)
	}
}
