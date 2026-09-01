package theme

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Parse decodes, normalizes and validates a theme in one step. Invalid themes
// never reach the renderer.
func Parse(data []byte, source string) (Theme, error) {
	var t Theme

	dec := toml.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return Theme{}, fmt.Errorf("parse theme %s: %s", source, describeTOMLError(err))
	}

	if t.Name == "" {
		t.Name = NameFromPath(source)
	}

	t.Normalize()
	if err := t.Validate(); err != nil {
		return Theme{}, err
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
