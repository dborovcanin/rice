package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/dborovcanin/rice/internal/theme"
)

// Load reads config.toml on top of DefaultConfig, so a user file only has to
// list what it changes. A missing file is not an error: defaults are complete.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		cfg.Normalize()
		return cfg, cfg.Validate()
	case err != nil:
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	if err := Decode(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %s", path, describeTOMLError(err))
	}

	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Decode parses TOML into an existing config, leaving untouched fields alone.
func Decode(data []byte, cfg *Config) error {
	dec := toml.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	return dec.Decode(cfg)
}

// describeTOMLError expands go-toml's strict-mode error, which otherwise says
// only that some field is unknown without naming it.
func describeTOMLError(err error) string {
	var strict *toml.StrictMissingError
	if errors.As(err, &strict) {
		return "unknown field:\n" + strict.String()
	}
	return err.Error()
}

// Marshal renders a config as TOML, which is how `rice init` writes defaults.
func Marshal(cfg Config) ([]byte, error) {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return data, nil
}

// Normalize expands user paths and fills in values templates rely on.
func (c *Config) Normalize() {
	if c.Generations.Keep <= 0 {
		c.Generations.Keep = 10
	}
	if c.Sway.Mod == "" {
		c.Sway.Mod = "Mod4"
	}
	if c.Sway.WallpaperMode == "" {
		c.Sway.WallpaperMode = "fill"
	}
	c.Sway.Wallpaper = ExpandPath(c.Sway.Wallpaper)
	for i := range c.Sway.Outputs {
		out := &c.Sway.Outputs[i]
		out.Wallpaper = ExpandPath(out.Wallpaper)
		if out.Scale == 0 {
			out.Scale = 1
		}
	}

	if c.Waybar.Position == "" {
		c.Waybar.Position = "top"
	}
	if c.Waybar.Layer == "" {
		c.Waybar.Layer = "top"
	}

	c.Swaylock.Image = ExpandPath(c.Swaylock.Image)
	if c.Swaylock.Image == "" {
		c.Swaylock.Image = c.Sway.Wallpaper
	}
	if c.Swaylock.ScalingMode == "" {
		c.Swaylock.ScalingMode = "fill"
	}

	if c.Foot.Term == "" {
		c.Foot.Term = "foot"
	}
	if c.Dunst.Origin == "" {
		c.Dunst.Origin = "top-center"
	}
	if c.Rofi.Width == "" {
		c.Rofi.Width = "30%"
	}
}

// Validate rejects configurations that would render invalid application
// configuration files.
func (c Config) Validate() error {
	var problems []string

	if strings.TrimSpace(c.Theme) == "" {
		problems = append(problems, "theme is empty")
	}
	if !c.Components.Any() {
		problems = append(problems, "no components enabled")
	}

	for i, o := range c.Sway.Outputs {
		if strings.TrimSpace(o.Name) == "" {
			problems = append(problems, fmt.Sprintf("sway.outputs[%d]: name is empty", i))
		}
		if o.Scale < 0 {
			problems = append(problems, fmt.Sprintf("sway.outputs[%d]: scale %.2f is negative", i, o.Scale))
		}
	}
	for i, w := range c.Sway.Workspaces {
		if strings.TrimSpace(w.Key) == "" {
			problems = append(problems, fmt.Sprintf("sway.workspaces[%d]: key is empty", i))
		}
		if strings.TrimSpace(w.Name) == "" {
			problems = append(problems, fmt.Sprintf("sway.workspaces[%d]: name is empty", i))
		}
	}
	for i, b := range c.Sway.Bindings {
		if strings.TrimSpace(b.Keys) == "" || strings.TrimSpace(b.Command) == "" {
			problems = append(problems, fmt.Sprintf("sway.bindings[%d]: keys and command are both required", i))
		}
	}
	for i, m := range c.Sway.Modes {
		if strings.TrimSpace(m.Name) == "" {
			problems = append(problems, fmt.Sprintf("sway.modes[%d]: name is empty", i))
		}
	}
	for i, r := range c.Sway.WindowRules {
		if strings.TrimSpace(r.Criteria) == "" || strings.TrimSpace(r.Commands) == "" {
			problems = append(problems, fmt.Sprintf("sway.window_rules[%d]: criteria and commands are both required", i))
		}
	}
	for i, a := range c.Sway.Assigns {
		if strings.TrimSpace(a.Criteria) == "" || strings.TrimSpace(a.Workspace) == "" {
			problems = append(problems, fmt.Sprintf("sway.assigns[%d]: criteria and workspace are both required", i))
		}
	}

	if c.Components.Waybar && len(c.Waybar.ModulesLeft)+len(c.Waybar.ModulesCenter)+len(c.Waybar.ModulesRight) == 0 {
		problems = append(problems, "waybar is enabled but no modules are configured")
	}
	if c.Foot.ScrollbackLines < 0 {
		problems = append(problems, "foot.scrollback_lines is negative")
	}

	// A border can be turned off, which is theme.BorderNone; anything further
	// below zero is a typo rather than a request.
	for _, b := range []struct {
		name   string
		border Border
	}{
		{"waybar", c.Waybar.Border},
		{"rofi", c.Rofi.Border},
		{"dunst", c.Dunst.Border},
	} {
		if b.border.Width < theme.BorderNone {
			problems = append(problems, fmt.Sprintf("%s.border.width %d is below %d, which is the value for no border",
				b.name, b.border.Width, theme.BorderNone))
		}
		if b.border.Radius < theme.BorderNone {
			problems = append(problems, fmt.Sprintf("%s.border.radius %d is below %d, which is the value for square corners",
				b.name, b.border.Radius, theme.BorderNone))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return &ValidationError{Problems: problems}
}

// Any reports whether at least one component is enabled.
func (c Components) Any() bool {
	return c.Sway || c.Waybar || c.Rofi || c.Foot || c.Dunst || c.Swaylock || c.GTK || c.Qt
}

// Names returns the enabled component names in deployment order.
func (c Components) Names() []string {
	all := []string{"sway", "waybar", "rofi", "foot", "dunst", "swaylock", "gtk", "qt"}
	var out []string
	for _, name := range all {
		if c.Enabled(name) {
			out = append(out, name)
		}
	}
	return out
}

// ValidationError reports every problem found in a configuration at once.
type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	return "config: " + strings.Join(e.Problems, "; ")
}
