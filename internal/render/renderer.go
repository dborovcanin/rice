// Package render turns a theme plus a configuration into concrete application
// configuration file contents. It knows nothing about generations, symlinks or
// the filesystem layout of the desktop.
package render

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/theme"
)

// Context is what every template sees. The semantic parts of the theme are
// promoted to the top level so templates read as `.Colors.Primary` rather than
// `.Theme.Colors.Primary`.
type Context struct {
	Theme    theme.Theme
	Colors   theme.Colors
	Terminal theme.Terminal
	UI       theme.UI
	Fonts    theme.Fonts
	Icons    theme.Icons
	Cursor   theme.Cursor
	GTK      theme.GTK

	// Border is the border the compositor draws: one width, one colour, one
	// radius for the whole desktop. A surface SwayFX does not decorate draws
	// its own frame, and resolves its override against this so the two match.
	Border theme.Border

	Config   config.Config
	Commands config.Commands
	Sway     config.Sway
	Waybar   config.Waybar
	Rofi     config.Rofi
	Foot     config.Foot
	Dunst    config.Dunst
	Swaylock config.Swaylock

	Generation int
	Version    string
	Dark       bool
}

// NewContext assembles the template context from the two sources of truth.
func NewContext(cfg config.Config, th theme.Theme, generation int, version string) Context {
	return Context{
		Theme:    th,
		Colors:   th.Colors,
		Terminal: th.Terminal,
		UI:       th.UI,
		Fonts:    th.Fonts,
		Icons:    th.Icons,
		Cursor:   th.Cursor,
		GTK:      th.GTK,
		Border:   th.Border(),

		Config:   cfg,
		Commands: cfg.Commands,
		Sway:     cfg.Sway,
		Waybar:   cfg.Waybar,
		Rofi:     cfg.Rofi,
		Foot:     cfg.Foot,
		Dunst:    cfg.Dunst,
		Swaylock: cfg.Swaylock,

		Generation: generation,
		Version:    version,
		Dark:       th.IsDark(),
	}
}

// Engine resolves template names against a user directory first and the
// built-in templates second, so any single file can be overridden without
// forking the rest.
type Engine struct {
	// UserDir may be empty or missing; it is checked first.
	UserDir string
	// Builtin holds the bundled templates, rooted at BuiltinRoot.
	Builtin fs.FS
	// BuiltinRoot is the directory inside Builtin holding the templates.
	BuiltinRoot string

	funcs template.FuncMap
}

// NewEngine builds an engine over a user template directory and the embedded
// template FS.
func NewEngine(userDir string, builtin fs.FS, builtinRoot string) *Engine {
	return &Engine{
		UserDir:     userDir,
		Builtin:     builtin,
		BuiltinRoot: builtinRoot,
		funcs:       FuncMap(),
	}
}

// ErrTemplateNotFound is returned when a template exists in neither location.
var ErrTemplateNotFound = errors.New("template not found")

// Render executes the named template, e.g. "foot/foot.ini.tmpl".
func (e *Engine) Render(name string, ctx Context) ([]byte, error) {
	src, origin, err := e.source(name)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(path.Base(name)).
		Funcs(e.funcs).
		Option("missingkey=error").
		Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", origin, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return nil, fmt.Errorf("render template %s: %w", origin, err)
	}
	return buf.Bytes(), nil
}

// Source reports where a template resolves from, which `rice doctor` and
// error messages need in order to be useful.
func (e *Engine) Source(name string) (string, error) {
	_, origin, err := e.source(name)
	return origin, err
}

func (e *Engine) source(name string) (data []byte, origin string, err error) {
	if e.UserDir != "" {
		p := filepath.Join(e.UserDir, filepath.FromSlash(name))
		data, err := os.ReadFile(p)
		if err == nil {
			return data, p, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("read template %s: %w", p, err)
		}
	}

	if e.Builtin != nil {
		p := path.Join(e.BuiltinRoot, name)
		data, err := fs.ReadFile(e.Builtin, p)
		if err == nil {
			return data, "builtin:" + p, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("read builtin template %s: %w", p, err)
		}
	}

	return nil, "", fmt.Errorf("%w: %s", ErrTemplateNotFound, name)
}
