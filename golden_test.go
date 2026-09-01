package rice_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

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

var update = flag.Bool("update", false, "rewrite the golden files")

// goldenThemes are rendered in full against testdata/golden. Together they
// cover the derivation paths: an explicit ANSI palette, opacity below 1, and
// radius/gap variations.
var goldenThemes = []string{"gruvbox-dark", "catppuccin-mocha", "tokyo-night"}

func newBuilder() *generation.Builder {
	engine := render.NewEngine("", rice.Templates, "templates")
	registry := adapter.NewRegistry(
		sway.New(),
		waybar.New(),
		rofi.New(),
		foot.New(),
		dunst.New(),
		swaylock.New(),
	)
	return generation.NewBuilder(engine, registry, rice.Version)
}

// TestGoldenConfigs renders the shipped defaults with each bundled theme and
// compares every generated file byte for byte. Run `go test ./... -update` to
// accept intentional template changes.
func TestGoldenConfigs(t *testing.T) {
	builder := newBuilder()

	cfg := config.DefaultConfig()
	cfg.Sway.Wallpaper = "/home/user/Pictures/wallpaper.jpg"
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config is invalid: %v", err)
	}

	themes := theme.NewStore("", rice.Themes, "themes")

	for _, name := range goldenThemes {
		t.Run(name, func(t *testing.T) {
			th, err := themes.Load(name)
			if err != nil {
				t.Fatalf("load theme: %v", err)
			}

			files, err := builder.Render(cfg, th, 1)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if len(files) == 0 {
				t.Fatal("no files rendered")
			}

			for _, f := range files {
				goldenPath := filepath.Join("testdata", "golden", name, filepath.FromSlash(f.Path))

				if *update {
					if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(goldenPath, f.Content, 0o644); err != nil {
						t.Fatal(err)
					}
					continue
				}

				want, err := os.ReadFile(goldenPath)
				if err != nil {
					t.Fatalf("read golden %s: %v (run: go test ./... -update)", goldenPath, err)
				}
				if string(f.Content) != string(want) {
					t.Errorf("%s differs from %s\n--- got ---\n%s", f.Path, goldenPath, f.Content)
				}
			}
		})
	}
}

// TestGeneratedConfigsValidate renders each bundled theme into a temporary
// directory and runs every adapter's validation over the result, so a template
// cannot ship syntactically broken output.
func TestGeneratedConfigsValidate(t *testing.T) {
	builder := newBuilder()
	themes := theme.NewStore("", rice.Themes, "themes")

	entries, err := themes.List()
	if err != nil {
		t.Fatalf("list themes: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no bundled themes")
	}

	cfg := config.DefaultConfig()
	cfg.Normalize()

	for _, e := range entries {
		t.Run(e.Name, func(t *testing.T) {
			th, err := themes.Load(e.Name)
			if err != nil {
				t.Fatalf("load theme: %v", err)
			}
			if _, err := builder.Build(t.TempDir(), cfg, th, 1, generation.BuildOptions{}); err != nil {
				t.Fatalf("build: %v", err)
			}
		})
	}
}

// TestWaybarDisabledFallsBackToSwayBar covers the branch where Rice generates
// Sway's own bar block instead of a Waybar configuration.
func TestWaybarDisabledFallsBackToSwayBar(t *testing.T) {
	builder := newBuilder()
	themes := theme.NewStore("", rice.Themes, "themes")
	th, err := themes.Load("gruvbox-dark")
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.Components.Waybar = false
	cfg.Normalize()

	files, err := builder.Render(cfg, th, 1)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var swayConfig string
	for _, f := range files {
		if f.Path == "waybar/config.jsonc" {
			t.Error("waybar output rendered while the component is disabled")
		}
		if f.Path == "sway/config" {
			swayConfig = string(f.Content)
		}
	}
	if swayConfig == "" {
		t.Fatal("no sway config rendered")
	}
	for _, want := range []string{"bar {", "status_command", "focused_workspace"} {
		if !contains(swayConfig, want) {
			t.Errorf("sway config should contain %q when waybar is disabled", want)
		}
	}
	if contains(swayConfig, "exec waybar") {
		t.Error("sway config should not start waybar when the component is disabled")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
