package dsl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/**
 * Naming()/ForbiddenPackages() builder tests: nesting guard, empty-name
 * rejection, storage order.
 */

func TestNaming_builder_stores_forbidden_packages(t *testing.T) {
	sd := Spec(func() {
		Version(1)
		Component("alphaC", "alpha/**")
		Naming(func() {
			ForbiddenPackages("utils", "helpers", "common")
		})
	})

	b := sd.Builder()
	require.NotNil(t, b.Naming)
	require.Len(t, b.Naming.ForbiddenPackages, 3)
	assert.Equal(t, "utils", b.Naming.ForbiddenPackages[0].Value)
	assert.Equal(t, "helpers", b.Naming.ForbiddenPackages[1].Value)
	assert.Equal(t, "common", b.Naming.ForbiddenPackages[2].Value)
}

func TestNaming_builder_rejects_outside_naming(t *testing.T) {
	assert.Panics(t, func() {
		Spec(func() {
			ForbiddenPackages("utils")
		})
	})
}

func TestNaming_builder_rejects_empty(t *testing.T) {
	assert.Panics(t, func() {
		Spec(func() {
			Naming(func() {
				ForbiddenPackages()
			})
		})
	})

	assert.Panics(t, func() {
		Spec(func() {
			Naming(func() {
				ForbiddenPackages("ok", "")
			})
		})
	})
}

func TestNaming_builder_multiple_calls_append(t *testing.T) {
	sd := Spec(func() {
		Version(1)
		Naming(func() {
			ForbiddenPackages("utils")
		})
		Naming(func() {
			ForbiddenPackages("helpers")
		})
	})

	b := sd.Builder()
	require.NotNil(t, b.Naming)
	assert.Len(t, b.Naming.ForbiddenPackages, 2)
}
