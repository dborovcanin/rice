package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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
