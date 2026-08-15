package render

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

// stringify renders a template-function argument as a string.
//
// Most ASCII template helpers accept interface{} because text/template
// passes whatever the template expression produced; the dominant case is
// already a string, so the fast path avoids fmt overhead.
func stringify(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", value)
}

func (r *Renderer) asciiColorize(color string, value interface{}) (string, error) {
	colorizer := newColorizer(r.colorPrinter)
	out, err := colorizer.colorize(
		color,
		stringify(value),
	)
	if err != nil {
		return "", fmt.Errorf("failed colorize: %w", err)
	}

	return out, nil
}

func (r *Renderer) asciiTrimPrefix(prefix string, value interface{}) string {
	return strings.TrimPrefix(stringify(value), prefix)
}

func (r *Renderer) asciiTrimSuffix(suffix string, value interface{}) string {
	return strings.TrimSuffix(stringify(value), suffix)
}

func (r *Renderer) asciiDefaultValue(def string, value interface{}) string {
	sValue := stringify(value)

	if sValue == "" {
		return def
	}

	return sValue
}

func (r *Renderer) asciiPadLeft(overallLen int, padStr string, value interface{}) string {
	s := stringify(value)

	padCountInt := 1 + ((overallLen - len(padStr)) / len(padStr))
	retStr := strings.Repeat(padStr, padCountInt) + s
	return retStr[(len(retStr) - overallLen):]
}

func (r *Renderer) asciiPadRight(overallLen int, padStr string, value interface{}) string {
	s := stringify(value)

	padCountInt := 1 + ((overallLen - len(padStr)) / len(padStr))
	retStr := s + strings.Repeat(padStr, padCountInt)
	return retStr[:overallLen]
}

func (r *Renderer) asciiLinePrefix(prefix string, value interface{}) string {
	lines := stringify(value)
	result := make([]string, 0)

	for _, line := range strings.Split(lines, "\n") {
		result = append(result, prefix+line)
	}

	return strings.Join(result, "\n")
}

func (r *Renderer) asciiPathDirectory(value interface{}) string {
	return path.Dir(stringify(value))
}

func (r *Renderer) asciiPlus(a, b interface{}) (int, error) {
	iA, err := toInt(a)
	if err != nil {
		return 0, fmt.Errorf("component A of 'plus' is not int: %s", a)
	}

	iB, err := toInt(b)
	if err != nil {
		return 0, fmt.Errorf("component B of 'plus' is not int: %s", b)
	}

	return iA + iB, nil
}

// toInt converts a template-function argument to int (int, int64, float64
// from YAML/JSON decoding, or numeric string).
func toInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("unsupported type %T", value)
	}
}

// asciiMinus is deliberately absent: it was dead code (referenced by no
// template) and buggy — it returned a+b instead of a-b. Should a template
// ever need subtraction, add it back with a real subtraction and a test.

func (r *Renderer) asciiConcat(sources ...interface{}) string {
	var sb strings.Builder

	for _, source := range sources {
		sb.WriteString(stringify(source))
	}

	return sb.String()
}
