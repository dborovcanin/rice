// Package waybar declares the generated Waybar configuration and stylesheet.
package waybar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dborovcanin/rice/internal/adapter"
)

type Adapter struct{}

// New returns the Waybar adapter.
func New() adapter.Adapter { return Adapter{} }

func (Adapter) Name() string { return "waybar" }

func (Adapter) Files() []adapter.File {
	return []adapter.File{
		{Template: "waybar/config.jsonc.tmpl", Path: "waybar/config.jsonc"},
		{Template: "waybar/style.css.tmpl", Path: "waybar/style.css"},
	}
}

func (Adapter) ConfigPaths() []adapter.ManagedPath {
	return []adapter.ManagedPath{
		{Source: "waybar/config.jsonc", Target: "waybar/config.jsonc"},
		{Source: "waybar/style.css", Target: "waybar/style.css"},
	}
}

// ReloadMode is signal: Waybar reloads on SIGUSR2, and restarts otherwise.
func (Adapter) ReloadMode() adapter.ReloadMode { return adapter.ReloadSignal }

// Validate parses the generated bar configuration as JSON. Rice emits strict
// JSON into the .jsonc file precisely so this check is meaningful.
func (Adapter) Validate(dir string) error {
	path := filepath.Join(dir, "waybar", "config.jsonc")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("validate waybar: %w", err)
	}
	var out any
	if err := json.Unmarshal(stripComments(data), &out); err != nil {
		return fmt.Errorf("validate waybar: %s: %w", path, err)
	}
	return nil
}

// stripComments removes // and /* */ comments from JSONC, leaving string
// literals alone so that URLs and format strings survive intact. Waybar
// accepts comments; encoding/json does not.
func stripComments(data []byte) []byte {
	out := make([]byte, 0, len(data))

	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		c := data[i]

		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}

		switch {
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			i++
		default:
			out = append(out, c)
		}
	}
	return out
}
