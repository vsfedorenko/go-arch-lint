package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ValidateFlagPairs is the single rule set behind the fail-fast contract
// for incoherent flag combinations, shared by the SDK path (archlint.Run)
// and the cobra tree the scaffolded runner drives (MustRunCLI). The
// contracts are pinned end-to-end in archlint_cli_test.go; these unit
// cases pin the rule boundaries directly.
func TestValidateFlagPairs(t *testing.T) {
	t.Run("baseline update without baseline is rejected", func(t *testing.T) {
		err := CheckOptions{BaselineUpdate: true}.ValidateFlagPairs()
		require.Error(t, err)
		assert.True(t, IsConfigError(err), "must be a config error, got %T", err)
		assert.Contains(t, err.Error(), "requires --baseline")
	})

	t.Run("baseline update with baseline is fine", func(t *testing.T) {
		assert.NoError(t, CheckOptions{BaselineUpdate: true, BaselinePath: "b.json"}.ValidateFlagPairs())
	})

	t.Run("one-line without json output is rejected", func(t *testing.T) {
		err := CheckOptions{OutputJSONOneLine: true, OutputType: OutputTypeASCII}.ValidateFlagPairs()
		require.Error(t, err)
		assert.True(t, IsConfigError(err), "must be a config error, got %T", err)
		assert.Contains(t, err.Error(), "--output-json-one-line")
	})

	t.Run("one-line with output-type json is fine", func(t *testing.T) {
		assert.NoError(t, CheckOptions{OutputJSONOneLine: true, OutputType: OutputTypeJSON}.ValidateFlagPairs())
	})

	t.Run("one-line with format json is fine", func(t *testing.T) {
		assert.NoError(t, CheckOptions{OutputJSONOneLine: true, Format: FormatJSON}.ValidateFlagPairs())
	})

	t.Run("coherent options pass", func(t *testing.T) {
		assert.NoError(t, CheckOptions{}.ValidateFlagPairs())
	})
}
