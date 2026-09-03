package theme

// Theme is the normalized appearance model every adapter renders from. It is
// deliberately application-agnostic: anything Sway- or Waybar-specific belongs
// in config, not here.
type Theme struct {
	Name        string `toml:"name"`
	Description string `toml:"description,omitempty"`
	Variant     string `toml:"variant,omitempty"`

	Colors   Colors   `toml:"colors,omitempty"`
	Terminal Terminal `toml:"terminal,omitempty"`
	UI       UI       `toml:"ui,omitempty"`
	Fonts    Fonts    `toml:"fonts,omitempty"`
	Icons    Icons    `toml:"icons,omitempty"`
	Cursor   Cursor   `toml:"cursor,omitempty"`
	GTK      GTK      `toml:"gtk,omitempty"`
}

// Colors is the semantic palette. Applications never see raw ANSI indexes
// except through Terminal.
type Colors struct {
	Background Color `toml:"background,omitempty"`
	Surface    Color `toml:"surface,omitempty"`
	SurfaceAlt Color `toml:"surface_alt,omitempty"`
	Overlay    Color `toml:"overlay,omitempty"`

	Foreground Color `toml:"foreground,omitempty"`
	Muted      Color `toml:"muted,omitempty"`

	Primary   Color `toml:"primary,omitempty"`
	Secondary Color `toml:"secondary,omitempty"`
	Accent    Color `toml:"accent,omitempty"`

	Success Color `toml:"success,omitempty"`
	Warning Color `toml:"warning,omitempty"`
	Error   Color `toml:"error,omitempty"`

	Border      Color `toml:"border,omitempty"`
	BorderFocus Color `toml:"border_focus,omitempty"`
}

// Terminal is the 16-color ANSI palette plus terminal-specific extras. Any
// field left unset is derived from Colors by Normalize.
type Terminal struct {
	Foreground Color `toml:"foreground,omitempty"`
	Background Color `toml:"background,omitempty"`

	Regular [8]Color `toml:"regular,omitempty"`
	Bright  [8]Color `toml:"bright,omitempty"`

	SelectionForeground Color `toml:"selection_foreground,omitempty"`
	SelectionBackground Color `toml:"selection_background,omitempty"`
	Cursor              Color `toml:"cursor,omitempty"`
	URL                 Color `toml:"url,omitempty"`
}

// UI holds geometry shared across the desktop.
type UI struct {
	Radius      int `toml:"radius,omitempty"`
	BorderWidth int `toml:"border_width,omitempty"`

	GapsInner int `toml:"gaps_inner,omitempty"`
	GapsOuter int `toml:"gaps_outer,omitempty"`

	Padding           int `toml:"padding,omitempty"`
	HorizontalPadding int `toml:"horizontal_padding,omitempty"`

	Opacity     float64 `toml:"opacity,omitempty"`
	BlurRadius  int     `toml:"blur_radius,omitempty"`
	BlurPasses  int     `toml:"blur_passes,omitempty"`
	BlurNoise   float64 `toml:"blur_noise,omitempty"`
	ShadowBlur  int     `toml:"shadow_blur,omitempty"`
	DimInactive float64 `toml:"dim_inactive,omitempty"`
}

// Border is the desktop's border as one value: the width the compositor draws
// around a window, the colour it draws it in, the colour a focused window
// gets, and the radius it rounds the corners to.
//
// It is assembled from the palette and the shared geometry rather than stored
// beside them, so the desktop has one border rather than four that drift
// apart. A surface the compositor does not decorate — a bar, a launcher, a
// notification — draws its own frame, and takes this one so that it matches.
type Border struct {
	Width  int
	Color  Color
	Focus  Color
	Radius int
}

// BorderNone is the width, or the radius, that means "draw none of it".
//
// Zero means "unset" everywhere in the theme format — it is what tells
// normalization to fill a value in, and what tells an application to follow
// the desktop — so turning a border off needs a value of its own. Anything
// negative reads as none and reaches a template as zero.
const BorderNone = -1

// Border is what the compositor draws around a window.
func (t Theme) Border() Border {
	return Border{
		Width:  t.UI.BorderWidth,
		Color:  t.Colors.Border,
		Focus:  t.Colors.BorderFocus,
		Radius: t.UI.Radius,
	}.Drawn()
}

// Drawn is the border as a template sees it: a width or radius asking for no
// border arrives as zero, so nothing has to know about the sentinel except
// the person who typed it.
func (b Border) Drawn() Border {
	b.Width = max(b.Width, 0)
	b.Radius = max(b.Radius, 0)
	return b
}

// Focused is the same border in the colour a focused window gets. A launcher
// is the surface you are looking at when it is open, so it follows this rather
// than the resting colour.
func (b Border) Focused() Border {
	b.Color = b.Focus
	return b
}

// Fonts describes the two font roles Rice cares about: interface chrome and
// monospaced terminal text.
type Fonts struct {
	UIFamily string `toml:"ui_family,omitempty"`
	UISize   int    `toml:"ui_size,omitempty"`

	MonoFamily string `toml:"mono_family,omitempty"`
	MonoSize   int    `toml:"mono_size,omitempty"`

	BarFamily string `toml:"bar_family,omitempty"`
	BarSize   int    `toml:"bar_size,omitempty"`
}

// Icons selects the icon theme, the size toolkits and launchers should draw
// icons at, and the lookup paths Dunst needs.
type Icons struct {
	Theme string   `toml:"theme,omitempty"`
	Size  int      `toml:"size,omitempty"`
	Paths []string `toml:"paths,omitempty"`
}

// Cursor selects the pointer theme and its size in logical pixels.
type Cursor struct {
	Theme string `toml:"theme,omitempty"`
	Size  int    `toml:"size,omitempty"`
}

// GTK carries toolkit hints that have no equivalent in the semantic palette.
type GTK struct {
	Theme           string `toml:"theme,omitempty"`
	PreferDark      bool   `toml:"prefer_dark,omitempty"`
	KvantumTheme    string `toml:"kvantum_theme,omitempty"`
	QtStyleOverride string `toml:"qt_style_override,omitempty"`
}

// IsDark reports whether the theme reads as a dark theme. An explicit variant
// wins; otherwise the background brightness decides.
func (t Theme) IsDark() bool {
	switch t.Variant {
	case "dark":
		return true
	case "light":
		return false
	}
	return t.Colors.Background.IsDark()
}

// BarFont returns the bar font family, falling back to the UI font.
func (f Fonts) BarFont() string {
	if f.BarFamily != "" {
		return f.BarFamily
	}
	return f.UIFamily
}

// BarFontSize returns the bar font size, falling back to the UI size.
func (f Fonts) BarFontSize() int {
	if f.BarSize != 0 {
		return f.BarSize
	}
	return f.UISize
}
