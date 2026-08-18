package printer

import (
	"strings"
	"testing"

	"github.com/logrusorgru/aurora/v3"
)

// The color printer is a thin adapter over aurora; the contract worth
// pinning: colors ON produce ANSI escapes, colors OFF produce plain text
// (the --no-colors / --output-color=false paths depend on it).

func TestColorPrinter_ColorsOn(t *testing.T) {
	cp := NewColorPrinter(aurora.NewAurora(true))
	out := cp.Red("boom")
	if !strings.HasPrefix(out, "\x1b[") {
		t.Fatalf("colors on must emit ANSI escapes, got %q", out)
	}
	if !strings.Contains(out, "boom") {
		t.Fatalf("payload lost: %q", out)
	}
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
		if out := fn("plain"); out != "plain" {
			t.Errorf("%s with colors off must be identity, got %q", name, out)
		}
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
		if out := fn("x"); !strings.Contains(out, "\x1b[") {
			t.Errorf("%s with colors on must emit escapes, got %q", name, out)
		}
	}
}
