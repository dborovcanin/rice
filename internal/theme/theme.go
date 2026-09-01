package theme

// Theme is the normalized appearance model every adapter renders from. It is
// deliberately application-agnostic: anything Sway- or Waybar-specific belongs
// in config, not here.
type Theme struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Variant     string `toml:"variant"`

	Colors   Colors   `toml:"colors"`
	Terminal Terminal `toml:"terminal"`
	UI       UI       `toml:"ui"`
	Fonts    Fonts    `toml:"fonts"`
	Icons    Icons    `toml:"icons"`
	Cursor   Cursor   `toml:"cursor"`
	GTK      GTK      `toml:"gtk"`
}

// Colors is the semantic palette. Applications never see raw ANSI indexes
// except through Terminal.
type Colors struct {
	Background Color `toml:"background"`
	Surface    Color `toml:"surface"`
	SurfaceAlt Color `toml:"surface_alt"`
	Overlay    Color `toml:"overlay"`

	Foreground Color `toml:"foreground"`
	Muted      Color `toml:"muted"`

	Primary   Color `toml:"primary"`
	Secondary Color `toml:"secondary"`
	Accent    Color `toml:"accent"`

	Success Color `toml:"success"`
	Warning Color `toml:"warning"`
	Error   Color `toml:"error"`

	Border      Color `toml:"border"`
	BorderFocus Color `toml:"border_focus"`
}

// Terminal is the 16-color ANSI palette plus terminal-specific extras. Any
// field left unset is derived from Colors by Normalize.
type Terminal struct {
	Foreground Color `toml:"foreground"`
	Background Color `toml:"background"`

	Regular [8]Color `toml:"regular"`
	Bright  [8]Color `toml:"bright"`

	SelectionForeground Color `toml:"selection_foreground"`
	SelectionBackground Color `toml:"selection_background"`
	Cursor              Color `toml:"cursor"`
	URL                 Color `toml:"url"`
}

// UI holds geometry shared across the desktop.
type UI struct {
	Radius      int `toml:"radius"`
	BorderWidth int `toml:"border_width"`

	GapsInner int `toml:"gaps_inner"`
	GapsOuter int `toml:"gaps_outer"`

	Padding           int `toml:"padding"`
	HorizontalPadding int `toml:"horizontal_padding"`

	Opacity     float64 `toml:"opacity"`
	BlurRadius  int     `toml:"blur_radius"`
	BlurPasses  int     `toml:"blur_passes"`
	BlurNoise   float64 `toml:"blur_noise"`
	ShadowBlur  int     `toml:"shadow_blur"`
	DimInactive float64 `toml:"dim_inactive"`
}

// Fonts describes the two font roles Rice cares about: interface chrome and
// monospaced terminal text.
type Fonts struct {
	UIFamily string `toml:"ui_family"`
	UISize   int    `toml:"ui_size"`

	MonoFamily string `toml:"mono_family"`
	MonoSize   int    `toml:"mono_size"`

	BarFamily string `toml:"bar_family"`
	BarSize   int    `toml:"bar_size"`
}

// Icons selects the icon theme and the lookup paths Dunst needs.
type Icons struct {
	Theme string   `toml:"theme"`
	Paths []string `toml:"paths"`
}

// Cursor selects the pointer theme and its size in logical pixels.
type Cursor struct {
	Theme string `toml:"theme"`
	Size  int    `toml:"size"`
}

// GTK carries toolkit hints that have no equivalent in the semantic palette.
type GTK struct {
	Theme           string `toml:"theme"`
	PreferDark      bool   `toml:"prefer_dark"`
	KvantumTheme    string `toml:"kvantum_theme"`
	QtStyleOverride string `toml:"qt_style_override"`
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
