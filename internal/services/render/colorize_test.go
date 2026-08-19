package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestColorize_Dispatch verifies every supported color paints through the
// printer and unknown colors surface as errors (template typos must not
// silently drop color).
func TestColorize_Dispatch(t *testing.T) {
	printer := passthroughPrinter{}
	c := newColorizer(&printer)

	for _, color := range []string{"red", "green", "yellow", "blue", "magenta", "cyan", "gray"} {
		out, err := c.colorize(color, "x")
		require.NoError(t, err, "colorize(%q)", color)
		assert.Equal(t, "x", out, "colorize(%q)", color)
	}

	_, err := c.colorize("chartreuse", "x")
	require.Error(t, err, "expected error for unknown color")
}

// passthroughPrinter satisfies colorPrinter; every method returns its
// input unchanged.
type passthroughPrinter struct{}

func (passthroughPrinter) Red(s string) string     { return s }
func (passthroughPrinter) Green(s string) string   { return s }
func (passthroughPrinter) Yellow(s string) string  { return s }
func (passthroughPrinter) Blue(s string) string    { return s }
func (passthroughPrinter) Magenta(s string) string { return s }
func (passthroughPrinter) Cyan(s string) string    { return s }
func (passthroughPrinter) Gray(s string) string    { return s }
