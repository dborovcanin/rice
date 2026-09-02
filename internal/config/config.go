package config

// Config is the user's source configuration: everything that is not
// appearance. Appearance lives in a theme, structure lives here.
type Config struct {
	Theme string `toml:"theme"`

	Components  Components  `toml:"components"`
	Generations Generations `toml:"generations"`
	Commands    Commands    `toml:"commands"`

	Sway     Sway     `toml:"sway"`
	Waybar   Waybar   `toml:"waybar"`
	Rofi     Rofi     `toml:"rofi"`
	Foot     Foot     `toml:"foot"`
	Dunst    Dunst    `toml:"dunst"`
	Swaylock Swaylock `toml:"swaylock"`
	GTK      GTK      `toml:"gtk"`
	Qt       Qt       `toml:"qt"`
}

// Components selects which applications Rice generates configuration for.
type Components struct {
	Sway     bool `toml:"sway"`
	Waybar   bool `toml:"waybar"`
	Rofi     bool `toml:"rofi"`
	Foot     bool `toml:"foot"`
	Dunst    bool `toml:"dunst"`
	Swaylock bool `toml:"swaylock"`
	GTK      bool `toml:"gtk"`
	Qt       bool `toml:"qt"`
}

// Enabled reports whether a component name is turned on.
func (c Components) Enabled(name string) bool {
	switch name {
	case "sway":
		return c.Sway
	case "waybar":
		return c.Waybar
	case "rofi":
		return c.Rofi
	case "foot":
		return c.Foot
	case "dunst":
		return c.Dunst
	case "swaylock":
		return c.Swaylock
	case "gtk":
		return c.GTK
	case "qt":
		return c.Qt
	}
	return false
}

// Generations controls how much history Rice keeps.
type Generations struct {
	Keep int `toml:"keep"`
}

// Commands are the programs Rice binds to and launches. Keeping them here
// means a template never hardcodes a binary name.
type Commands struct {
	Terminal       string `toml:"terminal"`
	TerminalSpawn  string `toml:"terminal_spawn"`
	Launcher       string `toml:"launcher"`
	Browser        string `toml:"browser"`
	Editor         string `toml:"editor"`
	Lock           string `toml:"lock"`
	Screenshot     string `toml:"screenshot"`
	Clipboard      string `toml:"clipboard"`
	Emoji          string `toml:"emoji"`
	VolumeUp       string `toml:"volume_up"`
	VolumeDown     string `toml:"volume_down"`
	VolumeMute     string `toml:"volume_mute"`
	MicMute        string `toml:"mic_mute"`
	BrightnessUp   string `toml:"brightness_up"`
	BrightnessDown string `toml:"brightness_down"`
}

// Sway describes the compositor: geometry, inputs, outputs and bindings.
type Sway struct {
	Mod           string `toml:"mod"`
	Wallpaper     string `toml:"wallpaper"`
	WallpaperMode string `toml:"wallpaper_mode"`

	SmartBorders      bool `toml:"smart_borders"`
	SmartGaps         bool `toml:"smart_gaps"`
	FocusFollowsMouse bool `toml:"focus_follows_mouse"`
	Titlebar          bool `toml:"titlebar"`

	Outputs     []Output      `toml:"outputs"`
	Workspaces  []Workspace   `toml:"workspaces"`
	Bindings    []Binding     `toml:"bindings"`
	Modes       []Mode        `toml:"modes"`
	WindowRules []WindowRule  `toml:"window_rules"`
	Assigns     []Assign      `toml:"assigns"`
	Startup     []StartupItem `toml:"startup"`

	Keyboard Keyboard `toml:"keyboard"`
	Touchpad Touchpad `toml:"touchpad"`
	Idle     Idle     `toml:"idle"`

	// WriteEnvironment writes environment.d/50-rice.conf, which is where the
	// cursor and the Qt platform theme have to be set: Sway's configuration
	// language cannot export a variable, and a generated qt5ct.conf that
	// nothing points QT_QPA_PLATFORMTHEME at is inert.
	//
	// The file is read by the systemd user manager at login, so a change to it
	// needs a fresh session rather than a reload.
	WriteEnvironment bool `toml:"write_environment"`
	// Environment is extra variables to put in that file.
	Environment map[string]string `toml:"environment"`

	Extra string `toml:"extra"`
}

// Output is a display and how it is positioned.
type Output struct {
	Name       string  `toml:"name"`
	Resolution string  `toml:"resolution"`
	Position   string  `toml:"position"`
	Scale      float64 `toml:"scale"`
	Transform  string  `toml:"transform"`
	Wallpaper  string  `toml:"wallpaper"`
	Disabled   bool    `toml:"disabled"`
}

// Workspace binds a key to a named workspace, optionally pinned to an output.
type Workspace struct {
	Key    string `toml:"key"`
	Name   string `toml:"name"`
	Output string `toml:"output"`
}

// Binding is one keybinding. Command is written verbatim after the keys, so
// it may be any Sway command, not only "exec".
type Binding struct {
	Keys    string `toml:"keys"`
	Command string `toml:"command"`
	Comment string `toml:"comment"`
}

// Mode is a Sway binding mode such as "resize" or a system menu.
type Mode struct {
	Name     string    `toml:"name"`
	Enter    string    `toml:"enter"`
	Bindings []Binding `toml:"bindings"`
}

// WindowRule is a for_window rule: criteria plus the commands to apply.
type WindowRule struct {
	Criteria string `toml:"criteria"`
	Commands string `toml:"commands"`
}

// Assign pins matching windows to a workspace.
type Assign struct {
	Criteria  string `toml:"criteria"`
	Workspace string `toml:"workspace"`
}

// StartupItem is a program launched with the session. Always uses exec_always,
// so it is re-run on every Sway reload.
type StartupItem struct {
	Command string `toml:"command"`
	Always  bool   `toml:"always"`
	Comment string `toml:"comment"`
}

// Keyboard is the xkb configuration for every keyboard.
type Keyboard struct {
	Layout      string `toml:"layout"`
	Variant     string `toml:"variant"`
	Options     string `toml:"options"`
	RepeatDelay int    `toml:"repeat_delay"`
	RepeatRate  int    `toml:"repeat_rate"`
}

// Touchpad is the libinput configuration for every touchpad.
type Touchpad struct {
	Tap                bool    `toml:"tap"`
	TapButtonMap       string  `toml:"tap_button_map"`
	NaturalScroll      bool    `toml:"natural_scroll"`
	DisableWhileTyping bool    `toml:"disable_while_typing"`
	MiddleEmulation    bool    `toml:"middle_emulation"`
	DragLock           bool    `toml:"drag_lock"`
	AccelProfile       string  `toml:"accel_profile"`
	PointerAccel       float64 `toml:"pointer_accel"`
}

// Idle configures swayidle timeouts, in seconds. Zero disables a timeout.
type Idle struct {
	Enabled     bool `toml:"enabled"`
	LockAfter   int  `toml:"lock_after"`
	ScreenOff   int  `toml:"screen_off"`
	SleepAfter  int  `toml:"sleep_after"`
	LockOnSleep bool `toml:"lock_on_sleep"`
}

// Waybar describes the bar layout. Modules holds raw per-module settings that
// are emitted as JSON, so any Waybar module option is reachable without Rice
// having to model it.
type Waybar struct {
	Position string `toml:"position"`
	Layer    string `toml:"layer"`
	Height   int    `toml:"height"`
	Spacing  int    `toml:"spacing"`

	ModulesLeft   []string `toml:"modules_left"`
	ModulesCenter []string `toml:"modules_center"`
	ModulesRight  []string `toml:"modules_right"`

	Modules map[string]map[string]any `toml:"modules"`

	ExtraCSS string `toml:"extra_css"`
}

// Rofi describes launcher geometry. Colors come from the theme.
type Rofi struct {
	Width       string   `toml:"width"`
	Lines       int      `toml:"lines"`
	Columns     int      `toml:"columns"`
	IconTheme   string   `toml:"icon_theme"`
	ShowIcons   bool     `toml:"show_icons"`
	Modes       []string `toml:"modes"`
	DisplayDrun string   `toml:"display_drun"`
	Extra       string   `toml:"extra"`
}

// Foot describes terminal behaviour. Palette comes from the theme.
type Foot struct {
	Server          bool   `toml:"server"`
	Shell           string `toml:"shell"`
	Term            string `toml:"term"`
	ScrollbackLines int    `toml:"scrollback_lines"`
	PadX            int    `toml:"pad_x"`
	PadY            int    `toml:"pad_y"`
	CursorStyle     string `toml:"cursor_style"`
	CursorBlink     bool   `toml:"cursor_blink"`
	Extra           string `toml:"extra"`
}

// Dunst describes notification layout and behaviour.
type Dunst struct {
	Origin          string `toml:"origin"`
	Width           int    `toml:"width"`
	Height          int    `toml:"height"`
	Offset          string `toml:"offset"`
	GapSize         int    `toml:"gap_size"`
	Follow          string `toml:"follow"`
	TimeoutLow      int    `toml:"timeout_low"`
	TimeoutNormal   int    `toml:"timeout_normal"`
	TimeoutCritical int    `toml:"timeout_critical"`
	MaxIconSize     int    `toml:"max_icon_size"`
	Extra           string `toml:"extra"`
}

// GTK selects which toolkit files Rice writes. GTK has no global icon-size
// setting, so the theme's icons.size does not reach it: what Rice can set is
// the theme, the icon theme, the font, the cursor and the dark preference.
type GTK struct {
	// Settings writes gtk-3.0/settings.ini and gtk-4.0/settings.ini.
	Settings bool `toml:"settings"`
	// CSS writes a small gtk.css that maps the palette onto the libadwaita
	// named colors, so GTK4 applications pick up the accent and the window
	// background. GTK3 applications honour far less of it.
	CSS bool `toml:"css"`
	// ExtraCSS is appended to the generated gtk.css.
	ExtraCSS string `toml:"extra_css"`
}

// Qt describes the Qt platform theme files. Kvantum and the icon theme come
// from the theme's [gtk] block, which carries the toolkit hints.
type Qt struct {
	// Qt5ct and Qt6ct write the matching platform theme configuration.
	Qt5ct bool `toml:"qt5ct"`
	Qt6ct bool `toml:"qt6ct"`
	// Kvantum writes Kvantum/kvantum.kvconfig selecting the theme's Kvantum
	// theme. It has no effect unless the Kvantum style is installed.
	Kvantum bool `toml:"kvantum"`
	// PlatformTheme is what QT_QPA_PLATFORMTHEME should be set to. It is not
	// exported by Rice; it is written into the Sway session environment.
	PlatformTheme string `toml:"platform_theme"`
}

// Swaylock describes the lock screen.
type Swaylock struct {
	Image              string `toml:"image"`
	ScalingMode        string `toml:"scaling_mode"`
	Blur               string `toml:"blur"`
	IndicatorRadius    int    `toml:"indicator_radius"`
	IndicatorThickness int    `toml:"indicator_thickness"`
	ShowFailed         bool   `toml:"show_failed_attempts"`
	Clock              bool   `toml:"clock"`
	Extra              string `toml:"extra"`
}
