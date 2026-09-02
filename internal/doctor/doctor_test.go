package doctor_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dborovcanin/rice/internal/command"
	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/doctor"
	"github.com/dborovcanin/rice/internal/theme"
)

// fcRunner answers fc-list with a fixed set of families.
type fcRunner struct {
	*command.Fake
	families string
}

func (r *fcRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	if _, err := r.Fake.Output(ctx, name, args...); err != nil {
		return nil, err
	}
	return []byte(r.families), nil
}

func newRunner() *fcRunner {
	return &fcRunner{Fake: command.NewFake(), families: "Inter\nIosevka\n"}
}

// dataHome points the XDG lookups at a temporary tree and creates the given
// directories inside it, so asset checks run against a known filesystem.
func dataHome(t *testing.T, dirs ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	t.Setenv("XDG_DATA_HOME", root)
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "empty"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("HOME", root)
	return root
}

func find(checks []doctor.Check, name string) (doctor.Check, bool) {
	for _, c := range checks {
		if c.Name == name {
			return c, true
		}
	}
	return doctor.Check{}, false
}

func TestAssetsFindInstalledThemes(t *testing.T) {
	dataHome(t,
		"icons/Papirus-Dark",
		"icons/Bibata/cursors",
		"themes/Orchis",
		"Kvantum/KvMine",
	)

	th := theme.Theme{
		Fonts:  theme.Fonts{UIFamily: "Inter", MonoFamily: "Iosevka", BarFamily: "Iosevka"},
		Icons:  theme.Icons{Theme: "Papirus-Dark"},
		Cursor: theme.Cursor{Theme: "Bibata", Size: 24},
		GTK:    theme.GTK{Theme: "Orchis", KvantumTheme: "KvMine"},
	}
	cfg := config.Config{
		Components: config.Components{GTK: true, Qt: true},
		Qt:         config.Qt{Kvantum: true},
	}

	checks := doctor.Assets(context.Background(), newRunner(), th, cfg)
	if got := doctor.Worst(checks); got != doctor.LevelOK {
		for _, c := range checks {
			t.Logf("%s %s = %s: %s", c.Level.Marker(), c.Name, c.Value, c.Detail)
		}
		t.Errorf("worst level = %s, want ok", got)
	}
}

func TestAssetsReportWhatIsMissing(t *testing.T) {
	dataHome(t)

	th := theme.Theme{
		Fonts:  theme.Fonts{UIFamily: "Nonesuch", MonoFamily: "Iosevka", BarFamily: "Iosevka"},
		Icons:  theme.Icons{Theme: "Absent-Icons"},
		Cursor: theme.Cursor{Theme: "Absent-Cursor", Size: 24},
	}

	checks := doctor.Assets(context.Background(), newRunner(), th, config.Config{})

	ui, ok := find(checks, "ui font")
	if !ok || ui.Level != doctor.LevelWarn {
		t.Errorf("ui font = %+v, want a warning", ui)
	}
	if !strings.Contains(ui.Detail, "not installed") {
		t.Errorf("ui font detail = %q", ui.Detail)
	}

	mono, _ := find(checks, "mono font")
	if mono.Level != doctor.LevelOK {
		t.Errorf("mono font = %+v, want ok", mono)
	}

	for _, name := range []string{"icon theme", "cursor theme"} {
		c, ok := find(checks, name)
		if !ok || c.Level != doctor.LevelWarn {
			t.Errorf("%s = %+v, want a warning", name, c)
		}
	}
}

// A generic family is a fontconfig alias, not something that can be installed,
// so asking whether it exists is meaningless and must not warn.
func TestAssetsAcceptGenericFamilies(t *testing.T) {
	dataHome(t)

	th := theme.Theme{
		Fonts: theme.Fonts{UIFamily: "sans-serif", MonoFamily: "monospace", BarFamily: "monospace"},
	}
	for _, c := range doctor.Assets(context.Background(), newRunner(), th, config.Config{}) {
		if strings.HasSuffix(c.Name, "font") && c.Level != doctor.LevelOK {
			t.Errorf("%s = %+v, want ok", c.Name, c)
		}
	}
}

// Adwaita ships inside GTK rather than as a directory, and is the default on
// most systems: warning about it would flag the most likely correct answer.
func TestAssetsAcceptBuiltinGTKThemes(t *testing.T) {
	dataHome(t)

	th := theme.Theme{
		Fonts: theme.Fonts{UIFamily: "Inter", MonoFamily: "Iosevka", BarFamily: "Iosevka"},
		GTK:   theme.GTK{Theme: "Adwaita-dark"},
	}
	cfg := config.Config{Components: config.Components{GTK: true}}

	c, ok := find(doctor.Assets(context.Background(), newRunner(), th, cfg), "gtk theme")
	if !ok || c.Level != doctor.LevelOK {
		t.Errorf("gtk theme = %+v, want ok", c)
	}
}

// Without fontconfig the answer is unknown, not "missing": Rice must not
// report a font as absent when it simply could not look.
func TestAssetsReportUnknownWithoutFontconfig(t *testing.T) {
	dataHome(t)

	runner := newRunner()
	runner.Missing = map[string]bool{"fc-list": true}

	th := theme.Theme{Fonts: theme.Fonts{UIFamily: "Inter", MonoFamily: "Iosevka"}}
	c, _ := find(doctor.Assets(context.Background(), runner, th, config.Config{}), "ui font")
	if c.Level != doctor.LevelUnknown {
		t.Errorf("ui font = %+v, want unknown", c)
	}
}

func TestSessionComparesTheEnvironment(t *testing.T) {
	th := theme.Theme{Cursor: theme.Cursor{Theme: "Bibata", Size: 24}}
	cfg := config.Config{
		Components: config.Components{Qt: true},
		Qt:         config.Qt{PlatformTheme: "qt5ct"},
		Sway:       config.Sway{WriteEnvironment: true},
	}

	env := map[string]string{
		"XCURSOR_THEME":        "Bibata",
		"XCURSOR_SIZE":         "24",
		"QT_QPA_PLATFORMTHEME": "qt5ct",
	}
	checks := doctor.Session(cfg, th, func(k string) string { return env[k] })
	if got := doctor.Worst(checks); got != doctor.LevelOK {
		t.Errorf("worst = %s, want ok", got)
	}

	// Unset is the common case after generating the file but before logging
	// out, and the message has to say so.
	delete(env, "QT_QPA_PLATFORMTHEME")
	c, _ := find(doctor.Session(cfg, th, func(k string) string { return env[k] }), "QT_QPA_PLATFORMTHEME")
	if c.Level != doctor.LevelWarn || !strings.Contains(c.Detail, "log out") {
		t.Errorf("QT_QPA_PLATFORMTHEME = %+v, want a warning about logging out", c)
	}

	// A different value means something else owns the variable.
	env["QT_QPA_PLATFORMTHEME"] = "gnome"
	c, _ = find(doctor.Session(cfg, th, func(k string) string { return env[k] }), "QT_QPA_PLATFORMTHEME")
	if c.Level != doctor.LevelWarn || !strings.Contains(c.Detail, "gnome") {
		t.Errorf("QT_QPA_PLATFORMTHEME = %+v, want the conflicting value named", c)
	}
}

func TestSessionSaysWhenNothingIsGenerated(t *testing.T) {
	checks := doctor.Session(config.Config{}, theme.Theme{}, func(string) string { return "" })
	if len(checks) != 1 || checks[0].Level != doctor.LevelWarn {
		t.Fatalf("checks = %+v, want one warning", checks)
	}
	if !strings.Contains(checks[0].Detail, "write_environment") {
		t.Errorf("detail = %q, want it to name the setting", checks[0].Detail)
	}
}
