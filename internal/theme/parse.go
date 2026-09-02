package theme

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Parse decodes, normalizes and validates a theme in one step. Invalid themes
// never reach the renderer.
func Parse(data []byte, source string) (Theme, error) {
	t, err := ParseSource(data, source)
	if err != nil {
		return Theme{}, err
	}

	t.Normalize()
	if err := t.Validate(); err != nil {
		return Theme{}, err
	}
	return t, nil
}

// ParseSource decodes a theme exactly as written, without filling in defaults.
// Unset fields stay zero, which is how the file records "derive this for me".
//
// Editing tools want this form: normalizing first would materialize every
// derived value, and an edit to a semantic color could then no longer reach
// the values derived from it.
func ParseSource(data []byte, source string) (Theme, error) {
	var t Theme

	dec := toml.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return Theme{}, fmt.Errorf("parse theme %s: %s", source, describeTOMLError(err))
	}

	if t.Name == "" {
		t.Name = NameFromPath(source)
	}
	return t, nil
}

// ParseFile reads and parses a theme from disk.
func ParseFile(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, fmt.Errorf("read theme: %w", err)
	}
	return Parse(data, path)
}

// ParseSourceFile reads a theme from disk in its source form.
func ParseSourceFile(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, fmt.Errorf("read theme: %w", err)
	}
	return ParseSource(data, path)
}

// Resolved returns a normalized, renderable copy of the theme, leaving the
// receiver in its source form.
func (t Theme) Resolved() Theme {
	t.Icons.Paths = slices.Clone(t.Icons.Paths)
	t.Normalize()
	return t
}

// NameFromPath derives a theme name from its file name, so
// "themes/gruvbox-dark.toml" becomes "gruvbox-dark".
func NameFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// describeTOMLError expands go-toml's strict-mode error, which otherwise says
// only that some field is unknown without naming it.
func describeTOMLError(err error) string {
	var strict *toml.StrictMissingError
	if errors.As(err, &strict) {
		return "unknown field:\n" + strict.String()
	}
	return err.Error()
}

// unsetArray matches a colour array whose every entry is unset. go-toml has no
// way to omit a fixed-size array, so an untouched ANSI palette would otherwise
// be written out as sixteen "#00000000" entries — which reads as broken rather
// than as absent, and invites someone to "fix" it by hand.
//
// The entries do round-trip correctly either way: "#00000000" parses back to
// the zero Color, which is what "derive this for me" means. This is about the
// file being readable.
var unsetArray = regexp.MustCompile(`^(regular|bright) = \['#00000000'(, '#00000000')*\]$`)

// Encode renders a theme as TOML in source form, ready to be written to a
// theme file. Values the theme leaves derived stay out of the output.
func Encode(t Theme) ([]byte, error) {
	data, err := toml.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("encode theme: %w", err)
	}

	var kept []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if unsetArray.MatchString(strings.TrimSpace(line)) {
			continue
		}
		kept = append(kept, line)
	}

	// Dropping every key in a table leaves its header behind with nothing
	// under it, which parses fine but reads like something went missing.
	var b strings.Builder
	b.Grow(len(data))
	for i, line := range kept {
		if isTableHeader(line) && !hasKeysUnder(kept[i+1:]) {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return []byte(strings.TrimRight(b.String(), "\n") + "\n"), nil
}

// isTableHeader reports whether a line opens a TOML table.
func isTableHeader(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")
}

// hasKeysUnder reports whether any key belongs to the table that just opened,
// which is true until the next table header.
func hasKeysUnder(rest []string) bool {
	for _, line := range rest {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isTableHeader(trimmed) {
			return false
		}
		return true
	}
	return false
}
