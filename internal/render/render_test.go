package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/theme"
)

func testTheme(t *testing.T) theme.Theme {
	t.Helper()
	th, err := theme.Parse([]byte(`
name = "test"

[colors]
background = "#282828"
foreground = "#ebdbb2"
primary = "#d79921"

[ui]
radius = 5
opacity = 0.9
`), "test.toml")
	if err != nil {
		t.Fatalf("parse test theme: %v", err)
	}
	return th
}

func testEngine(t *testing.T, files fstest.MapFS) *Engine {
	t.Helper()
	return NewEngine("", files, "templates")
}

func TestRenderUsesThemeAndConfig(t *testing.T) {
	engine := testEngine(t, fstest.MapFS{
		"templates/x.tmpl": &fstest.MapFile{
			Data: []byte("bg={{ bare .Colors.Background }} radius={{ .UI.Radius }} mod={{ .Sway.Mod }} gen={{ .Generation }}"),
		},
	})

	cfg := config.DefaultConfig()
	cfg.Normalize()
	out, err := engine.Render("x.tmpl", NewContext(cfg, testTheme(t), 7, "0.1.0"))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "bg=282828 radius=5 mod=Mod4 gen=7"
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestRenderUserTemplateWins(t *testing.T) {
	builtin := fstest.MapFS{
		"templates/x.tmpl": &fstest.MapFile{Data: []byte("builtin")},
		"templates/y.tmpl": &fstest.MapFile{Data: []byte("builtin y")},
	}
	userDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(userDir, "x.tmpl"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(userDir, builtin, "templates")
	cfg := config.DefaultConfig()
	cfg.Normalize()
	ctx := NewContext(cfg, testTheme(t), 1, "0.1.0")

	out, err := engine.Render("x.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if string(out) != "user" {
		t.Errorf("user template should win, got %q", out)
	}

	out, err = engine.Render("y.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if string(out) != "builtin y" {
		t.Errorf("builtin fallback failed, got %q", out)
	}
}

func TestRenderMissingTemplate(t *testing.T) {
	engine := testEngine(t, fstest.MapFS{})
	cfg := config.DefaultConfig()
	cfg.Normalize()
	_, err := engine.Render("nope.tmpl", NewContext(cfg, testTheme(t), 1, "0.1.0"))
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "template not found") {
		t.Errorf("error = %v", err)
	}
}

func TestRenderReportsTemplateErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "syntax error", body: "{{ .Colors", want: "parse template"},
		{name: "unknown field", body: "{{ .Colors.Nope }}", want: "render template"},
		{name: "wrong func arity", body: "{{ mix .Colors.Primary }}", want: "wrong number of args"},
	}
	cfg := config.DefaultConfig()
	cfg.Normalize()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := testEngine(t, fstest.MapFS{
				"templates/x.tmpl": &fstest.MapFile{Data: []byte(tt.body)},
			})
			_, err := engine.Render("x.tmpl", NewContext(cfg, testTheme(t), 1, "0.1.0"))
			if err == nil {
				t.Fatal("want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestFuncs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Normalize()
	ctx := NewContext(cfg, testTheme(t), 1, "0.1.0")

	tests := []struct {
		name string
		body string
		want string
	}{
		{"bare", "{{ bare .Colors.Primary }}", "d79921"},
		{"alpha", "{{ bareA (alpha 0.5 .Colors.Primary) }}", "d7992180"},
		{"lighten", "{{ hex (lighten 1.0 .Colors.Background) }}", "#ffffff"},
		{"darken", "{{ hex (darken 1.0 .Colors.Primary) }}", "#000000"},
		{"contrast", "{{ contrast .Colors.Background }}", "#ffffff"},
		{"arith", "{{ add 2 3 }} {{ sub 5 2 }} {{ mul 3 3 }} {{ div 9 2 }} {{ div 1 0 }}", "5 3 9 4 0"},
		{"scale", "{{ scale 0.9 100 }}", "90"},
		{"percentage", "{{ percentage 0.925 }}", "92.5%"},
		{"float", "{{ float 1.0 }} {{ float 0.5 }}", "1 0.5"},
		{"font", "{{ font .Fonts.MonoFamily .Fonts.MonoSize }}", "monospace 11"},
		{"join", `{{ join "," (lines "a\nb") }}`, "a,b"},
		{"default", `{{ default "fallback" "" }} {{ default "fallback" "set" }}`, "fallback set"},
		{"quote", `{{ quote "a b" }}`, `"a b"`},
		{"json", `{{ json .Waybar.ModulesCenter }}`, `["sway/window"]`},
		{"json no html escape", `{{ json "<tt>" }}`, `"<tt>"`},
		{"indent", `{{ indent 2 "a\nb" }}`, "a\n  b"},
		{"comment", `{{ comment "a\nb" }}`, "# a\n# b"},
		{"hasPrefix", `{{ hasPrefix "exec" "exec foo" }}`, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := testEngine(t, fstest.MapFS{
				"templates/x.tmpl": &fstest.MapFile{Data: []byte(tt.body)},
			})
			out, err := engine.Render("x.tmpl", ctx)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if string(out) != tt.want {
				t.Errorf("got %q, want %q", out, tt.want)
			}
		})
	}
}
