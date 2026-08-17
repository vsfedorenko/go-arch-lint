package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsfedorenko/go-arch-lint/internal/models"
)

/**
 * ComputeCoupling tests. Reuses the cycles-test fixtures: cyclesSpec
 * builds components owning packages, cyclesFile builds file holds.
 */

func TestComputeCoupling_fan_in_out_and_stability(t *testing.T) {
	spec := cyclesSpec(map[string][]string{
		tcAlpha: {tcPkgAlpha},
		tcBeta:  {tcPkgBeta},
		tcCore:  {tcPkgCore},
	})

	// alpha -> beta (alpha depends on beta)
	// core  -> alpha, core -> beta
	holds := []models.FileHold{
		cyclesFile(tcAlpha, "internal/alpha/a.go", cyclesTestModule+"/"+tcPkgBeta),
		cyclesFile(tcCore, "internal/x.go", cyclesTestModule+"/"+tcPkgAlpha, cyclesTestModule+"/"+tcPkgBeta),
	}

	coupling := ComputeCoupling(spec, holds)
	byName := map[string]models.ComponentCoupling{}
	for _, c := range coupling {
		byName[c.Name] = c
	}

	// alpha: out 1 (beta), in 1 (core) -> I = 1/2
	a := byName[tcAlpha]
	assert.Equal(t, 1, a.OutboundDeps)
	assert.Equal(t, 1, a.InboundDeps)
	assert.InDelta(t, 0.5, a.Stability, 1e-9)

	// beta: out 0, in 2 (alpha, core) -> I = 0/2 = 0 (maximally stable)
	b := byName[tcBeta]
	assert.Equal(t, 0, b.OutboundDeps)
	assert.Equal(t, 2, b.InboundDeps)
	assert.InDelta(t, 0.0, b.Stability, 1e-9)

	// core: out 2, in 0 -> I = 2/2 = 1 (maximally unstable)
	c := byName[tcCore]
	assert.Equal(t, 2, c.OutboundDeps)
	assert.Equal(t, 0, c.InboundDeps)
	assert.InDelta(t, 1.0, c.Stability, 1e-9)
}

func TestComputeCoupling_isolated_component_zero(t *testing.T) {
	spec := cyclesSpec(map[string][]string{
		tcAlpha: {tcPkgAlpha},
		tcBeta:  {tcPkgBeta},
	})

	// No imports at all: both isolated, coupling stays zeroed.
	coupling := ComputeCoupling(spec, []models.FileHold{
		cyclesFile(tcAlpha, "internal/alpha/a.go"),
	})

	require.Len(t, coupling, 2)
	for _, c := range coupling {
		assert.Equal(t, 0, c.OutboundDeps)
		assert.Equal(t, 0, c.InboundDeps)
		assert.InDelta(t, 0.0, c.Stability, 1e-9)
	}
}

func TestComputeCoupling_result_sorted_by_name(t *testing.T) {
	spec := cyclesSpec(map[string][]string{
		tcAlpha: {tcPkgAlpha},
		tcBeta:  {tcPkgBeta},
		tcCore:  {tcPkgCore},
	})

	coupling := ComputeCoupling(spec, nil)
	require.Len(t, coupling, 3)
	assert.Equal(t, []string{tcAlpha, tcBeta, tcCore}, []string{
		coupling[0].Name, coupling[1].Name, coupling[2].Name,
	})
}

func TestComputeCoupling_every_component_present(t *testing.T) {
	spec := cyclesSpec(map[string][]string{
		tcAlpha: {tcPkgAlpha},
		tcBeta:  {tcPkgBeta},
	})

	// Even with zero files resolved every declared component appears.
	coupling := ComputeCoupling(spec, nil)
	assert.Len(t, coupling, 2)
}

func TestComputeCoupling_self_and_vendor_edges_ignored(t *testing.T) {
	spec := cyclesSpec(map[string][]string{
		tcAlpha: {tcPkgAlpha, "internal/alpha/sub"},
	})

	// Self-import (sub-package) and vendor import must not count.
	holds := []models.FileHold{
		cyclesFile(tcAlpha, "internal/alpha/a.go",
			cyclesTestModule+"/internal/alpha/sub",
			"github.com/some/vendor/lib",
		),
	}

	coupling := ComputeCoupling(spec, holds)
	require.Len(t, coupling, 1)
	assert.Equal(t, 0, coupling[0].OutboundDeps)
	assert.Equal(t, 0, coupling[0].InboundDeps)
}
