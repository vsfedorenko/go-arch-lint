package dsl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/** Visibility()/VisibleTo() DSL contract. */

func TestVisibility_collects_rules(t *testing.T) {
	spec := Spec(func() {
		Version(1)
		Workdir("internal")
		Component("alpha", "alpha/**")
		Component("beta", "beta/**")
		Visibility(func() {
			VisibleTo("alpha")
			VisibleTo("beta", "alpha")
		})
	})

	require.NotNil(t, spec.builder.Visibility)
	require.Len(t, spec.builder.Visibility.Rules, 2)

	ruleAlpha := spec.builder.Visibility.Rules[0]
	assert.Equal(t, "alpha", ruleAlpha.Component)
	assert.Empty(t, ruleAlpha.Allowed)

	ruleBeta := spec.builder.Visibility.Rules[1]
	assert.Equal(t, "beta", ruleBeta.Component)
	assert.Equal(t, []string{"alpha"}, ruleBeta.Allowed)
}

func TestVisibility_nested_visible_to_required(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r, "VisibleTo outside Visibility must panic")
	}()

	Spec(func() {
		Version(1)
		VisibleTo("alpha") // not inside Visibility -> panic
	})
}

func TestVisibility_empty_component_panics(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r)
	}()

	Spec(func() {
		Version(1)
		Visibility(func() {
			VisibleTo("")
		})
	})
}

func TestVisibility_self_in_allowed_panics(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r)
	}()

	Spec(func() {
		Version(1)
		Visibility(func() {
			VisibleTo("alpha", "alpha")
		})
	})
}

func TestVisibility_duplicate_allowed_ok(t *testing.T) {
	// Duplicates in Allowed are tolerated — the checker uses a set.
	spec := Spec(func() {
		Version(1)
		Visibility(func() {
			VisibleTo("alpha", "beta", "beta")
		})
	})

	require.Len(t, spec.builder.Visibility.Rules, 1)
	assert.Len(t, spec.builder.Visibility.Rules[0].Allowed, 2)
}
