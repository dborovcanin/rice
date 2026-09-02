// Package gtk declares the generated GTK toolkit configuration.
//
// GTK exposes no global icon size, so the theme's icons.size does not reach
// it. What Rice can set is the widget theme, the icon theme, the interface
// font, the cursor and the dark preference — enough for GTK applications to
// stop looking like they belong to a different desktop.
package gtk

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/config"
)

// versions are the GTK generations Rice writes for. Both read the same
// settings.ini keys, so one rendered file serves both.
var versions = []string{"gtk-3.0", "gtk-4.0"}

type Adapter struct{}

// New returns the GTK adapter.
func New() adapter.Adapter { return Adapter{} }

func (Adapter) Name() string { return "gtk" }

// Files is the full set, used when no configuration narrows it.
func (Adapter) Files() []adapter.File {
	return []adapter.File{
		{Template: "gtk/settings.ini.tmpl", Path: "gtk/settings.ini"},
		{Template: "gtk/gtk.css.tmpl", Path: "gtk/gtk.css"},
	}
}

// FilesFor drops the stylesheet when the palette mapping is off.
func (Adapter) FilesFor(cfg config.Config) []adapter.File {
	var files []adapter.File
	if cfg.GTK.Settings {
		files = append(files, adapter.File{
			Template: "gtk/settings.ini.tmpl", Path: "gtk/settings.ini",
		})
	}
	if cfg.GTK.CSS {
		files = append(files, adapter.File{
			Template: "gtk/gtk.css.tmpl", Path: "gtk/gtk.css",
		})
	}
	return files
}

func (Adapter) ConfigPaths() []adapter.ManagedPath {
	return pathsFor(true, true)
}

func (Adapter) ConfigPathsFor(cfg config.Config) []adapter.ManagedPath {
	return pathsFor(cfg.GTK.Settings, cfg.GTK.CSS)
}

// pathsFor links one rendered file into every GTK version directory, because
// GTK 3 and GTK 4 read the same keys from separate locations.
func pathsFor(settings, css bool) []adapter.ManagedPath {
	var out []adapter.ManagedPath
	for _, version := range versions {
		if settings {
			out = append(out, adapter.ManagedPath{
				Source: "gtk/settings.ini",
				Target: filepath.Join(version, "settings.ini"),
			})
		}
		if css {
			out = append(out, adapter.ManagedPath{
				Source: "gtk/gtk.css",
				Target: filepath.Join(version, "gtk.css"),
			})
		}
	}
	return out
}

// ReloadMode is new-instances-only: settings.ini and gtk.css are read at
// startup, so running applications keep the appearance they launched with.
// GTK does watch gtk.css in some builds, but not reliably enough to promise.
func (Adapter) ReloadMode() adapter.ReloadMode { return adapter.ReloadNewInstancesOnly }

// Validate checks the INI shape of settings.ini and the brace balance of the
// stylesheet, whichever were generated.
func (Adapter) Validate(dir string) error {
	if err := validateINI(filepath.Join(dir, "gtk", "settings.ini")); err != nil {
		return err
	}
	return validateCSS(filepath.Join(dir, "gtk", "gtk.css"))
}

func validateINI(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("validate gtk: %w", err)
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
				return fmt.Errorf("validate gtk: %s:%d: unterminated section header", path, line)
			}
			continue
		}
		if !strings.Contains(text, "=") {
			return fmt.Errorf("validate gtk: %s:%d: not a key=value pair: %q", path, line, text)
		}
	}
	return scanner.Err()
}

func validateCSS(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("validate gtk: %w", err)
	}

	if open, closed := strings.Count(string(data), "{"), strings.Count(string(data), "}"); open != closed {
		return fmt.Errorf("validate gtk: %s: %d '{' against %d '}'", path, open, closed)
	}
	return nil
}
