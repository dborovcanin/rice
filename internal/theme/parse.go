package theme

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
