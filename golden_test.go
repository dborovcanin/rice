package rice_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	rice "github.com/dborovcanin/rice"
	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/adapter/dunst"
	"github.com/dborovcanin/rice/internal/adapter/foot"
	"github.com/dborovcanin/rice/internal/adapter/gtk"
	"github.com/dborovcanin/rice/internal/adapter/qt"
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

// hexColor finds a spelled-out colour. Waybar selectors start with "#" too,
// but no module is named in hex.
var hexColor = regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b`)

// goldenThemes are rendered in full against testdata/golden. Together they
// cover the derivation paths: an explicit ANSI palette, opacity below 1,
// radius/gap variations, and a light variant — which takes a different branch
// wherever a template asks whether the theme is dark.
//
// Every bundled theme is checked by TestEveryBundledThemeIsUsable and handed
// to the real validators; goldens are for reading diffs, not for coverage.
var goldenThemes = []string{"gruvbox-dark", "catppuccin-mocha", "tokyo-night", "catppuccin-latte"}

func newBuilder() *generation.Builder {
	engine := render.NewEngine("", rice.Templates, "templates")
	registry := adapter.NewRegistry(
		sway.New(),
		waybar.New(),
		rofi.New(),
		foot.New(),
		dunst.New(),
		swaylock.New(),
		gtk.New(),
		qt.New(),
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

// TestOptionalToolkitFiles renders the toolkit files the default configuration
// leaves out. Without this, the templates behind an off-by-default setting
// could rot unnoticed: the golden run never reaches them.
func TestOptionalToolkitFiles(t *testing.T) {
	builder := newBuilder()

	cfg := config.DefaultConfig()
	cfg.GTK.CSS = true
	cfg.GTK.ExtraCSS = "/* appended */"
	cfg.Normalize()

	themes := theme.NewStore("", rice.Themes, "themes")
	th, err := themes.Load("tokyo-night")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}

	dir := t.TempDir()
	if _, err := builder.Build(dir, cfg, th, 1, generation.BuildOptions{}); err != nil {
		t.Fatalf("build: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "gtk", "gtk.css"))
	if err != nil {
		t.Fatalf("read gtk.css: %v", err)
	}
	css := string(data)

	// The stylesheet exists to carry the palette into libadwaita, so the
	// palette had better be in it.
	for _, want := range []string{
		"@define-color accent_color " + th.Colors.Primary.String(),
		"@define-color window_bg_color " + th.Colors.Background.String(),
		"/* appended */",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("gtk.css is missing %q", want)
		}
	}
}

// TestEnvironmentFileCarriesTheCursor guards the one generated file that
// affects the whole session rather than a single application.
func TestEnvironmentFileCarriesTheCursor(t *testing.T) {
	builder := newBuilder()

	cfg := config.DefaultConfig()
	cfg.Sway.Environment = map[string]string{"MOZ_ENABLE_WAYLAND": "1"}
	cfg.Normalize()

	themes := theme.NewStore("", rice.Themes, "themes")
	th, err := themes.Load("gruvbox-dark")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}

	dir := t.TempDir()
	if _, err := builder.Build(dir, cfg, th, 1, generation.BuildOptions{}); err != nil {
		t.Fatalf("build: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "sway", "environment.conf"))
	if err != nil {
		t.Fatalf("read environment.conf: %v", err)
	}
	env := string(data)

	for _, want := range []string{
		"XCURSOR_THEME=" + th.Cursor.Theme,
		"XCURSOR_SIZE=28",
		"QT_QPA_PLATFORMTHEME=qt5ct",
		"MOZ_ENABLE_WAYLAND=1",
	} {
		if !strings.Contains(env, want) {
			t.Errorf("environment.conf is missing %q", want)
		}
	}

	// Turning the Qt component off must take its variable with it, or the
	// session points at a platform theme with no configuration behind it.
	cfg.Components.Qt = false
	dir = t.TempDir()
	if _, err := builder.Build(dir, cfg, th, 1, generation.BuildOptions{}); err != nil {
		t.Fatalf("build: %v", err)
	}
	data, _ = os.ReadFile(filepath.Join(dir, "sway", "environment.conf"))
	if strings.Contains(string(data), "QT_QPA_PLATFORMTHEME") {
		t.Error("QT_QPA_PLATFORMTHEME survived turning the Qt component off")
	}
}

// A theme that names no icon or cursor set must not produce settings with
// nothing after the "=". An empty value is not the same as an absent one:
// GTK and the session environment both take it literally.
func TestSparseThemeWritesNoEmptyValues(t *testing.T) {
	builder := newBuilder()

	cfg := config.DefaultConfig()
	cfg.Normalize()

	th, err := theme.Parse([]byte(`
name = "sparse"

[colors]
background = "#101010"
foreground = "#e0e0e0"
primary = "#5599ff"
`), "sparse.toml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if th.Icons.Theme != "" || th.Cursor.Theme != "" {
		t.Fatal("this test needs a theme that names neither an icon nor a cursor theme")
	}

	dir := t.TempDir()
	if _, err := builder.Build(dir, cfg, th, 1, generation.BuildOptions{}); err != nil {
		t.Fatalf("build: %v", err)
	}

	for _, rel := range []string{
		"gtk/settings.ini", "qt/qt5ct.conf", "sway/environment.conf",
	} {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "[") {
				continue
			}
			if strings.HasSuffix(trimmed, "=") {
				t.Errorf("%s:%d has an empty value: %q", rel, i+1, trimmed)
			}
		}
	}
}

// validators are the applications that can check their own configuration.
// Rice's own validation catches the shape of a file; only the application
// itself knows whether a key still exists.
var validators = []struct {
	binary string
	file   string
	args   func(path string) []string
}{
	{"foot", "foot/foot.ini", func(p string) []string { return []string{"--check-config", "-c", p} }},
	{"sway", "sway/config", func(p string) []string { return []string{"-C", "-c", p} }},
	{"rofi", "rofi/config.rasi", func(p string) []string { return []string{"-config", p, "-dump-config"} }},
}

// TestGeneratedConfigsPassTheirOwnValidators hands each generated file to the
// application that reads it.
//
// This exists because two deprecations shipped unnoticed: foot moved the ANSI
// palette from [colors] to [colors-dark] and dropped [cursor].color, and
// nothing in Rice could have known. Golden files only prove the output has not
// changed, not that it is still correct.
func TestGeneratedConfigsPassTheirOwnValidators(t *testing.T) {
	builder := newBuilder()

	cfg := config.DefaultConfig()
	// The wallpaper is a fixture path that does not exist, and sway rejects a
	// background it cannot read. That is a missing file, not a bad config.
	cfg.Sway.Wallpaper = ""
	for i := range cfg.Sway.Outputs {
		cfg.Sway.Outputs[i].Wallpaper = ""
	}
	cfg.Normalize()

	themes := theme.NewStore("", rice.Themes, "themes")

	for _, name := range bundledThemes(t) {
		t.Run(name, func(t *testing.T) {
			th, err := themes.Load(name)
			if err != nil {
				t.Fatalf("load theme: %v", err)
			}

			dir := t.TempDir()
			if _, err := builder.Build(dir, cfg, th, 1, generation.BuildOptions{}); err != nil {
				t.Fatalf("build: %v", err)
			}

			for _, v := range validators {
				t.Run(v.binary, func(t *testing.T) {
					path, err := exec.LookPath(v.binary)
					if err != nil {
						t.Skipf("%s is not installed", v.binary)
					}

					target := filepath.Join(dir, filepath.FromSlash(v.file))
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()

					out, err := exec.CommandContext(ctx, path, v.args(target)...).CombinedOutput()

					// Sway reports a bad config through its output rather than
					// its exit status, so both are checked.
					if err != nil || bytes.Contains(out, []byte("[ERROR]")) {
						t.Errorf("%s rejected the generated configuration: %v\n%s", v.binary, err, out)
					}
				})
			}
		})
	}
}

// A launcher is read at a glance from across the screen, so it often wants to
// be larger than the rest of the desktop. Its font and icon size override the
// theme's, and fall back to them when unset.
func TestRofiFontAndIconSizeOverrideTheTheme(t *testing.T) {
	builder := newBuilder()
	themes := theme.NewStore("", rice.Themes, "themes")
	th, err := themes.Load("gruvbox-dark")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}

	render := func(t *testing.T, cfg config.Config) string {
		t.Helper()
		cfg.Normalize()
		files, err := builder.Render(cfg, th, 1)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		for _, f := range files {
			if f.Path == "rofi/config.rasi" {
				return string(f.Content)
			}
		}
		t.Fatal("rofi produced no configuration")
		return ""
	}

	// Unset, the launcher follows the theme.
	out := render(t, config.DefaultConfig())
	if want := fmt.Sprintf("%q", th.Fonts.UIFamily+" "+strconv.Itoa(th.Fonts.UISize)); !strings.Contains(out, want) {
		t.Errorf("font should fall back to the theme's %s:\n%s", want, firstLines(out, 20))
	}
	if want := fmt.Sprintf("%dpx", th.Icons.Size); !strings.Contains(out, want) {
		t.Errorf("icon size should fall back to the theme's %s", want)
	}

	// Set, they win.
	cfg := config.DefaultConfig()
	cfg.Rofi.FontFamily = "Iosevka"
	cfg.Rofi.FontSize = 22
	cfg.Rofi.IconSize = 48

	out = render(t, cfg)
	if !strings.Contains(out, `"Iosevka 22"`) {
		t.Errorf("the launcher font override was ignored:\n%s", firstLines(out, 20))
	}
	if !strings.Contains(out, "48px") {
		t.Error("the launcher icon size override was ignored")
	}
	if strings.Contains(out, fmt.Sprintf("%dpx", th.Icons.Size)) {
		t.Error("the theme's icon size is still in the output")
	}
}

// Every design has to render, in every theme, and produce a bar configuration
// Waybar can read. A design is shape only: none of them may name a colour that
// did not come from the theme.
func TestEveryWaybarDesignRendersInEveryTheme(t *testing.T) {
	builder := newBuilder()
	themes := theme.NewStore("", rice.Themes, "themes")
	bar := waybar.New()

	designs := config.WaybarDesigns()
	if len(designs) < 10 {
		t.Fatalf("only %d designs; the point of the set is that there is a choice", len(designs))
	}

	entries, err := themes.List()
	if err != nil {
		t.Fatalf("list themes: %v", err)
	}

	for _, design := range designs {
		for _, entry := range entries {
			t.Run(design.Name+"/"+entry.Name, func(t *testing.T) {
				th, err := themes.Load(entry.Name)
				if err != nil {
					t.Fatalf("load theme: %v", err)
				}

				cfg := config.DefaultConfig()
				cfg.Waybar.Design = design.Name
				cfg.Normalize()
				if err := cfg.Validate(); err != nil {
					t.Fatalf("a bundled design should be a valid setting: %v", err)
				}

				files, err := builder.Render(cfg, th, 1)
				if err != nil {
					t.Fatalf("render: %v", err)
				}

				dir := t.TempDir()
				style := ""
				for _, f := range files {
					out := filepath.Join(dir, filepath.FromSlash(f.Path))
					if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(out, f.Content, 0o644); err != nil {
						t.Fatal(err)
					}
					if f.Path == "waybar/style.css" {
						style = string(f.Content)
					}
				}

				// The bar configuration has to be readable as JSON, which is
				// what Waybar will do with it.
				if err := bar.Validate(dir); err != nil {
					t.Errorf("%s: %v", design.Name, err)
				}

				if len(style) < 500 {
					t.Errorf("%s: stylesheet is %d bytes, which is not a design", design.Name, len(style))
				}
				if strings.Contains(style, "{{") {
					t.Errorf("%s: stylesheet still holds template syntax", design.Name)
				}
				// Colours come from the theme, so the only hex literals in a
				// stylesheet are the ones the @define-color block was given.
				for _, line := range strings.Split(style, "\n") {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "@define-color") {
						continue
					}
					for _, hex := range hexColor.FindAllString(trimmed, -1) {
						// `contrast` resolves to black or white, which is a
						// reading of the theme's colour rather than a colour
						// of the design's own.
						if strings.EqualFold(hex, "#000000") || strings.EqualFold(hex, "#ffffff") {
							continue
						}
						t.Errorf("%s: %q names a colour outside the theme block", design.Name, trimmed)
					}
				}
			})
		}
	}
}

// The registry and the template directory are two halves of the same fact, and
// a design in one and not the other is a template that never renders or a name
// that fails to.
func TestWaybarDesignsMatchTheirTemplates(t *testing.T) {
	entries, err := fs.ReadDir(rice.Templates, "templates/waybar/designs")
	if err != nil {
		t.Fatalf("read designs: %v", err)
	}

	styles := map[string]bool{}
	layouts := map[string]bool{}
	for _, e := range entries {
		switch name, ok := strings.CutSuffix(e.Name(), ".css.tmpl"); {
		case ok:
			styles[name] = true
		default:
			name, ok := strings.CutSuffix(e.Name(), ".jsonc.tmpl")
			if !ok {
				t.Errorf("%s is neither a stylesheet nor a layout", e.Name())
				continue
			}
			layouts[name] = true
		}
	}

	for _, d := range config.WaybarDesigns() {
		if !styles[d.Name] {
			t.Errorf("design %q has no stylesheet", d.Name)
		}
		delete(styles, d.Name)

		if d.Layout != layouts[d.Name] {
			t.Errorf("design %q says Layout=%v but has layout template=%v", d.Name, d.Layout, layouts[d.Name])
		}
		delete(layouts, d.Name)
	}
	for name := range styles {
		t.Errorf("stylesheet %q belongs to no design and can never be selected", name)
	}
	for name := range layouts {
		t.Errorf("layout %q belongs to no design", name)
	}
}

// A terminal is read for hours at a time, so its font overrides the theme's
// monospaced one, and falls back to it when unset.
func TestFootFontOverridesTheTheme(t *testing.T) {
	builder := newBuilder()
	themes := theme.NewStore("", rice.Themes, "themes")
	th, err := themes.Load("gruvbox-dark")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}

	render := func(t *testing.T, cfg config.Config) string {
		t.Helper()
		cfg.Normalize()
		files, err := builder.Render(cfg, th, 1)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		for _, f := range files {
			if f.Path == "foot/foot.ini" {
				return string(f.Content)
			}
		}
		t.Fatal("foot produced no configuration")
		return ""
	}

	// Unset, the terminal follows the theme's monospaced font.
	out := render(t, config.DefaultConfig())
	if want := fmt.Sprintf("font=%s:size=%d", th.Fonts.MonoFamily, th.Fonts.MonoSize); !strings.Contains(out, want) {
		t.Errorf("font should fall back to the theme's %q:\n%s", want, firstLines(out, 20))
	}

	// Set, it wins.
	cfg := config.DefaultConfig()
	cfg.Foot.FontFamily = "Iosevka"
	cfg.Foot.FontSize = 15

	out = render(t, cfg)
	if !strings.Contains(out, "font=Iosevka:size=15") {
		t.Errorf("the terminal font override was ignored:\n%s", firstLines(out, 20))
	}
}

// A notification is read from wherever you happen to be looking, so its font
// overrides the theme's, and falls back to it when unset.
func TestDunstFontOverridesTheTheme(t *testing.T) {
	builder := newBuilder()
	themes := theme.NewStore("", rice.Themes, "themes")
	th, err := themes.Load("gruvbox-dark")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}

	render := func(t *testing.T, cfg config.Config) string {
		t.Helper()
		cfg.Normalize()
		files, err := builder.Render(cfg, th, 1)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		for _, f := range files {
			if f.Path == "dunst/dunstrc" {
				return string(f.Content)
			}
		}
		t.Fatal("dunst produced no configuration")
		return ""
	}

	// Unset, notifications follow the theme.
	out := render(t, config.DefaultConfig())
	if want := "font = " + th.Fonts.UIFamily + " " + strconv.Itoa(th.Fonts.UISize); !strings.Contains(out, want) {
		t.Errorf("font should fall back to the theme's %q:\n%s", want, firstLines(out, 40))
	}

	// Set, it wins.
	cfg := config.DefaultConfig()
	cfg.Dunst.FontFamily = "Iosevka"
	cfg.Dunst.FontSize = 18

	out = render(t, cfg)
	if !strings.Contains(out, "font = Iosevka 18") {
		t.Errorf("the notification font override was ignored:\n%s", firstLines(out, 40))
	}
}

// The desktop has one border, and everything that draws its own frame follows
// it. An application that draws its own can be given a different one; a window
// SwayFX decorates cannot, because the compositor draws that border itself.
func TestBorderPropagatesFromTheCompositorAndCanBeOverridden(t *testing.T) {
	builder := newBuilder()
	themes := theme.NewStore("", rice.Themes, "themes")
	th, err := themes.Load("gruvbox-dark")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}

	render := func(t *testing.T, cfg config.Config) map[string]string {
		t.Helper()
		cfg.Normalize()
		files, err := builder.Render(cfg, th, 1)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		out := map[string]string{}
		for _, f := range files {
			out[f.Path] = string(f.Content)
		}
		return out
	}

	wm := th.Border()

	// The bar is asked for the design that draws both a border and a radius:
	// the default one is flush segments, which has neither to check.
	bordered := config.DefaultConfig()
	bordered.Waybar.Design = "pills"
	files := render(t, bordered)

	// Unset, every self-decorated surface draws the compositor's border.
	for _, c := range []struct{ path, want string }{
		{"sway/config", fmt.Sprintf("pixel %d", wm.Width)},
		{"sway/config", fmt.Sprintf("corner_radius %d", wm.Radius)},
		{"rofi/config.rasi", fmt.Sprintf("border:           %dpx", wm.Width)},
		{"rofi/config.rasi", fmt.Sprintf("border-color:     %s", wm.Focus)},
		{"rofi/config.rasi", fmt.Sprintf("border-radius:    %dpx", wm.Radius)},
		{"dunst/dunstrc", fmt.Sprintf("frame_width = %d", wm.Width)},
		{"dunst/dunstrc", fmt.Sprintf("frame_color = %q", wm.Color)},
		{"dunst/dunstrc", fmt.Sprintf("corner_radius = %d", wm.Radius)},
		{"waybar/style.css", fmt.Sprintf("@define-color border %s;", wm.Focus)},
		{"waybar/style.css", fmt.Sprintf("border-radius: %dpx", wm.Radius)},
	} {
		if !strings.Contains(files[c.path], c.want) {
			t.Errorf("%s should follow the desktop's border %q:\n%s", c.path, c.want, firstLines(files[c.path], 40))
		}
	}

	// Set, each surface draws its own and the compositor keeps the desktop's.
	cfg := config.DefaultConfig()
	cfg.Waybar.Design = "pills"
	cfg.Rofi.Border = config.Border{Width: 6, Color: theme.MustParseColor("#ff0000"), Radius: 20}
	cfg.Dunst.Border = config.Border{Width: 1, Color: theme.MustParseColor("#00ff00"), Radius: 2}
	cfg.Waybar.Border = config.Border{Width: 4, Color: theme.MustParseColor("#0000ff"), Radius: 9}

	files = render(t, cfg)
	for _, c := range []struct{ path, want string }{
		{"rofi/config.rasi", "border:           6px"},
		{"rofi/config.rasi", "border-color:     #ff0000"},
		{"rofi/config.rasi", "border-radius:    20px"},
		{"dunst/dunstrc", "frame_width = 1"},
		{"dunst/dunstrc", `frame_color = "#00ff00"`},
		{"dunst/dunstrc", "corner_radius = 2"},
		{"waybar/style.css", "@define-color border #0000ff;"},
		{"waybar/style.css", "border: 4px solid @border;"},
		{"waybar/style.css", "border-radius: 9px"},
		// The compositor is not one of them: it is where the border comes from.
		{"sway/config", fmt.Sprintf("pixel %d", wm.Width)},
		{"sway/config", fmt.Sprintf("corner_radius %d", wm.Radius)},
	} {
		if !strings.Contains(files[c.path], c.want) {
			t.Errorf("%s missing %q:\n%s", c.path, c.want, firstLines(files[c.path], 40))
		}
	}

	// A border can be turned off, which zero cannot say because zero already
	// means "follow the desktop".
	cfg = config.DefaultConfig()
	cfg.Rofi.Border = config.Border{Width: theme.BorderNone, Radius: theme.BorderNone}
	files = render(t, cfg)
	for _, c := range []struct{ path, want string }{
		{"rofi/config.rasi", "border:           0px"},
		{"rofi/config.rasi", "border-radius:    0px"},
	} {
		if !strings.Contains(files[c.path], c.want) {
			t.Errorf("%s missing %q:\n%s", c.path, c.want, firstLines(files[c.path], 40))
		}
	}

	// And the desktop can lose its border, which reaches Sway as "none"
	// rather than as a border zero pixels wide.
	bare := th
	bare.UI.BorderWidth = theme.BorderNone
	files = func() map[string]string {
		cfg := config.DefaultConfig()
		cfg.Normalize()
		rendered, err := builder.Render(cfg, bare, 1)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		out := map[string]string{}
		for _, f := range rendered {
			out[f.Path] = string(f.Content)
		}
		return out
	}()
	if !strings.Contains(files["sway/config"], "default_border none") {
		t.Errorf("sway should draw no border:\n%s", firstLines(files["sway/config"], 60))
	}
	if !strings.Contains(files["dunst/dunstrc"], "frame_width = 0") {
		t.Error("notifications should follow the desktop into having no frame")
	}

	// A terminal is an ordinary window: SwayFX draws its border, so nothing in
	// its configuration mentions one.
	for _, word := range []string{"border", "corner_radius"} {
		if strings.Contains(files["foot/foot.ini"], word) {
			t.Errorf("foot.ini mentions %q; a decorated window's border is the compositor's", word)
		}
	}
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// contrastRatio is the WCAG ratio between two colours, from 1 to 21.
func contrastRatio(a, b theme.Color) float64 {
	lum := func(c theme.Color) float64 {
		channel := func(v uint8) float64 {
			f := float64(v) / 255
			if f <= 0.04045 {
				return f / 12.92
			}
			return math.Pow((f+0.055)/1.055, 2.4)
		}
		return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
	}

	la, lb := lum(a), lum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// bundledThemes is every theme Rice ships.
func bundledThemes(t *testing.T) []string {
	t.Helper()

	entries, err := theme.NewStore("", rice.Themes, "themes").List()
	if err != nil {
		t.Fatalf("list themes: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	if len(names) == 0 {
		t.Fatal("no bundled themes")
	}
	return names
}

// TestEveryBundledThemeIsUsable holds every shipped theme to the same standard,
// so adding one cannot quietly ship a palette nobody can read.
func TestEveryBundledThemeIsUsable(t *testing.T) {
	builder := newBuilder()
	cfg := config.DefaultConfig()
	cfg.Normalize()

	themes := theme.NewStore("", rice.Themes, "themes")

	for _, name := range bundledThemes(t) {
		t.Run(name, func(t *testing.T) {
			th, err := themes.Load(name)
			if err != nil {
				t.Fatalf("load: %v", err)
			}

			// The variant has to match the background, or every consumer that
			// branches on it — foot's colour section, the GTK dark hint —
			// picks the wrong side.
			wantDark := th.Colors.Background.IsDark()
			if got := th.Variant == "dark"; got != wantDark {
				t.Errorf("variant is %q but the background %s is dark=%v",
					th.Variant, th.Colors.Background, wantDark)
			}
			if th.IsDark() != wantDark {
				t.Errorf("IsDark() = %v, want %v", th.IsDark(), wantDark)
			}

			// Text has to be readable against its background. 4.5:1 is the
			// WCAG AA threshold for body text; the AAA threshold of 7 would
			// exclude palettes like Solarized that are deliberately low
			// contrast, which is a design choice rather than a defect.
			if got := contrastRatio(th.Colors.Foreground, th.Colors.Background); got < 4.5 {
				t.Errorf("foreground/background contrast = %.2f, want at least 4.5", got)
			}
			if got := contrastRatio(th.Terminal.Foreground, th.Terminal.Background); got < 4.5 {
				t.Errorf("terminal contrast = %.2f, want at least 4.5", got)
			}

			// Surfaces have to be distinguishable from the background, or the
			// desktop is one flat sheet. They must also not overshoot into
			// being unreadable.
			for label, c := range map[string]theme.Color{
				"surface": th.Colors.Surface, "surface_alt": th.Colors.SurfaceAlt,
			} {
				if c == th.Colors.Background {
					t.Errorf("%s is identical to the background", label)
				}
				// Surfaces carry chrome and short labels rather than body
				// text, so the large-text threshold applies.
				if got := contrastRatio(th.Colors.Foreground, c); got < 3.5 {
					t.Errorf("foreground on %s = %.2f, want at least 3.5", label, got)
				}
			}

			// Muted text is meant to recede, not to vanish.
			if got := contrastRatio(th.Colors.Muted, th.Colors.Background); got < 1.8 {
				t.Errorf("muted contrast = %.2f, want at least 1.8", got)
			}

			// Every accent has to be visible against the background.
			for label, c := range map[string]theme.Color{
				"primary": th.Colors.Primary, "secondary": th.Colors.Secondary,
				"accent": th.Colors.Accent, "success": th.Colors.Success,
				"warning": th.Colors.Warning, "error": th.Colors.Error,
			} {
				// Accents mark and indicate rather than carry text, and a
				// yellow on a light background is inherently around 2:1 —
				// true of every light palette, not a fault of these. The bar
				// is set to catch invisible, not to enforce readable.
				if got := contrastRatio(c, th.Colors.Background); got < 1.8 {
					t.Errorf("%s (%s) on the background = %.2f, want at least 1.8", label, c, got)
				}
			}

			// And it has to render.
			dir := t.TempDir()
			if _, err := builder.Build(dir, cfg, th, 1, generation.BuildOptions{}); err != nil {
				t.Fatalf("build: %v", err)
			}
		})
	}
}
