// Package cli wires the Rice command tree. Commands stay thin: they resolve
// paths, load configuration and delegate to the core packages.
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	rice "github.com/dborovcanin/rice"
	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/adapter/dunst"
	"github.com/dborovcanin/rice/internal/adapter/foot"
	"github.com/dborovcanin/rice/internal/adapter/rofi"
	"github.com/dborovcanin/rice/internal/adapter/sway"
	"github.com/dborovcanin/rice/internal/adapter/swaylock"
	"github.com/dborovcanin/rice/internal/adapter/waybar"
	"github.com/dborovcanin/rice/internal/command"
	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/generation"
	"github.com/dborovcanin/rice/internal/ownership"
	"github.com/dborovcanin/rice/internal/reload"
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

	// ConfigDir is where applications keep their configuration, normally
	// ~/.config. Deployment only ever writes below it.
	ConfigDir string
	Runner    command.Runner
	Reload    *reload.Manager
}

// NewApp resolves the Rice root and wires the core packages together.
func NewApp(root, configDir string) (*App, error) {
	var (
		paths config.Paths
		err   error
	)
	if root != "" {
		paths = config.NewPaths(config.ExpandPath(root))
	} else if paths, err = config.DefaultPaths(); err != nil {
		return nil, err
	}

	if configDir == "" {
		configDir, err = DefaultConfigDir()
		if err != nil {
			return nil, err
		}
	} else {
		configDir = config.ExpandPath(configDir)
	}

	runner := command.New()
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
		Paths:     paths,
		Registry:  registry,
		Themes:    theme.NewStore(paths.ThemesDir, rice.Themes, "themes"),
		Engine:    engine,
		Builder:   builder,
		Store:     generation.NewStore(paths, builder),
		ConfigDir: configDir,
		Runner:    runner,
		Reload:    reload.NewWith(runner),
	}, nil
}

// DefaultConfigDir resolves where applications keep their configuration:
// $XDG_CONFIG_HOME, else ~/.config.
func DefaultConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config"), nil
}

// ManifestPath is where the adoption manifest lives.
func (a *App) ManifestPath() string {
	return filepath.Join(a.Paths.StateDir, ownership.ManifestName)
}

// Manifest loads the adoption manifest, which is empty when nothing has been
// adopted yet.
func (a *App) Manifest() (ownership.Manifest, error) {
	return ownership.LoadManifest(a.ManifestPath())
}

// Plan works out what deploying the enabled components would do. It modifies
// nothing.
func (a *App) Plan(cfg config.Config) (ownership.Plan, error) {
	adapters, err := a.Registry.Select(cfg.Components.Names())
	if err != nil {
		return ownership.Plan{}, err
	}
	return ownership.BuildPlan(adapters, a.Paths, a.ConfigDir)
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
