// Package rofi declares the generated Rofi launcher configuration.
package rofi

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dborovcanin/rice/internal/adapter"
)

type Adapter struct{}

// New returns the Rofi adapter.
func New() adapter.Adapter { return Adapter{} }

func (Adapter) Name() string { return "rofi" }

func (Adapter) Files() []adapter.File {
	return []adapter.File{
		{Template: "rofi/config.rasi.tmpl", Path: "rofi/config.rasi"},
	}
}

func (Adapter) ConfigPaths() []adapter.ManagedPath {
	return []adapter.ManagedPath{
		{Source: "rofi/config.rasi", Target: "rofi/config.rasi"},
	}
}

// ReloadMode is new-instances-only: Rofi reads its config at launch.
func (Adapter) ReloadMode() adapter.ReloadMode { return adapter.ReloadNewInstancesOnly }

// Validate checks that rasi braces balance.
func (Adapter) Validate(dir string) error {
	path := filepath.Join(dir, "rofi", "config.rasi")
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("validate rofi: %w", err)
	}
	defer f.Close()

	depth := 0
	scanner := bufio.NewScanner(f)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(text, "//") || strings.HasPrefix(text, "/*") {
			continue
		}
		depth += strings.Count(text, "{") - strings.Count(text, "}")
		if depth < 0 {
			return fmt.Errorf("validate rofi: %s:%d: unexpected '}'", path, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if depth != 0 {
		return fmt.Errorf("validate rofi: %s: %d unclosed block(s)", path, depth)
	}
	return nil
}
