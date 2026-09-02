package palette_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"

	"github.com/dborovcanin/rice/internal/palette"
	"github.com/dborovcanin/rice/internal/theme"
)

// draw builds an image from horizontal bands of the given colors, which is
// enough structure to test a palette without shipping a wallpaper.
func draw(colors ...color.RGBA) image.Image {
	const size = 120
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	band := size / len(colors)
	for y := range size {
		c := colors[min(y/band, len(colors)-1)]
		for x := range size {
			img.Set(x, y, c)
		}
	}
	return img
}

func rgba(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 0xff} }

// contrast is the WCAG ratio, repeated here so the test asserts the property
// rather than trusting the implementation's own helper.
func contrast(a, b theme.Color) float64 {
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

func TestDerivedThemeIsUsable(t *testing.T) {
	// Mostly a deep blue ground, with three accents across the rest, which is
	// roughly how a wallpaper is built.
	img := draw(
		rgba(20, 24, 38),
		rgba(20, 24, 38),
		rgba(22, 26, 42),
		rgba(70, 130, 200),
		rgba(200, 90, 120),
		rgba(230, 220, 190),
	)

	th, err := palette.FromImage(img, palette.Options{Name: "from-image"})
	if err != nil {
		t.Fatalf("FromImage: %v", err)
	}

	// It has to survive the ordinary pipeline: normalize, then validate.
	th.Normalize()
	if err := th.Validate(); err != nil {
		t.Fatalf("derived theme does not validate: %v", err)
	}

	if got := contrast(th.Colors.Foreground, th.Colors.Background); got < 7 {
		t.Errorf("foreground/background contrast = %.2f, want at least 7", got)
	}
	if th.Variant != "dark" {
		t.Errorf("variant = %q, want dark for a dark image", th.Variant)
	}

	// The accents have to be distinguishable from each other, or there is one
	// accent wearing three names.
	accents := []theme.Color{th.Colors.Primary, th.Colors.Secondary, th.Colors.Accent}
	for i := range accents {
		for j := i + 1; j < len(accents); j++ {
			if accents[i] == accents[j] {
				t.Errorf("accents %d and %d are identical (%s)", i, j, accents[i])
			}
		}
	}
}

// The same image must always give the same theme. A palette that changes
// between runs is not a theme.
func TestDerivationIsDeterministic(t *testing.T) {
	img := draw(rgba(30, 30, 40), rgba(180, 60, 90), rgba(60, 180, 140), rgba(220, 200, 120))

	first, err := palette.FromImage(img, palette.Options{Name: "x"})
	if err != nil {
		t.Fatalf("FromImage: %v", err)
	}
	for range 5 {
		again, err := palette.FromImage(img, palette.Options{Name: "x"})
		if err != nil {
			t.Fatalf("FromImage: %v", err)
		}
		if again.Colors != first.Colors || again.Variant != first.Variant {
			t.Fatalf("the same image produced a different theme:\n%+v\n%+v", first.Colors, again.Colors)
		}
	}
}

func TestVariantFollowsTheImageAndTheOption(t *testing.T) {
	dark := draw(rgba(10, 10, 14), rgba(40, 60, 90))
	light := draw(rgba(245, 243, 238), rgba(200, 190, 170))

	for _, c := range []struct {
		name string
		img  image.Image
		want string
	}{
		{"dark image", dark, "dark"},
		{"light image", light, "light"},
	} {
		th, err := palette.FromImage(c.img, palette.Options{})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if th.Variant != c.want {
			t.Errorf("%s: variant = %q, want %q", c.name, th.Variant, c.want)
		}
	}

	// An explicit variant wins over what the image suggests.
	th, err := palette.FromImage(dark, palette.Options{Variant: "light"})
	if err != nil {
		t.Fatalf("FromImage: %v", err)
	}
	if th.Variant != "light" {
		t.Errorf("variant = %q, want the requested light", th.Variant)
	}
	if !th.Colors.Background.IsDark() == false {
		t.Error("a light variant should have a light background")
	}
	if got := contrast(th.Colors.Foreground, th.Colors.Background); got < 7 {
		t.Errorf("light contrast = %.2f, want at least 7", got)
	}
}

// A wallpaper with no colour in it still has to produce three usable accents.
func TestMonochromeImageStillYieldsAccents(t *testing.T) {
	img := draw(rgba(18, 18, 18), rgba(90, 90, 90), rgba(160, 160, 160))

	th, err := palette.FromImage(img, palette.Options{Name: "grey"})
	if err != nil {
		t.Fatalf("FromImage: %v", err)
	}
	th.Normalize()
	if err := th.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	for name, c := range map[string]theme.Color{
		"primary":   th.Colors.Primary,
		"secondary": th.Colors.Secondary,
		"accent":    th.Colors.Accent,
	} {
		if c.IsZero() {
			t.Errorf("%s was left unset", name)
		}
	}
	if th.Colors.Primary == th.Colors.Secondary {
		t.Error("primary and secondary should differ even on a grey image")
	}
}

// Success, warning and error must read as themselves whatever the image is:
// a green that is not green is worse than one that does not match.
func TestStatusColorsKeepTheirMeaning(t *testing.T) {
	img := draw(rgba(30, 20, 60), rgba(120, 60, 200), rgba(200, 120, 240))

	th, err := palette.FromImage(img, palette.Options{})
	if err != nil {
		t.Fatalf("FromImage: %v", err)
	}

	if th.Colors.Success.G <= th.Colors.Success.R || th.Colors.Success.G <= th.Colors.Success.B {
		t.Errorf("success = %s, want a green", th.Colors.Success)
	}
	if th.Colors.Error.R <= th.Colors.Error.G || th.Colors.Error.R <= th.Colors.Error.B {
		t.Errorf("error = %s, want a red", th.Colors.Error)
	}
}

// Everything the theme model can derive is left unset, so a generated theme
// reads like a hand-written one and keeps following its own semantic colors.
func TestDerivableValuesAreLeftUnset(t *testing.T) {
	img := draw(rgba(25, 30, 40), rgba(90, 140, 190))

	th, err := palette.FromImage(img, palette.Options{Name: "sparse"})
	if err != nil {
		t.Fatalf("FromImage: %v", err)
	}

	if !th.Colors.Surface.IsZero() {
		t.Error("surface should be derived, not chosen from pixels")
	}
	if !th.Colors.Muted.IsZero() {
		t.Error("muted should be derived")
	}
	if !th.Terminal.Background.IsZero() {
		t.Error("the terminal palette should be derived")
	}
	for i, c := range th.Terminal.Regular {
		if !c.IsZero() {
			t.Errorf("terminal.regular.%d should be derived", i)
		}
	}
}

func TestFromReaderRejectsWhatItCannotDecode(t *testing.T) {
	if _, err := palette.FromReader(bytes.NewReader([]byte("not an image")), palette.Options{}); err == nil {
		t.Error("decoding nonsense should fail")
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, draw(rgba(40, 40, 60), rgba(150, 120, 90))); err != nil {
		t.Fatalf("encode: %v", err)
	}
	th, err := palette.FromReader(&buf, palette.Options{Name: "png"})
	if err != nil {
		t.Fatalf("FromReader: %v", err)
	}
	if th.Description == "" {
		t.Error("a derived theme should say where it came from")
	}
}

// An image says what colours to use and nothing else. Everything a wallpaper
// cannot express has to come from somewhere, or Rice writes empty font and
// icon-theme settings out to the toolkits.
func TestDerivedThemeInheritsWhatAnImageCannotSay(t *testing.T) {
	base := theme.Theme{
		Variant: "dark",
		Fonts:   theme.Fonts{UIFamily: "Inter", UISize: 11, MonoFamily: "Iosevka", MonoSize: 12},
		Icons:   theme.Icons{Theme: "Papirus-Dark", Size: 32, Paths: []string{"/usr/share/icons"}},
		Cursor:  theme.Cursor{Theme: "Bibata", Size: 24},
		GTK:     theme.GTK{Theme: "Orchis-Dark", KvantumTheme: "KvMine"},
		UI:      theme.UI{Radius: 12, GapsInner: 6},
	}

	img := draw(rgba(20, 24, 38), rgba(20, 24, 38), rgba(70, 130, 200))
	th, err := palette.FromImage(img, palette.Options{Name: "inherited", Base: base})
	if err != nil {
		t.Fatalf("FromImage: %v", err)
	}

	if th.Fonts != base.Fonts {
		t.Errorf("fonts = %+v, want them inherited", th.Fonts)
	}
	if th.Cursor != base.Cursor {
		t.Errorf("cursor = %+v, want it inherited", th.Cursor)
	}
	if th.Icons.Theme != base.Icons.Theme || th.Icons.Size != base.Icons.Size {
		t.Errorf("icons = %+v, want them inherited", th.Icons)
	}
	if th.UI != base.UI {
		t.Errorf("geometry = %+v, want it inherited", th.UI)
	}
	if th.Name != "inherited" {
		t.Errorf("name = %q", th.Name)
	}

	// The colours are the image's, not the base's.
	if th.Colors.Background == base.Colors.Background {
		t.Error("the palette should come from the image")
	}
}

// A widget theme built for the other variant is worse than none: a dark GTK
// theme on a light palette looks broken. Normalization can pick a matching
// one, so the inherited value is cleared when the variant flips.
func TestVariantFlipDropsTheInheritedWidgetTheme(t *testing.T) {
	base := theme.Theme{
		Variant: "dark",
		GTK:     theme.GTK{Theme: "Orchis-Dark", KvantumTheme: "KvDark"},
		Fonts:   theme.Fonts{UIFamily: "Inter"},
	}

	img := draw(rgba(20, 24, 38), rgba(20, 24, 38), rgba(70, 130, 200))

	same, err := palette.FromImage(img, palette.Options{Base: base, Variant: "dark"})
	if err != nil {
		t.Fatalf("FromImage: %v", err)
	}
	if same.GTK.Theme != "Orchis-Dark" {
		t.Errorf("gtk theme = %q, want it kept when the variant matches", same.GTK.Theme)
	}

	flipped, err := palette.FromImage(img, palette.Options{Base: base, Variant: "light"})
	if err != nil {
		t.Fatalf("FromImage: %v", err)
	}
	if flipped.GTK.Theme != "" {
		t.Errorf("gtk theme = %q, want it dropped when the variant flips", flipped.GTK.Theme)
	}
	if flipped.GTK.KvantumTheme != "" {
		t.Errorf("kvantum theme = %q, want it dropped too", flipped.GTK.KvantumTheme)
	}
	// The rest still comes across.
	if flipped.Fonts.UIFamily != "Inter" {
		t.Error("fonts should survive a variant flip")
	}
}
