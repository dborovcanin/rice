package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/generation"
	"github.com/dborovcanin/rice/internal/session"
)

func TestThemeNameFromImage(t *testing.T) {
	cases := []struct{ path, want string }{
		{"/home/user/Pictures/wallpaper.jpg", "wallpaper"},
		{"wall.png", "wall"},
		{"~/Pictures/Deep Space 9.jpeg", "Deep-Space-9"},
		{"/tmp/2026-09-01_shot.png", "2026-09-01_shot"},
		// A name that would escape the theme directory, or that the theme
		// store could not find again, has to come back usable.
		{"../../etc/passwd", "passwd"},
		{"/x/....png", "from-image"},
	}

	for _, c := range cases {
		got := themeNameFromImage(c.path)
		if got != c.want {
			t.Errorf("themeNameFromImage(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// Whatever an image is called, the derived name must be one the theme store
// will accept.
func TestThemeNameFromImageIsAlwaysValid(t *testing.T) {
	paths := []string{
		"a b c.png", "../escape.png", "/weird/%$#@.jpg", ".hidden.png",
		"name.with.dots.png", "/", "üñïçø∂é.png",
	}
	for _, p := range paths {
		name := themeNameFromImage(p)
		if err := session.ValidThemeName(name); err != nil {
			t.Errorf("themeNameFromImage(%q) = %q, which is rejected: %v", p, name, err)
		}
	}
}

// completionApp must not panic when the shared application has not been built,
// which is the normal state inside a completion process.
func TestCompletionFunctionsSurviveAnAbsentApp(t *testing.T) {
	none := func() *App { return nil }

	// These resolve their own App from the environment; whatever they find,
	// they must return rather than crash.
	t.Setenv("RICE_HOME", t.TempDir())

	for name, fn := range map[string]completionFunc{
		"themes":      completeThemes(none),
		"components":  completeComponents(none),
		"generations": completeGenerations(none),
	} {
		values, directive := fn(&cobra.Command{}, nil, "")
		if directive == cobra.ShellCompDirectiveError && name == "themes" {
			t.Errorf("%s completion failed outright", name)
		}
		for _, v := range values {
			if strings.HasPrefix(v, "\t") {
				t.Errorf("%s completion produced an entry with no value: %q", name, v)
			}
		}
	}
}

// Theme completion must offer the bundled themes even with an empty root, and
// must filter by what has been typed.
func TestThemeCompletionFiltersByPrefix(t *testing.T) {
	t.Setenv("RICE_HOME", t.TempDir())
	complete := completeThemes(func() *App { return nil })

	all, _ := complete(&cobra.Command{}, nil, "")
	if len(all) == 0 {
		t.Fatal("no themes were offered")
	}

	names := func(entries []string) []string {
		var out []string
		for _, e := range entries {
			name, _, _ := strings.Cut(e, "\t")
			out = append(out, name)
		}
		return out
	}
	if !slices.Contains(names(all), "tokyo-night") {
		t.Errorf("bundled themes missing from %v", names(all))
	}

	filtered, _ := complete(&cobra.Command{}, nil, "tok")
	for _, name := range names(filtered) {
		if !strings.HasPrefix(name, "tok") {
			t.Errorf("completion for %q offered %q", "tok", name)
		}
	}
	if len(filtered) == 0 {
		t.Error("a matching prefix offered nothing")
	}
}

// Every command in the tree needs a short description and an example, because
// `rice --help` is where someone finds out what Rice does.
func TestEveryCommandIsDocumented(t *testing.T) {
	root := NewRootCmd()

	forEachCommand(root, func(cmd *cobra.Command) {
		// Cobra generates these, and they are not ours to document.
		if cmd.Name() == "completion" || cmd.Parent() != nil && cmd.Parent().Name() == "completion" {
			return
		}
		if cmd.Name() == "help" {
			return
		}

		if cmd.Short == "" {
			t.Errorf("%q has no short description", cmd.CommandPath())
		}
		if cmd.Long == "" {
			t.Errorf("%q has no long description", cmd.CommandPath())
		}
		if cmd.Example == "" && cmd.HasSubCommands() == false {
			t.Errorf("%q has no example", cmd.CommandPath())
		}
	})
}

func TestWriteDiffReportsEachKindOfChange(t *testing.T) {
	base := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(base, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("foot/foot.ini", "a\nb\nc\n")
	write("rofi/config.rasi", "gone\n")
	// The manifest differs on every build and says nothing about the
	// configuration, so it must not appear.
	write(generation.ManifestName, "generation = 1\n")

	files := []generation.Rendered{
		{Component: "foot", Path: "foot/foot.ini", Content: []byte("a\nB\nc\n")},
		{Component: "waybar", Path: "waybar/style.css", Content: []byte("new\n")},
	}

	var out strings.Builder
	if err := writeDiff(&out, diffOptions{
		base: base, baseLabel: "generation 000001", files: files, context: 1,
	}); err != nil {
		t.Fatalf("writeDiff: %v", err)
	}
	got := out.String()

	for _, want := range []string{"-b", "+B", "+new", "-gone"} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, generation.ManifestName) {
		t.Errorf("the manifest should not be diffed:\n%s", got)
	}

	// The summary counts the same three files.
	out.Reset()
	if err := writeDiff(&out, diffOptions{
		base: base, baseLabel: "generation 000001", files: files, stat: true,
	}); err != nil {
		t.Fatalf("writeDiff: %v", err)
	}
	stat := out.String()
	for _, want := range []string{
		"changed  foot/foot.ini", "new      waybar/style.css",
		"removed  rofi/config.rasi", "3 file(s) differ",
	} {
		if !strings.Contains(stat, want) {
			t.Errorf("summary is missing %q:\n%s", want, stat)
		}
	}
}

func TestWriteDiffSaysNothingChanged(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "foot")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "foot.ini"), []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := writeDiff(&out, diffOptions{
		base:      base,
		baseLabel: "generation 000007",
		files:     []generation.Rendered{{Component: "foot", Path: "foot/foot.ini", Content: []byte("same\n")}},
	}); err != nil {
		t.Fatalf("writeDiff: %v", err)
	}
	if !strings.Contains(out.String(), "No change against generation 000007.") {
		t.Errorf("unchanged output = %q", out.String())
	}
}

func TestWriteDiffFiltersByComponent(t *testing.T) {
	base := t.TempDir()
	for _, dir := range []string{"foot", "waybar"} {
		if err := os.MkdirAll(filepath.Join(base, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "foot", "foot.ini"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "waybar", "style.css"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := []generation.Rendered{
		{Component: "foot", Path: "foot/foot.ini", Content: []byte("new\n")},
		{Component: "waybar", Path: "waybar/style.css", Content: []byte("new\n")},
	}

	var out strings.Builder
	if err := writeDiff(&out, diffOptions{
		base: base, baseLabel: "base", files: files, component: "foot", stat: true,
	}); err != nil {
		t.Fatalf("writeDiff: %v", err)
	}
	got := out.String()

	if !strings.Contains(got, "foot/foot.ini") {
		t.Errorf("the selected component is missing:\n%s", got)
	}
	if strings.Contains(got, "waybar") {
		t.Errorf("another component leaked in:\n%s", got)
	}
}

// With nothing committed yet, every file is new rather than an error.
func TestWriteDiffAgainstNothing(t *testing.T) {
	var out strings.Builder
	if err := writeDiff(&out, diffOptions{
		base:      "",
		baseLabel: "nothing",
		files:     []generation.Rendered{{Component: "foot", Path: "foot/foot.ini", Content: []byte("x\n")}},
		stat:      true,
	}); err != nil {
		t.Fatalf("writeDiff: %v", err)
	}
	if !strings.Contains(out.String(), "new      foot/foot.ini") {
		t.Errorf("output = %q", out.String())
	}
}
