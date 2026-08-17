package dsl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/**
 * Interfaces()/MustLiveWithConsumer() builder tests.
 */

func TestInterfaces_builder_enables_rule(t *testing.T) {
	sd := Spec(func() {
		Version(1)
		Interfaces(func() {
			MustLiveWithConsumer()
		})
	})

	b := sd.Builder()
	require.NotNil(t, b.InterfacePlacement)
	assert.True(t, b.InterfacePlacement.MustLiveWithConsumer)
}

func TestInterfaces_builder_default_off(t *testing.T) {
	sd := Spec(func() {
		Version(1)
	})

	b := sd.Builder()
	assert.Nil(t, b.InterfacePlacement)
}

func TestInterfaces_builder_rejects_outside(t *testing.T) {
	assert.Panics(t, func() {
		Spec(func() {
			MustLiveWithConsumer()
		})
	})
}
