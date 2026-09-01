// Package cli wires the Rice command tree. Commands stay thin: they resolve
// paths, load configuration and delegate to the core packages.
package cli

import (
	"fmt"
	"os"

	rice "github.com/dborovcanin/rice"
	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/adapter/dunst"
	"github.com/dborovcanin/rice/internal/adapter/foot"
	"github.com/dborovcanin/rice/internal/adapter/rofi"
	"github.com/dborovcanin/rice/internal/adapter/sway"
	"github.com/dborovcanin/rice/internal/adapter/swaylock"
	"github.com/dborovcanin/rice/internal/adapter/waybar"
	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/generation"
	"github.com/dborovcanin/rice/internal/render"
	"github.com/dborovcanin/rice/internal/theme"
)

// App holds everything the commands share. It is built once per invocation,
// after flags are parsed, so --root can redirect the whole tree.
type App struct {
	Paths    config.Paths
	Registry *adapter.Registry
	Themes   *theme.Store
	Engine   *render.Engine
	Builder  *generation.Builder
	Store    *generation.Store
}

// NewApp resolves the Rice root and wires the core packages together.
func NewApp(root string) (*App, error) {
	var (
		paths config.Paths
		err   error
	)
	if root != "" {
		paths = config.NewPaths(config.ExpandPath(root))
	} else if paths, err = config.DefaultPaths(); err != nil {
		return nil, err
	}

	registry := adapter.NewRegistry(
		sway.New(),
		waybar.New(),
		rofi.New(),
		foot.New(),
		dunst.New(),
		swaylock.New(),
	)
	engine := render.NewEngine(paths.TemplatesDir, rice.Templates, "templates")
	builder := generation.NewBuilder(engine, registry, rice.Version)

	return &App{
		Paths:    paths,
		Registry: registry,
		Themes:   theme.NewStore(paths.ThemesDir, rice.Themes, "themes"),
		Engine:   engine,
		Builder:  builder,
		Store:    generation.NewStore(paths, builder),
	}, nil
}

// Config loads the user's configuration, falling back to complete defaults
// when config.toml does not exist yet.
func (a *App) Config() (config.Config, error) {
	return config.Load(a.Paths.ConfigFile)
}

// Theme loads a theme by name, or the configured theme when name is empty.
func (a *App) Theme(cfg config.Config, name string) (theme.Theme, error) {
	if name == "" {
		name = cfg.Theme
	}
	return a.Themes.Load(name)
}

// ConfigExists reports whether the user has run `rice init`.
func (a *App) ConfigExists() bool {
	_, err := os.Stat(a.Paths.ConfigFile)
	return err == nil
}

// requireConfig produces a helpful error when Rice has not been initialized.
func (a *App) requireConfig() error {
	if a.ConfigExists() {
		return nil
	}
	return fmt.Errorf("no configuration at %s: run `rice init` first", a.Paths.ConfigFile)
}
