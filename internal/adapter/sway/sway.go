// Package sway declares the generated SwayFX compositor configuration.
package sway

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/config"
)

type Adapter struct{}

// New returns the SwayFX adapter.
func New() adapter.Adapter { return Adapter{} }

func (Adapter) Name() string { return "sway" }

func (Adapter) Files() []adapter.File { return filesFor(true) }

// FilesFor drops the environment file when it is turned off.
func (Adapter) FilesFor(cfg config.Config) []adapter.File {
	return filesFor(cfg.Sway.WriteEnvironment)
}

func filesFor(environment bool) []adapter.File {
	files := []adapter.File{
		{Template: "sway/config.tmpl", Path: "sway/config"},
	}
	if environment {
		files = append(files, adapter.File{
			Template: "sway/environment.conf.tmpl", Path: "sway/environment.conf",
		})
	}
	return files
}

func (Adapter) ConfigPaths() []adapter.ManagedPath { return pathsFor(true) }

func (Adapter) ConfigPathsFor(cfg config.Config) []adapter.ManagedPath {
	return pathsFor(cfg.Sway.WriteEnvironment)
}

func pathsFor(environment bool) []adapter.ManagedPath {
	paths := []adapter.ManagedPath{
		{Source: "sway/config", Target: "sway/config"},
	}
	if environment {
		paths = append(paths, adapter.ManagedPath{
			Source: "sway/environment.conf",
			Target: filepath.Join("environment.d", "50-rice.conf"),
		})
	}
	return paths
}

// ReloadMode is hot: `swaymsg reload` re-reads the whole config in place.
func (Adapter) ReloadMode() adapter.ReloadMode { return adapter.ReloadHot }

// Validate checks brace balance across the generated config. Full validation
// needs `sway -C`, which belongs to deployment rather than generation.
func (Adapter) Validate(dir string) error {
	path := filepath.Join(dir, "sway", "config")
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("validate sway: %w", err)
	}
	defer f.Close()

	depth := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(text, "#") {
			continue
		}
		depth += strings.Count(text, "{") - strings.Count(text, "}")
		if depth < 0 {
			return fmt.Errorf("validate sway: %s:%d: unexpected '}'", path, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if depth != 0 {
		return fmt.Errorf("validate sway: %s: %d unclosed block(s)", path, depth)
	}
	return nil
}
