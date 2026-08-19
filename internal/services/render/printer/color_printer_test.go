package printer

import (
	"testing"

	"github.com/logrusorgru/aurora/v3"
	"github.com/stretchr/testify/assert"
)

// The color printer is a thin adapter over aurora; the contract worth
// pinning: colors ON produce ANSI escapes, colors OFF produce plain text
// (the --no-colors / --output-color=false paths depend on it).

func TestColorPrinter_ColorsOn(t *testing.T) {
	cp := NewColorPrinter(aurora.NewAurora(true))
	out := cp.Red("boom")
	assert.Contains(t, out, "\x1b[", "colors on must emit ANSI escapes")
	assert.Contains(t, out, "boom", "payload must survive")
}

func TestColorPrinter_ColorsOff(t *testing.T) {
	cp := NewColorPrinter(aurora.NewAurora(false))
	for name, fn := range map[string]func(string) string{
		"red":     cp.Red,
		"green":   cp.Green,
		"yellow":  cp.Yellow,
		"blue":    cp.Blue,
		"magenta": cp.Magenta,
		"cyan":    cp.Cyan,
		"white":   cp.White,
		"gray":    cp.Gray,
	} {
		assert.Equal(t, "plain", fn("plain"), "%s with colors off must be identity", name)
	}
}

func TestColorPrinter_ColorsOnAllMethods(t *testing.T) {
	cp := NewColorPrinter(aurora.NewAurora(true))
	for name, fn := range map[string]func(string) string{
		"red":     cp.Red,
		"green":   cp.Green,
		"yellow":  cp.Yellow,
		"blue":    cp.Blue,
		"magenta": cp.Magenta,
		"cyan":    cp.Cyan,
		"white":   cp.White,
		"gray":    cp.Gray,
	} {
		assert.Contains(t, fn("x"), "\x1b[", "%s with colors on must emit escapes", name)
	}
}
