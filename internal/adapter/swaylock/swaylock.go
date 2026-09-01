// Package swaylock declares the generated lock screen configuration.
package swaylock

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dborovcanin/rice/internal/adapter"
)

type Adapter struct{}

// New returns the swaylock adapter.
func New() adapter.Adapter { return Adapter{} }

func (Adapter) Name() string { return "swaylock" }

func (Adapter) Files() []adapter.File {
	return []adapter.File{
		{Template: "swaylock/config.tmpl", Path: "swaylock/config"},
	}
}

func (Adapter) ConfigPaths() []adapter.ManagedPath {
	return []adapter.ManagedPath{
		{Source: "swaylock/config", Target: "swaylock/config"},
	}
}

// ReloadMode is new-instances-only: the config is read when the screen locks.
func (Adapter) ReloadMode() adapter.ReloadMode { return adapter.ReloadNewInstancesOnly }

// Validate checks that every line is a long option, with or without a value,
// which is the format swaylock's config parser accepts.
func (Adapter) Validate(dir string) error {
	path := filepath.Join(dir, "swaylock", "config")
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("validate swaylock: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if strings.HasPrefix(text, "-") {
			return fmt.Errorf("validate swaylock: %s:%d: options must not be dash-prefixed: %q", path, line, text)
		}
		if strings.ContainsAny(strings.SplitN(text, "=", 2)[0], " \t") {
			return fmt.Errorf("validate swaylock: %s:%d: malformed option: %q", path, line, text)
		}
	}
	return scanner.Err()
}
