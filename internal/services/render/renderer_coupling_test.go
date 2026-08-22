package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models"
	"github.com/vsfedorenko/go-arch-lint/v3/internal/view"
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

	require.NoError(t, r.RenderModel(couplingMappingModel(), nil), "render")

	out := buf.String()

	assert.Contains(t, out, "coupling: out 2 | in 1 | stability 0.67", "expected coupling line in text output")
	assert.Equal(t, 1, strings.Count(out, "coupling:"), "components without metrics must not print a coupling line")
	assert.Contains(t, out, tcSvcName, "expected both components listed")
	assert.Contains(t, out, "models", "expected both components listed")
}

func TestRenderModel_JSON_MappingCouplingFields(t *testing.T) {
	r, buf := newTestRenderer(t, models.FormatJSON)
	r.asciiTemplates = view.Templates

	require.NoError(t, r.RenderModel(couplingMappingModel(), nil), "render")

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

	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed), "json parse\nraw: %s", buf.String())

	require.Len(t, parsed.Payload.MappingGrouped, 2, "grouped entries")

	withMetrics := parsed.Payload.MappingGrouped[0]
	require.Equal(t, tcSvcName, withMetrics.ComponentName, "services must be first")
	require.NotNil(t, withMetrics.Coupling, "services must carry coupling")
	assert.Equal(t, 2, withMetrics.Coupling.OutboundDeps, "OutboundDeps")
	assert.Equal(t, 1, withMetrics.Coupling.InboundDeps, "InboundDeps")
	assert.InDelta(t, 0.6667, withMetrics.Coupling.Stability, 0.0001, "Stability")

	withoutMetrics := parsed.Payload.MappingGrouped[1]
	assert.Nil(t, withoutMetrics.Coupling, "models must have no coupling (omitempty)")
}
