package assets_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dborovcanin/rice/internal/assets"
)

// tree builds a data directory. A path ending in "/" is a directory; anything
// else is a file.
func tree(t *testing.T, paths ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, p := range paths {
		full := filepath.Join(root, p)
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	t.Setenv("XDG_DATA_HOME", root)
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "system"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("HOME", root)
	return root
}

// A directory in the right place is not a theme. Icon themes carry an
// index.theme, cursor themes a cursors directory, GTK themes a version
// directory; anything else is somebody's stray folder.
func TestListRequiresTheMarkerOfEachKind(t *testing.T) {
	tree(t,
		"icons/Papirus/index.theme",
		"icons/NotATheme/",
		"icons/Bibata/cursors/",
		"themes/Orchis/gtk-4.0/",
		"themes/StrayFolder/",
	)

	if got := assets.List(assets.IconThemes); !slices.Equal(got, []string{"Papirus"}) {
		t.Errorf("icon themes = %v, want [Papirus]", got)
	}
	if got := assets.List(assets.CursorThemes); !slices.Equal(got, []string{"Bibata"}) {
		t.Errorf("cursor themes = %v, want [Bibata]", got)
	}

	gtk := assets.List(assets.GTKThemes)
	if !slices.Contains(gtk, "Orchis") {
		t.Errorf("gtk themes = %v, want Orchis", gtk)
	}
	if slices.Contains(gtk, "StrayFolder") {
		t.Errorf("gtk themes = %v, should not include a directory with no version subdirectory", gtk)
	}
}

// GTK's built-in themes have no directory anywhere, and must still be offered.
func TestListIncludesBuiltinGTKThemes(t *testing.T) {
	tree(t)

	gtk := assets.List(assets.GTKThemes)
	for _, want := range []string{"Adwaita", "Adwaita-dark"} {
		if !slices.Contains(gtk, want) {
			t.Errorf("gtk themes = %v, want %s", gtk, want)
		}
	}
	if !assets.Installed(assets.GTKThemes, "Adwaita-dark") {
		t.Error("Adwaita-dark should count as installed")
	}
	if !assets.Builtin(assets.GTKThemes, "adwaita") {
		t.Error("Builtin should match case-insensitively")
	}
	if assets.Builtin(assets.IconThemes, "Adwaita") {
		t.Error("only GTK themes are built in")
	}
}

// The same theme installed system-wide and per-user is one theme.
func TestListDeduplicatesAcrossRoots(t *testing.T) {
	root := tree(t,
		"icons/Papirus/index.theme",
		"system/icons/Papirus/index.theme",
	)
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "system"))

	if got := assets.List(assets.IconThemes); !slices.Equal(got, []string{"Papirus"}) {
		t.Errorf("icon themes = %v, want one Papirus", got)
	}
}

func TestInstalled(t *testing.T) {
	tree(t, "icons/Papirus/index.theme")

	if !assets.Installed(assets.IconThemes, "Papirus") {
		t.Error("Papirus should be installed")
	}
	if assets.Installed(assets.IconThemes, "Nonesuch") {
		t.Error("Nonesuch should not be installed")
	}
	if assets.Installed(assets.IconThemes, "") {
		t.Error("an empty name is never installed")
	}
}

func TestKvantumThemesComeFromTheConfigDirectory(t *testing.T) {
	tree(t, "config/Kvantum/KvMine/")

	if got := assets.List(assets.KvantumThemes); !slices.Contains(got, "KvMine") {
		t.Errorf("kvantum themes = %v, want KvMine", got)
	}
}
