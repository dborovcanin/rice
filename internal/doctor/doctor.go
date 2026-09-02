// Package doctor checks that what a theme asks for actually exists on the
// machine: the fonts it names, the icon, cursor, GTK and Kvantum themes it
// selects, and the session environment the toolkits need.
//
// A theme that names a font nobody has installed still renders, still
// validates and still deploys. Everything looks correct except the desktop.
// These checks are the difference between that and an answer.
package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dborovcanin/rice/internal/command"
	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/fonts"
	"github.com/dborovcanin/rice/internal/theme"
)

// Level is how much a finding matters.
type Level int

const (
	// LevelOK means the thing exists and matches.
	LevelOK Level = iota
	// LevelWarn means something is off but the desktop still works, usually
	// by falling back to a default.
	LevelWarn
	// LevelUnknown means the check could not run, which is not a failure of
	// the configuration.
	LevelUnknown
)

func (l Level) String() string {
	switch l {
	case LevelOK:
		return "ok"
	case LevelWarn:
		return "warn"
	default:
		return "unknown"
	}
}

// Marker is the single character a report puts in front of a check.
func (l Level) Marker() string {
	switch l {
	case LevelOK:
		return "✓"
	case LevelWarn:
		return "!"
	default:
		return "?"
	}
}

// Check is one finding.
type Check struct {
	// Name is what was checked, such as "mono font".
	Name string
	// Value is what the theme or configuration asked for.
	Value string
	// Level is how much it matters.
	Level Level
	// Detail explains a warning, and is empty when everything is fine.
	Detail string
}

// Environment reads an environment variable. It is a parameter so the checks
// can be tested without touching the process environment.
type Environment func(string) string

// Assets reports whether the fonts, icon theme, cursor theme, GTK theme and
// Kvantum theme a resolved theme names are installed.
func Assets(ctx context.Context, runner command.Runner, th theme.Theme, cfg config.Config) []Check {
	var checks []Check
	checks = append(checks, fontChecks(ctx, runner, th)...)

	checks = append(checks, dirCheck(
		"icon theme", th.Icons.Theme,
		iconDirs(), func(dir string) string { return filepath.Join(dir, th.Icons.Theme) },
		"icons fall back to whatever the toolkit finds",
	))

	checks = append(checks, dirCheck(
		"cursor theme", th.Cursor.Theme,
		cursorDirs(), func(dir string) string { return filepath.Join(dir, th.Cursor.Theme, "cursors") },
		"the pointer falls back to the default cursor",
	))

	if cfg.Components.GTK {
		checks = append(checks, gtkThemeCheck(th.GTK.Theme))
	}

	if cfg.Components.Qt && cfg.Qt.Kvantum {
		name := th.GTK.KvantumTheme
		if name == "" {
			checks = append(checks, Check{
				Name: "kvantum theme", Value: "unset", Level: LevelWarn,
				Detail: "the theme names no Kvantum theme, so the generated one is a guess",
			})
		} else {
			checks = append(checks, dirCheck(
				"kvantum theme", name,
				kvantumDirs(), func(dir string) string { return filepath.Join(dir, name) },
				"Kvantum falls back to its default theme",
			))
		}
	}

	return checks
}

// builtinGTKThemes ship inside GTK itself rather than as a directory under
// share/themes, so looking for them on disk finds nothing even though they
// work. Adwaita is the default on most systems, which would make the check
// warn about the one theme most likely to be correct.
var builtinGTKThemes = map[string]bool{
	"adwaita":             true,
	"adwaita-dark":        true,
	"default":             true,
	"emacs":               true,
	"highcontrast":        true,
	"highcontrastinverse": true,
	"raleigh":             true,
}

func gtkThemeCheck(name string) Check {
	if builtinGTKThemes[strings.ToLower(strings.TrimSpace(name))] {
		return Check{Name: "gtk theme", Value: name, Level: LevelOK, Detail: "built into GTK"}
	}
	return dirCheck(
		"gtk theme", name,
		dataDirs("themes"), func(dir string) string { return filepath.Join(dir, name) },
		"GTK applications fall back to Adwaita",
	)
}

// fontChecks resolves the theme's font families against fontconfig.
func fontChecks(ctx context.Context, runner command.Runner, th theme.Theme) []Check {
	families := []struct{ name, family string }{
		{"ui font", th.Fonts.UIFamily},
		{"mono font", th.Fonts.MonoFamily},
		{"bar font", th.Fonts.BarFont()},
	}

	catalog, err := fonts.Load(ctx, runner)
	if err != nil {
		var checks []Check
		for _, f := range families {
			checks = append(checks, Check{
				Name: f.name, Value: f.family, Level: LevelUnknown,
				Detail: "fontconfig is not available to check with",
			})
		}
		return checks
	}

	var checks []Check
	for _, f := range families {
		check := Check{Name: f.name, Value: f.family}
		switch {
		case isGenericFamily(f.family):
			// "monospace" and "sans-serif" are fontconfig aliases, not
			// families; asking whether they are installed is meaningless.
			check.Level = LevelOK
			check.Detail = "generic family, resolved by fontconfig"
		case catalog.Has(f.family):
			check.Level = LevelOK
		default:
			check.Level = LevelWarn
			check.Detail = "not installed: fontconfig will substitute another family"
		}
		checks = append(checks, check)
	}
	return checks
}

// isGenericFamily reports whether a family name is a fontconfig alias rather
// than something that can be installed.
func isGenericFamily(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "monospace", "mono", "sans-serif", "sans", "serif", "cursive", "fantasy", "system-ui":
		return true
	}
	return false
}

// dirCheck looks for a directory under any of several roots.
func dirCheck(name, value string, roots []string, path func(string) string, consequence string) Check {
	check := Check{Name: name, Value: value}
	if strings.TrimSpace(value) == "" {
		check.Level = LevelWarn
		check.Value = "unset"
		check.Detail = consequence
		return check
	}

	for _, root := range roots {
		if info, err := os.Stat(path(root)); err == nil && info.IsDir() {
			check.Level = LevelOK
			return check
		}
	}

	check.Level = LevelWarn
	check.Detail = "not found in " + strings.Join(roots, ", ") + ": " + consequence
	return check
}

// Session checks that the environment the toolkits read matches what Rice
// generates. It is the usual reason a correct configuration has no effect: the
// file is written, but the session that would read it has not restarted.
func Session(cfg config.Config, th theme.Theme, env Environment) []Check {
	if env == nil {
		env = os.Getenv
	}

	var checks []Check

	if !cfg.Sway.WriteEnvironment {
		return append(checks, Check{
			Name: "session environment", Value: "not generated", Level: LevelWarn,
			Detail: "sway.write_environment is off, so the cursor and Qt platform theme are yours to set",
		})
	}

	checks = append(checks, envCheck("XCURSOR_THEME", th.Cursor.Theme, env))
	checks = append(checks, envCheck("XCURSOR_SIZE", strconv.Itoa(th.Cursor.Size), env))

	if cfg.Components.Qt && cfg.Qt.PlatformTheme != "" {
		checks = append(checks, envCheck("QT_QPA_PLATFORMTHEME", cfg.Qt.PlatformTheme, env))
	}
	return checks
}

// envCheck compares one variable against what Rice would set it to.
func envCheck(name, want string, env Environment) Check {
	check := Check{Name: name, Value: want}

	switch got := env(name); {
	case got == want:
		check.Level = LevelOK
	case got == "":
		check.Level = LevelWarn
		check.Detail = "not set in this session: log out and back in to pick it up"
	default:
		check.Level = LevelWarn
		check.Detail = "this session has " + got + ": something else is setting it"
	}
	return check
}

// Worst is the highest level in a set of checks, so a caller can summarize.
func Worst(checks []Check) Level {
	worst := LevelOK
	for _, c := range checks {
		if c.Level > worst {
			worst = c.Level
		}
	}
	return worst
}

// dataDirs returns the XDG data directories with a subdirectory appended, in
// lookup order: the user's own first, then the system's.
func dataDirs(sub string) []string {
	var roots []string

	if home := os.Getenv("XDG_DATA_HOME"); home != "" {
		roots = append(roots, home)
	} else if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, ".local", "share"))
	}

	dirs := os.Getenv("XDG_DATA_DIRS")
	if dirs == "" {
		dirs = "/usr/local/share:/usr/share"
	}
	roots = append(roots, filepath.SplitList(dirs)...)

	out := make([]string, 0, len(roots))
	for _, root := range roots {
		out = append(out, filepath.Join(root, sub))
	}
	return out
}

// iconDirs is where icon themes live. ~/.icons is the legacy location, and
// still in use.
func iconDirs() []string {
	dirs := dataDirs("icons")
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".icons"))
	}
	return dirs
}

// cursorDirs is where cursor themes live, which is the same set as icons.
func cursorDirs() []string { return iconDirs() }

// kvantumDirs is where Kvantum themes live: the user's configuration
// directory first, then the shared data directories.
func kvantumDirs() []string {
	var dirs []string
	if cfg := os.Getenv("XDG_CONFIG_HOME"); cfg != "" {
		dirs = append(dirs, filepath.Join(cfg, "Kvantum"))
	} else if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".config", "Kvantum"))
	}
	return append(dirs, dataDirs("Kvantum")...)
}
