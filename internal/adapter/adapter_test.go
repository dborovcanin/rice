package adapter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/adapter/dunst"
	"github.com/dborovcanin/rice/internal/adapter/foot"
	"github.com/dborovcanin/rice/internal/adapter/rofi"
	"github.com/dborovcanin/rice/internal/adapter/sway"
	"github.com/dborovcanin/rice/internal/adapter/swaylock"
	"github.com/dborovcanin/rice/internal/adapter/waybar"
)

func all() []adapter.Adapter {
	return []adapter.Adapter{
		sway.New(), waybar.New(), rofi.New(), foot.New(), dunst.New(), swaylock.New(),
	}
}

func TestRegistry(t *testing.T) {
	r := adapter.NewRegistry(all()...)

	names := r.Names()
	if len(names) != 6 {
		t.Fatalf("Names() = %v", names)
	}

	if _, err := r.Get("sway"); err != nil {
		t.Errorf("Get(sway): %v", err)
	}
	if _, err := r.Get("i3"); err == nil {
		t.Error("Get(i3) should fail")
	}

	selected, err := r.Select([]string{"foot", "sway"})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if selected[0].Name() != "foot" || selected[1].Name() != "sway" {
		t.Errorf("Select should preserve order, got %s, %s", selected[0].Name(), selected[1].Name())
	}
	if _, err := r.Select([]string{"foot", "nope"}); err == nil {
		t.Error("Select should fail on an unknown component")
	}
}

func TestAdapterDeclarations(t *testing.T) {
	for _, a := range all() {
		t.Run(a.Name(), func(t *testing.T) {
			files := a.Files()
			if len(files) == 0 {
				t.Fatal("no files declared")
			}
			for _, f := range files {
				if !strings.HasSuffix(f.Template, ".tmpl") {
					t.Errorf("template %q should end in .tmpl", f.Template)
				}
				if f.Path == "" {
					t.Error("file has no destination path")
				}
				if f.FileMode() != 0o644 {
					t.Errorf("default mode = %v", f.FileMode())
				}
			}

			paths := a.ConfigPaths()
			if len(paths) != len(files) {
				t.Errorf("%d config paths for %d files", len(paths), len(files))
			}
			declared := map[string]bool{}
			for _, f := range files {
				declared[f.Path] = true
			}
			for _, p := range paths {
				if !declared[p.Source] {
					t.Errorf("config path %q has no generated file", p.Source)
				}
			}
		})
	}
}

func TestReloadModeString(t *testing.T) {
	tests := map[adapter.ReloadMode]string{
		adapter.ReloadNone:             "none",
		adapter.ReloadHot:              "hot",
		adapter.ReloadSignal:           "signal",
		adapter.ReloadRestart:          "restart",
		adapter.ReloadNewInstancesOnly: "new-instances-only",
	}
	for mode, want := range tests {
		if got := mode.String(); got != want {
			t.Errorf("ReloadMode(%d) = %q, want %q", mode, got, want)
		}
	}
}

// TestValidateRejectsBrokenOutput feeds each adapter a good and a broken file,
// since these validators are the last thing standing between a template bug
// and a desktop that will not start.
func TestValidateRejectsBrokenOutput(t *testing.T) {
	tests := []struct {
		adapter adapter.Adapter
		path    string
		good    string
		bad     string
		badWant string
	}{
		{
			adapter: sway.New(),
			path:    "sway/config",
			good:    "# comment {\nbar {\n    position top\n}\n",
			bad:     "bar {\n    position top\n",
			badWant: "unclosed",
		},
		{
			adapter: foot.New(),
			path:    "foot/foot.ini",
			good:    "# comment\nfont=mono:size=12\n\n[colors]\nalpha=0.9\n",
			bad:     "font=mono\n[colors\nalpha=0.9\n",
			badWant: "unterminated section header",
		},
		{
			adapter: waybar.New(),
			path:    "waybar/config.jsonc",
			good:    "// comment with \"quotes\"\n{\n  \"format\": \"<tt>{}</tt> // not a comment\"\n}\n",
			bad:     "{\n  \"format\": ,\n}\n",
			badWant: "invalid character",
		},
		{
			adapter: rofi.New(),
			path:    "rofi/config.rasi",
			good:    "configuration {\n    modes: \"drun\";\n}\n",
			bad:     "configuration {\n    modes: \"drun\";\n}\n}\n",
			badWant: "unexpected '}'",
		},
		{
			adapter: dunst.New(),
			path:    "dunst/dunstrc",
			good:    "[global]\n    origin = top-center\n",
			bad:     "[urgency_low]\n    timeout = 5\n",
			badWant: "missing [global]",
		},
		{
			adapter: swaylock.New(),
			path:    "swaylock/config",
			good:    "# comment\ndaemonize\nindicator-radius=100\n",
			bad:     "--daemonize\n",
			badWant: "dash-prefixed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.adapter.Name(), func(t *testing.T) {
			goodDir := writeFile(t, tt.path, tt.good)
			if err := tt.adapter.Validate(goodDir); err != nil {
				t.Errorf("valid output rejected: %v", err)
			}

			badDir := writeFile(t, tt.path, tt.bad)
			err := tt.adapter.Validate(badDir)
			if err == nil {
				t.Fatal("broken output accepted")
			}
			if !strings.Contains(err.Error(), tt.badWant) {
				t.Errorf("error = %v, want it to mention %q", err, tt.badWant)
			}

			if err := tt.adapter.Validate(t.TempDir()); err == nil {
				t.Error("a missing file should fail validation")
			}
		})
	}
}

func writeFile(t *testing.T, rel, content string) string {
	t.Helper()
	dir := t.TempDir()
	dest := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
