// Package dunst declares the generated notification daemon configuration.
package dunst

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dborovcanin/rice/internal/adapter"
)

type Adapter struct{}

// New returns the Dunst adapter.
func New() adapter.Adapter { return Adapter{} }

func (Adapter) Name() string { return "dunst" }

func (Adapter) Files() []adapter.File {
	return []adapter.File{
		{Template: "dunst/dunstrc.tmpl", Path: "dunst/dunstrc"},
	}
}

func (Adapter) ConfigPaths() []adapter.ManagedPath {
	return []adapter.ManagedPath{
		{Source: "dunst/dunstrc", Target: "dunst/dunstrc"},
	}
}

// ReloadMode is signal: `dunstctl reload` re-reads the config in place.
func (Adapter) ReloadMode() adapter.ReloadMode { return adapter.ReloadSignal }

// Validate checks that dunstrc is a well-formed INI file and that it defines
// the [global] section Dunst requires.
func (Adapter) Validate(dir string) error {
	path := filepath.Join(dir, "dunst", "dunstrc")
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("validate dunst: %w", err)
	}
	defer f.Close()

	hasGlobal := false
	scanner := bufio.NewScanner(f)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if strings.HasPrefix(text, "[") {
			if !strings.HasSuffix(text, "]") {
				return fmt.Errorf("validate dunst: %s:%d: unterminated section header", path, line)
			}
			if text == "[global]" {
				hasGlobal = true
			}
			continue
		}
		if !strings.Contains(text, "=") {
			return fmt.Errorf("validate dunst: %s:%d: not a key=value pair: %q", path, line, text)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !hasGlobal {
		return fmt.Errorf("validate dunst: %s: missing [global] section", path)
	}
	return nil
}
