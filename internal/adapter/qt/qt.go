// Package qt declares the generated Qt platform theme configuration.
//
// Qt has no global icon size either, so the theme's icons.size does not reach
// it. What Rice sets is the widget style, the icon theme and the fonts, plus
// the Kvantum theme when Kvantum is the chosen style.
package qt

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

// New returns the Qt adapter.
func New() adapter.Adapter { return Adapter{} }

func (Adapter) Name() string { return "qt" }

// Files is the full set, used when no configuration narrows it.
func (Adapter) Files() []adapter.File { return filesFor(true, true, true) }

// FilesFor generates only the platform themes that are turned on.
func (Adapter) FilesFor(cfg config.Config) []adapter.File {
	return filesFor(cfg.Qt.Qt5ct, cfg.Qt.Qt6ct, cfg.Qt.Kvantum)
}

func (Adapter) ConfigPaths() []adapter.ManagedPath { return pathsFor(true, true, true) }

func (Adapter) ConfigPathsFor(cfg config.Config) []adapter.ManagedPath {
	return pathsFor(cfg.Qt.Qt5ct, cfg.Qt.Qt6ct, cfg.Qt.Kvantum)
}

// qt5ct and qt6ct read the same format from different directories, so they are
// rendered separately only to keep the generation readable as two files that
// match the two places they are deployed to.
func filesFor(qt5, qt6, kvantum bool) []adapter.File {
	var out []adapter.File
	if qt5 {
		out = append(out, adapter.File{Template: "qt/qtct.conf.tmpl", Path: "qt/qt5ct.conf"})
	}
	if qt6 {
		out = append(out, adapter.File{Template: "qt/qtct.conf.tmpl", Path: "qt/qt6ct.conf"})
	}
	if kvantum {
		out = append(out, adapter.File{
			Template: "qt/kvantum.kvconfig.tmpl", Path: "qt/kvantum.kvconfig",
		})
	}
	return out
}

func pathsFor(qt5, qt6, kvantum bool) []adapter.ManagedPath {
	var out []adapter.ManagedPath
	if qt5 {
		out = append(out, adapter.ManagedPath{
			Source: "qt/qt5ct.conf", Target: filepath.Join("qt5ct", "qt5ct.conf"),
		})
	}
	if qt6 {
		out = append(out, adapter.ManagedPath{
			Source: "qt/qt6ct.conf", Target: filepath.Join("qt6ct", "qt6ct.conf"),
		})
	}
	if kvantum {
		out = append(out, adapter.ManagedPath{
			Source: "qt/kvantum.kvconfig", Target: filepath.Join("Kvantum", "kvantum.kvconfig"),
		})
	}
	return out
}

// ReloadMode is new-instances-only: qt5ct and qt6ct are read when a Qt
// application starts.
func (Adapter) ReloadMode() adapter.ReloadMode { return adapter.ReloadNewInstancesOnly }

// Validate checks the INI shape of whichever files were generated.
func (Adapter) Validate(dir string) error {
	for _, name := range []string{"qt5ct.conf", "qt6ct.conf", "kvantum.kvconfig"} {
		if err := validateINI(filepath.Join(dir, "qt", name)); err != nil {
			return err
		}
	}
	return nil
}

func validateINI(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("validate qt: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if strings.HasPrefix(text, "[") {
			if !strings.HasSuffix(text, "]") {
				return fmt.Errorf("validate qt: %s:%d: unterminated section header", path, line)
			}
			continue
		}
		if !strings.Contains(text, "=") {
			return fmt.Errorf("validate qt: %s:%d: not a key=value pair: %q", path, line, text)
		}
	}
	return scanner.Err()
}
