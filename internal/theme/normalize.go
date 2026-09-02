package theme

// Normalize fills in every value an adapter may reference but a theme author
// is allowed to omit. It is idempotent, and always runs before validation so
// that validation errors describe the effective theme, not the file.
func (t *Theme) Normalize() {
	c := &t.Colors

	// Surfaces step away from the background, which means lighter on a dark
	// theme and darker on a light one. Always lightening would sink a light
	// theme's surfaces into a white background and make them invisible.
	step := t.away()

	if c.Surface.IsZero() {
		c.Surface = step(c.Background, 0.04)
	}
	if c.SurfaceAlt.IsZero() {
		c.SurfaceAlt = step(c.Surface, 0.06)
	}
	if c.Overlay.IsZero() {
		c.Overlay = step(c.SurfaceAlt, 0.06)
	}
	if c.Muted.IsZero() {
		c.Muted = c.Foreground.Mix(c.Background, 0.45)
	}
	if c.Secondary.IsZero() {
		c.Secondary = c.Primary
	}
	if c.Accent.IsZero() {
		c.Accent = c.Secondary
	}
	if c.Success.IsZero() {
		c.Success = c.Primary
	}
	if c.Warning.IsZero() {
		c.Warning = c.Secondary
	}
	if c.Error.IsZero() {
		c.Error = MustParseColor("#e06c75")
	}
	if c.Border.IsZero() {
		c.Border = c.SurfaceAlt
	}
	if c.BorderFocus.IsZero() {
		c.BorderFocus = c.Primary
	}

	t.normalizeTerminal()
	t.normalizeUI()
	t.normalizeFonts()

	if t.Icons.Size == 0 {
		t.Icons.Size = 24
	}
	if t.Cursor.Size == 0 {
		t.Cursor.Size = 24
	}
	if t.Variant == "" {
		if c.Background.IsDark() {
			t.Variant = "dark"
		} else {
			t.Variant = "light"
		}
	}
	if t.GTK.Theme == "" {
		if t.IsDark() {
			t.GTK.Theme = "Adwaita-dark"
		} else {
			t.GTK.Theme = "Adwaita"
		}
	}
	t.GTK.PreferDark = t.GTK.PreferDark || t.IsDark()
}

// away returns the direction a derived shade should move to stay visible
// against the background: lighter on a dark theme, darker on a light one.
//
// The variant is not settled yet when colours are derived, so this reads the
// background rather than the field.
func (t *Theme) away() func(Color, float64) Color {
	if t.Colors.Background.IsDark() {
		return Color.Lighten
	}
	return Color.Darken
}

// normalizeTerminal derives any unset ANSI slot from the semantic palette, so
// a theme only has to spell out the 16 colors when it wants exact control.
func (t *Theme) normalizeTerminal() {
	c := t.Colors
	term := &t.Terminal

	if term.Background.IsZero() {
		term.Background = c.Background
	}
	if term.Foreground.IsZero() {
		term.Foreground = c.Foreground
	}

	step := t.away()

	derivedRegular := [8]Color{
		c.Background,
		c.Error,
		c.Success,
		c.Warning,
		c.Primary,
		c.Secondary,
		c.Accent,
		step(c.Foreground, 0.15),
	}
	for i := range term.Regular {
		if term.Regular[i].IsZero() {
			term.Regular[i] = derivedRegular[i]
		}
	}
	for i := range term.Bright {
		if !term.Bright[i].IsZero() {
			continue
		}
		switch i {
		case 0:
			term.Bright[i] = c.Muted
		case 7:
			term.Bright[i] = c.Foreground
		default:
			// "Bright" means "further from the background", not "lighter":
			// a lighter bright red is invisible on a light theme.
			term.Bright[i] = step(term.Regular[i], 0.2)
		}
	}

	if term.SelectionBackground.IsZero() {
		term.SelectionBackground = c.SurfaceAlt
	}
	if term.SelectionForeground.IsZero() {
		term.SelectionForeground = c.Foreground
	}
	if term.Cursor.IsZero() {
		term.Cursor = c.Primary
	}
	if term.URL.IsZero() {
		term.URL = c.Secondary
	}
}

func (t *Theme) normalizeUI() {
	ui := &t.UI

	if ui.BorderWidth == 0 {
		ui.BorderWidth = 2
	}
	if ui.Padding == 0 {
		ui.Padding = 4
	}
	if ui.HorizontalPadding == 0 {
		ui.HorizontalPadding = 8
	}
	if ui.Opacity == 0 {
		ui.Opacity = 1
	}
	if ui.BlurRadius > 0 && ui.BlurPasses == 0 {
		ui.BlurPasses = 1
	}
}

func (t *Theme) normalizeFonts() {
	f := &t.Fonts

	if f.UIFamily == "" {
		f.UIFamily = "sans-serif"
	}
	if f.MonoFamily == "" {
		f.MonoFamily = "monospace"
	}
	if f.UISize == 0 {
		f.UISize = 11
	}
	if f.MonoSize == 0 {
		f.MonoSize = f.UISize
	}
}
