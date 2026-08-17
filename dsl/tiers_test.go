package dsl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/**
 * Tiers()/Tier() builder tests: declaration order, duplicates, unknown
 * components, double assignment — all must panic at spec-build time with
 * clear messages (fail fast, not at lint time).
 */

func TestTiers_builder_declares_ordered_layers(t *testing.T) {
	sd := Spec(func() {
		Version(1)
		Workdir("internal")
		Component("alphaC", "alpha/**")
		Component("betaC", "beta/**")
		Tiers("domain", "app")
		Tier("domain", "alphaC")
		Tier("app", "betaC")
	})

	b := sd.Builder()
	require.Len(t, b.Tiers, 2)
	assert.Equal(t, "domain", b.Tiers[0].Name)
	assert.Equal(t, []string{"alphaC"}, b.Tiers[0].Components)
	assert.Equal(t, "app", b.Tiers[1].Name)
	assert.Equal(t, []string{"betaC"}, b.Tiers[1].Components)
}

func TestTiers_builder_rejects_unknown_component(t *testing.T) {
	assert.Panics(t, func() {
		Spec(func() {
			Version(1)
			Component("alphaC", "alpha/**")
			Tiers("domain")
			Tier("domain", "ghost")
		})
	})
}

func TestTiers_builder_rejects_duplicate_tier(t *testing.T) {
	assert.Panics(t, func() {
		Spec(func() {
			Version(1)
			Tiers("domain", "app")
			Tiers("domain")
		})
	})
}

func TestTiers_builder_rejects_component_in_two_tiers(t *testing.T) {
	assert.Panics(t, func() {
		Spec(func() {
			Version(1)
			Component("alphaC", "alpha/**")
			Tiers("domain", "app")
			Tier("domain", "alphaC")
			Tier("app", "alphaC")
		})
	})
}

func TestTiers_builder_undeclared_tier_name(t *testing.T) {
	assert.Panics(t, func() {
		Spec(func() {
			Version(1)
			Component("alphaC", "alpha/**")
			Tier("ghost", "alphaC")
		})
	})
}

func TestTiers_builder_empty_args(t *testing.T) {
	assert.Panics(t, func() {
		Spec(func() { Tiers() })
	})
}
