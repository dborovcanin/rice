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
