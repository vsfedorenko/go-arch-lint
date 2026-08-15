package render

import "fmt"

// Color names accepted by the `colorize` template function.
const (
	colorRed     colorName = "red"
	colorGreen   colorName = "green"
	colorYellow  colorName = "yellow"
	colorBlue    colorName = "blue"
	colorMagenta colorName = "magenta"
	colorCyan    colorName = "cyan"
	colorGray    colorName = "gray"
)

type (
	colorizer struct {
		printer colorPrinter
	}

	colorName = string
)

func newColorizer(printer colorPrinter) *colorizer {
	return &colorizer{
		printer: printer,
	}
}

// painters maps each supported color to its printer method.
var painters = map[colorName]func(colorPrinter, string) string{
	colorRed:     (colorPrinter).Red,
	colorGreen:   (colorPrinter).Green,
	colorYellow:  (colorPrinter).Yellow,
	colorBlue:    (colorPrinter).Blue,
	colorMagenta: (colorPrinter).Magenta,
	colorCyan:    (colorPrinter).Cyan,
	colorGray:    (colorPrinter).Gray,
}

// colorize paints input with the named color. Unknown colors are an error —
// a typo in a template must surface, not silently drop the color.
func (c *colorizer) colorize(color colorName, input string) (string, error) {
	if paint, ok := painters[color]; ok {
		return paint(c.printer, input), nil
	}
	return "", fmt.Errorf("invalid color '%s'", color)
}
