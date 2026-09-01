package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Theme != DefaultConfig().Theme {
		t.Errorf("theme = %q, want the default", cfg.Theme)
	}
	if len(cfg.Sway.Bindings) == 0 {
		t.Error("defaults should include key bindings")
	}
}

func TestLoadMergesOntoDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	src := `
theme = "tokyo-night"

[components]
waybar = false

[sway]
mod = "Mod1"

[foot]
scrollback_lines = 1000
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Theme != "tokyo-night" {
		t.Errorf("theme = %q", cfg.Theme)
	}
	if cfg.Components.Waybar {
		t.Error("waybar should be disabled")
	}
	if !cfg.Components.Sway {
		t.Error("unspecified components should keep their default")
	}
	if cfg.Sway.Mod != "Mod1" {
		t.Errorf("mod = %q", cfg.Sway.Mod)
	}
	if cfg.Foot.ScrollbackLines != 1000 {
		t.Errorf("scrollback = %d", cfg.Foot.ScrollbackLines)
	}
	if len(cfg.Sway.Bindings) == 0 {
		t.Error("default bindings should survive a partial config")
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("theme = \"x\"\nkolor = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "kolor") {
		t.Errorf("error should name the unknown field, got: %v", err)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{
			name:   "empty theme",
			mutate: func(c *Config) { c.Theme = "" },
			want:   "theme is empty",
		},
		{
			name:   "no components",
			mutate: func(c *Config) { c.Components = Components{} },
			want:   "no components enabled",
		},
		{
			name:   "output without name",
			mutate: func(c *Config) { c.Sway.Outputs = []Output{{Position: "0 0"}} },
			want:   "sway.outputs[0]",
		},
		{
			name:   "binding without command",
			mutate: func(c *Config) { c.Sway.Bindings = []Binding{{Keys: "$mod+q"}} },
			want:   "sway.bindings[0]",
		},
		{
			name:   "assign without workspace",
			mutate: func(c *Config) { c.Sway.Assigns = []Assign{{Criteria: "[app_id=\"x\"]"}} },
			want:   "sway.assigns[0]",
		},
		{
			name: "waybar without modules",
			mutate: func(c *Config) {
				c.Waybar.ModulesLeft = nil
				c.Waybar.ModulesCenter = nil
				c.Waybar.ModulesRight = nil
			},
			want: "no modules",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.mutate(&cfg)
			cfg.Normalize()

			err := cfg.Validate()
			if err == nil {
				t.Fatal("want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("shipped defaults are invalid: %v", err)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	data, err := Marshal(DefaultConfig())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("reload marshalled config: %v", err)
	}
	want := DefaultConfig()
	want.Normalize()
	if len(cfg.Sway.Bindings) != len(want.Sway.Bindings) {
		t.Errorf("bindings = %d, want %d", len(cfg.Sway.Bindings), len(want.Sway.Bindings))
	}
	if len(cfg.Waybar.Modules) != len(want.Waybar.Modules) {
		t.Errorf("waybar modules = %d, want %d", len(cfg.Waybar.Modules), len(want.Waybar.Modules))
	}
}

func TestComponentsNames(t *testing.T) {
	c := Components{Sway: true, Foot: true}
	got := c.Names()
	want := []string{"sway", "foot"}
	if len(got) != len(want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Names() = %v, want %v", got, want)
		}
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"~/Pictures/bg.jpg", filepath.Join(home, "Pictures/bg.jpg")},
		{"~", home},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}
	for _, tt := range tests {
		if got := ExpandPath(tt.in); got != tt.want {
			t.Errorf("ExpandPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPathsDerivation(t *testing.T) {
	p := NewPaths("/rice")
	if p.ConfigFile != "/rice/config.toml" {
		t.Errorf("ConfigFile = %q", p.ConfigFile)
	}
	if p.Generation(42) != "/rice/generations/000042" {
		t.Errorf("Generation(42) = %q", p.Generation(42))
	}
	if p.Current != "/rice/current" {
		t.Errorf("Current = %q", p.Current)
	}
}
