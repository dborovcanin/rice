package theme

// Normalize fills in every value an adapter may reference but a theme author
// is allowed to omit. It is idempotent, and always runs before validation so
// that validation errors describe the effective theme, not the file.
func (t *Theme) Normalize() {
	c := &t.Colors

	if c.Surface.IsZero() {
		c.Surface = c.Background.Lighten(0.04)
	}
	if c.SurfaceAlt.IsZero() {
		c.SurfaceAlt = c.Surface.Lighten(0.06)
	}
	if c.Overlay.IsZero() {
		c.Overlay = c.SurfaceAlt.Lighten(0.06)
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

	derivedRegular := [8]Color{
		c.Background,
		c.Error,
		c.Success,
		c.Warning,
		c.Primary,
		c.Secondary,
		c.Accent,
		c.Foreground.Darken(0.15),
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
			term.Bright[i] = term.Regular[i].Lighten(0.2)
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
