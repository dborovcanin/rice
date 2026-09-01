package theme

import (
	"reflect"
	"strings"
	"testing"
)

const minimalTheme = `
name = "minimal"

[colors]
background = "#181825"
foreground = "#cdd6f4"
primary = "#89b4fa"
`

func TestParseFillsDerivedValues(t *testing.T) {
	th, err := Parse([]byte(minimalTheme), "minimal.toml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if th.Colors.Surface.IsZero() || th.Colors.Muted.IsZero() || th.Colors.Border.IsZero() {
		t.Error("normalization left semantic colors unset")
	}
	if th.Colors.Secondary != th.Colors.Primary {
		t.Error("secondary should fall back to primary")
	}
	for i := range th.Terminal.Regular {
		if th.Terminal.Regular[i].IsZero() || th.Terminal.Bright[i].IsZero() {
			t.Fatalf("ANSI slot %d left unset", i)
		}
	}
	if th.Variant != "dark" {
		t.Errorf("variant = %q, want dark", th.Variant)
	}
	if th.Fonts.MonoSize != th.Fonts.UISize {
		t.Error("mono size should fall back to ui size")
	}
	if th.Cursor.Size == 0 {
		t.Error("cursor size should get a default")
	}
}

func TestParseKeepsExplicitValues(t *testing.T) {
	src := `
name = "explicit"
variant = "light"

[colors]
background = "#181825"
foreground = "#cdd6f4"
primary = "#89b4fa"

[terminal]
regular = ["#000001", "#000002", "#000003", "#000004", "#000005", "#000006", "#000007", "#000008"]

[ui]
radius = 12
opacity = 0.5

[fonts]
mono_family = "Iosevka"
mono_size = 14
`
	th, err := Parse([]byte(src), "explicit.toml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := th.Terminal.Regular[0].Hex(); got != "#000001" {
		t.Errorf("regular[0] = %s, want #000001", got)
	}
	if th.UI.Radius != 12 || th.UI.Opacity != 0.5 {
		t.Errorf("ui overridden values lost: %+v", th.UI)
	}
	if th.Fonts.MonoFamily != "Iosevka" || th.Fonts.MonoSize != 14 {
		t.Errorf("font overrides lost: %+v", th.Fonts)
	}
	if th.IsDark() {
		t.Error("explicit light variant should win over background luminance")
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "missing required color",
			src:  "name = \"x\"\n[colors]\nbackground = \"#000000\"\n",
			want: "colors.foreground is not set",
		},
		{
			name: "invalid color",
			src:  "name = \"x\"\n[colors]\nbackground = \"nope\"\n",
			want: "invalid hex digit",
		},
		{
			name: "unknown field",
			src:  "name = \"x\"\nwallpaper = \"a.png\"\n",
			want: "wallpaper",
		},
		{
			name: "opacity out of range",
			src:  minimalTheme + "\n[ui]\nopacity = 1.5\n",
			want: "ui.opacity",
		},
		{
			name: "identical fg and bg",
			src:  "name = \"x\"\n[colors]\nbackground = \"#111111\"\nforeground = \"#111111\"\nprimary = \"#222222\"\n",
			want: "identical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.src), tt.name)
			if err == nil {
				t.Fatal("want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestNormalizeIsIdempotent(t *testing.T) {
	th, err := Parse([]byte(minimalTheme), "minimal.toml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	before := th
	th.Normalize()
	if !reflect.DeepEqual(th, before) {
		t.Error("second Normalize changed the theme")
	}
}

func TestNameFallsBackToFileName(t *testing.T) {
	th, err := Parse([]byte("[colors]\nbackground=\"#000000\"\nforeground=\"#ffffff\"\nprimary=\"#ff0000\"\n"), "themes/gruvbox-dark.toml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if th.Name != "gruvbox-dark" {
		t.Errorf("name = %q, want gruvbox-dark", th.Name)
	}
}
