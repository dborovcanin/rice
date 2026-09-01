package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"text/template"

	"github.com/dborovcanin/rice/internal/theme"
)

// FuncMap is the template vocabulary available to every adapter template.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		// Colors.
		"hex":      func(c theme.Color) string { return c.Hex() },
		"bare":     func(c theme.Color) string { return c.Bare() },
		"bareA":    func(c theme.Color) string { return c.BareA() },
		"argb":     func(c theme.Color) string { return c.ARGB() },
		"rgba":     func(c theme.Color) string { return c.RGBA() },
		"alpha":    func(f float64, c theme.Color) theme.Color { return c.Alpha(f) },
		"lighten":  func(f float64, c theme.Color) theme.Color { return c.Lighten(f) },
		"darken":   func(f float64, c theme.Color) theme.Color { return c.Darken(f) },
		"mix":      func(f float64, a, b theme.Color) theme.Color { return a.Mix(b, f) },
		"contrast": func(c theme.Color) theme.Color { return c.Contrast() },

		// Numbers.
		"add":        func(a, b int) int { return a + b },
		"sub":        func(a, b int) int { return a - b },
		"mul":        func(a, b int) int { return a * b },
		"div":        divide,
		"scale":      func(f float64, n int) int { return int(math.Round(float64(n) * f)) },
		"percentage": percentage,
		"float":      formatFloat,

		// Text.
		"quote":     strconv.Quote,
		"join":      func(sep string, items []string) string { return strings.Join(items, sep) },
		"indent":    indent,
		"trim":      strings.TrimSpace,
		"upper":     strings.ToUpper,
		"lower":     strings.ToLower,
		"replace":   func(old, new, s string) string { return strings.ReplaceAll(s, old, new) },
		"contains":  func(sub, s string) bool { return strings.Contains(s, sub) },
		"hasPrefix": func(prefix, s string) bool { return strings.HasPrefix(s, prefix) },
		"default":   defaultValue,
		"comment":   commentBlock,

		// Structured output.
		"json":       toJSON,
		"jsonIndent": toJSONIndent,

		// Composition.
		"font":  pangoFont,
		"lines": splitLines,
	}
}

func divide(a, b int) int {
	if b == 0 {
		return 0
	}
	return a / b
}

func percentage(f float64) string {
	return formatFloat(f*100) + "%"
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// indent prefixes every line after the first with n spaces, which keeps
// multi-line values aligned inside nested blocks.
func indent(n int, s string) string {
	pad := strings.Repeat(" ", n)
	return strings.ReplaceAll(s, "\n", "\n"+pad)
}

// defaultValue returns fallback when value is empty or zero.
func defaultValue(fallback, value any) any {
	switch v := value.(type) {
	case string:
		if v == "" {
			return fallback
		}
	case int:
		if v == 0 {
			return fallback
		}
	case float64:
		if v == 0 {
			return fallback
		}
	case nil:
		return fallback
	}
	return value
}

// commentBlock prefixes each line with "# ", for passing user notes through
// into generated files.
func commentBlock(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "# " + l
	}
	return strings.Join(lines, "\n")
}

func toJSON(v any) (string, error) {
	return encodeJSON(v, "", "")
}

// toJSONIndent renders v as JSON indented by n spaces, with continuation lines
// aligned to the current position in the template.
func toJSONIndent(n int, v any) (string, error) {
	return encodeJSON(v, strings.Repeat(" ", n), "  ")
}

// encodeJSON marshals without HTML escaping: Waybar format strings contain
// Pango markup, and \u003c in a config file helps nobody.
func encodeJSON(v any, prefix, indent string) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent(prefix, indent)
	if err := enc.Encode(v); err != nil {
		return "", fmt.Errorf("marshal json: %w", err)
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

// pangoFont builds the "Family Size" string Pango-based applications expect.
func pangoFont(family string, size int) string {
	if family == "" {
		return ""
	}
	return fmt.Sprintf("%s %d", family, size)
}

func splitLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
