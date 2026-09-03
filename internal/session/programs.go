package session

import (
	"strconv"
	"sync"

	"github.com/dborovcanin/rice/internal/assets"
)

// ProgramFields returns the settings that belong to one program rather than to
// the theme. They live in config.toml, because they are structure rather than
// appearance: a bar's height and a launcher's width are decisions about the
// desktop, not about the palette, and they should survive a theme change.
//
// This is a curated set, not every key in config.toml. Outputs, workspaces,
// bindings and window rules are lists, and editing a list well needs an
// interface of its own; until that exists, they stay in the file where they are
// already comfortable to edit by hand.
func ProgramFields(component string) []Field {
	programOnce.Do(buildProgramFields)
	return programFields[component]
}

// ProgramFieldsExist reports whether a component has anything to edit.
func ProgramFieldsExist(component string) bool { return len(ProgramFields(component)) > 0 }

var (
	programOnce   sync.Once
	programFields map[string][]Field
)

func buildProgramFields() {
	programFields = map[string][]Field{
		"waybar":   waybarFields(),
		"rofi":     rofiFields(),
		"foot":     footFields(),
		"dunst":    dunstFields(),
		"swaylock": swaylockFields(),
		"gtk":      gtkFields(),
		"qt":       qtFields(),
	}

	for _, fields := range programFields {
		for _, f := range fields {
			programKeys[f.Key] = f
		}
	}
}

// notes explain a program whose settings are not where you would look for
// them. Sway is the desktop rather than an application on it, so its settings
// are in the global SwayFX section.
var notes = map[string]string{
	"sway": "the compositor's settings are in the global SwayFX section",
	"gtk":  "how GTK applications look comes from the theme; these are the files Rice writes",
	"qt":   "how Qt applications look comes from the theme; these are the files Rice writes",
}

// Note is a line explaining a program whose settings are elsewhere, or empty.
func Note(component string) string { return notes[component] }

// programKeys maps a field key to its field, so a lookup by key works for
// program fields as well as theme fields.
var programKeys = map[string]Field{}

// lookupProgramField finds a program field by key.
func lookupProgramField(key string) (Field, bool) {
	programOnce.Do(buildProgramFields)
	f, ok := programKeys[key]
	return f, ok
}

// Small constructors, so the tables below read as data.

// These build configuration fields, so they set StoreConfig explicitly.
// StoreTheme is the zero value, and a field saved to the wrong file is silently
// lost rather than loudly wrong.

func pInt(key, label, help string, min, max, step float64, get func(*Draft) *int) Field {
	return Field{
		Key: key, Label: label, Help: help, Kind: KindInt, Store: StoreConfig,
		Min: min, Max: max, Step: step, num: get,
	}
}

func pBool(key, label, help string, get func(*Draft) *bool) Field {
	return Field{Key: key, Label: label, Help: help, Kind: KindBool, Store: StoreConfig, flag: get}
}

func pText(key, label, help string, get func(*Draft) *string) Field {
	return Field{Key: key, Label: label, Help: help, Kind: KindText, Store: StoreConfig, text: get}
}

// override marks a field whose unset value means "follow the theme", and says
// what the theme would give. Without the fallback the row reads as blank, and
// the launcher's actual font becomes something you have to know rather than
// something you can see.
func override(f Field, fallback func(Draft) string) Field {
	f.Derives = true
	f.fallback = fallback
	return f
}

// pickFont turns a text field into one picked from the installed families.
func pickFont(f Field) Field {
	f.Kind = KindFont
	return f
}

// pickIconTheme turns a text field into one picked from the installed icon
// themes.
func pickIconTheme(f Field) Field {
	f.Assets, f.PicksAssets = assets.IconThemes, true
	return f
}

func pChoice(key, label, help string, choices []string, get func(*Draft) *string) Field {
	return Field{
		Key: key, Label: label, Help: help, Kind: KindChoice,
		Store: StoreConfig, Choices: choices, text: get,
	}
}

func waybarFields() []Field {
	return []Field{
		pChoice("waybar.position", "Position", "which edge the bar sits on",
			[]string{"top", "bottom", "left", "right"},
			func(d *Draft) *string { return &d.Config.Waybar.Position }),
		pChoice("waybar.layer", "Layer", "where the bar sits in the stack",
			[]string{"top", "bottom", "overlay"},
			func(d *Draft) *string { return &d.Config.Waybar.Layer }),
		pInt("waybar.height", "Height", "bar height in pixels; 0 fits the content", 0, 200, 1,
			func(d *Draft) *int { return &d.Config.Waybar.Height }),
		pInt("waybar.spacing", "Spacing", "gap between modules, in pixels", 0, 64, 1,
			func(d *Draft) *int { return &d.Config.Waybar.Spacing }),
	}
}

func rofiFields() []Field {
	return []Field{
		pText("rofi.width", "Width", "any rasi width, such as 40% or 600px",
			func(d *Draft) *string { return &d.Config.Rofi.Width }),
		pInt("rofi.lines", "Lines", "rows shown before scrolling", 1, 64, 1,
			func(d *Draft) *int { return &d.Config.Rofi.Lines }),
		pInt("rofi.columns", "Columns", "how many columns of results", 1, 8, 1,
			func(d *Draft) *int { return &d.Config.Rofi.Columns }),
		pBool("rofi.show_icons", "Show icons", "",
			func(d *Draft) *bool { return &d.Config.Rofi.ShowIcons }),
		// These four override the theme for the launcher alone. They are
		// declared as overrides — unset means "follow the theme" — so the
		// editor marks them derived, shows what the theme would give, and
		// clears them back with `c`. They pick from the same lists as the
		// global font and icon-theme fields, because they hold the same kind
		// of value.
		override(pickIconTheme(pText("rofi.icon_theme", "Icon theme",
			"empty follows the theme's icon set",
			func(d *Draft) *string { return &d.Config.Rofi.IconTheme })),
			func(d Draft) string { return d.Theme.Icons.Theme }),

		override(pickFont(pText("rofi.font_family", "Font family",
			"empty follows the theme's interface font",
			func(d *Draft) *string { return &d.Config.Rofi.FontFamily })),
			func(d Draft) string { return d.Theme.Fonts.UIFamily }),

		override(pInt("rofi.font_size", "Font size", "0 follows the theme's interface size", 0, 96, 1,
			func(d *Draft) *int { return &d.Config.Rofi.FontSize }),
			func(d Draft) string { return strconv.Itoa(d.Theme.Fonts.UISize) }),

		override(pInt("rofi.icon_size", "Icon size", "0 follows the theme's icon size", 0, 256, 2,
			func(d *Draft) *int { return &d.Config.Rofi.IconSize }),
			func(d Draft) string { return strconv.Itoa(d.Theme.Icons.Size) }),
		pText("rofi.display_drun", "Drun label", "the prompt shown in application mode",
			func(d *Draft) *string { return &d.Config.Rofi.DisplayDrun }),
	}
}

// footFields leads with the shared ANSI palette: it is what a terminal is for,
// and it is the first thing anyone wants to change about one.
func footFields() []Field {
	behaviour := []Field{
		pBool("foot.server", "Server mode", "generate configuration for footserver and footclient",
			func(d *Draft) *bool { return &d.Config.Foot.Server }),
		pText("foot.shell", "Shell", "empty means the login shell",
			func(d *Draft) *string { return &d.Config.Foot.Shell }),
		pText("foot.term", "TERM", "the terminfo name to advertise",
			func(d *Draft) *string { return &d.Config.Foot.Term }),
		pInt("foot.scrollback_lines", "Scrollback", "lines of history kept", 0, 1000000, 1000,
			func(d *Draft) *int { return &d.Config.Foot.ScrollbackLines }),
		pInt("foot.pad_x", "Pad x", "horizontal padding in pixels", 0, 128, 1,
			func(d *Draft) *int { return &d.Config.Foot.PadX }),
		pInt("foot.pad_y", "Pad y", "vertical padding in pixels", 0, 128, 1,
			func(d *Draft) *int { return &d.Config.Foot.PadY }),
		pChoice("foot.cursor_style", "Cursor style", "",
			[]string{"block", "beam", "underline"},
			func(d *Draft) *string { return &d.Config.Foot.CursorStyle }),
		pBool("foot.cursor_blink", "Cursor blink", "",
			func(d *Draft) *bool { return &d.Config.Foot.CursorBlink }),
	}

	return append(TerminalFields(), behaviour...)
}

func gtkFields() []Field {
	return []Field{
		pBool("gtk.settings", "Settings file", "write gtk-3.0 and gtk-4.0 settings.ini",
			func(d *Draft) *bool { return &d.Config.GTK.Settings }),
		pBool("gtk.css", "Palette stylesheet", "map the palette onto libadwaita's named colors",
			func(d *Draft) *bool { return &d.Config.GTK.CSS }),
		pBool("sway.write_environment", "Session environment", "write environment.d/50-rice.conf; needs a re-login",
			func(d *Draft) *bool { return &d.Config.Sway.WriteEnvironment }),
	}
}

func qtFields() []Field {
	return []Field{
		pBool("qt.qt5ct", "qt5ct", "write the Qt 5 platform theme",
			func(d *Draft) *bool { return &d.Config.Qt.Qt5ct }),
		pBool("qt.qt6ct", "qt6ct", "write the Qt 6 platform theme",
			func(d *Draft) *bool { return &d.Config.Qt.Qt6ct }),
		pBool("qt.kvantum", "Kvantum", "select the theme's Kvantum theme",
			func(d *Draft) *bool { return &d.Config.Qt.Kvantum }),
		pChoice("qt.platform_theme", "Platform theme", "what QT_QPA_PLATFORMTHEME is set to",
			[]string{"qt5ct", "qt6ct", "gtk2", "gnome"},
			func(d *Draft) *string { return &d.Config.Qt.PlatformTheme }),
	}
}

func dunstFields() []Field {
	return []Field{
		pChoice("dunst.origin", "Origin", "which corner notifications appear in",
			[]string{
				"top-left", "top-center", "top-right",
				"left-center", "center", "right-center",
				"bottom-left", "bottom-center", "bottom-right",
			},
			func(d *Draft) *string { return &d.Config.Dunst.Origin }),
		pInt("dunst.width", "Width", "notification width in pixels", 0, 2000, 10,
			func(d *Draft) *int { return &d.Config.Dunst.Width }),
		pInt("dunst.height", "Height", "maximum notification height in pixels", 0, 2000, 10,
			func(d *Draft) *int { return &d.Config.Dunst.Height }),
		pText("dunst.offset", "Offset", "distance from the origin, as \"x,y\"",
			func(d *Draft) *string { return &d.Config.Dunst.Offset }),
		pInt("dunst.gap_size", "Gap size", "space between stacked notifications", 0, 64, 1,
			func(d *Draft) *int { return &d.Config.Dunst.GapSize }),
		pChoice("dunst.follow", "Follow", "which display notifications appear on",
			[]string{"mouse", "keyboard", "none"},
			func(d *Draft) *string { return &d.Config.Dunst.Follow }),
		pInt("dunst.timeout_low", "Timeout low", "seconds; 0 never expires", 0, 300, 1,
			func(d *Draft) *int { return &d.Config.Dunst.TimeoutLow }),
		pInt("dunst.timeout_normal", "Timeout normal", "seconds; 0 never expires", 0, 300, 1,
			func(d *Draft) *int { return &d.Config.Dunst.TimeoutNormal }),
		pInt("dunst.timeout_critical", "Timeout critical", "seconds; 0 never expires", 0, 300, 1,
			func(d *Draft) *int { return &d.Config.Dunst.TimeoutCritical }),
		pInt("dunst.max_icon_size", "Max icon size", "pixels", 0, 256, 4,
			func(d *Draft) *int { return &d.Config.Dunst.MaxIconSize }),
		// Overrides of the theme's interface font, for notifications alone.
		// Unset means "follow the theme", so the editor shows what the theme
		// would give rather than a blank, and `c` puts it back.
		override(pickFont(pText("dunst.font_family", "Font family",
			"empty follows the theme's interface font",
			func(d *Draft) *string { return &d.Config.Dunst.FontFamily })),
			func(d Draft) string { return d.Theme.Fonts.UIFamily }),

		override(pInt("dunst.font_size", "Font size", "0 follows the theme's interface size", 0, 96, 1,
			func(d *Draft) *int { return &d.Config.Dunst.FontSize }),
			func(d Draft) string { return strconv.Itoa(d.Theme.Fonts.UISize) }),
	}
}

func swaylockFields() []Field {
	return []Field{
		pText("swaylock.image", "Image", "lock screen background; empty uses the colors",
			func(d *Draft) *string { return &d.Config.Swaylock.Image }),
		pChoice("swaylock.scaling_mode", "Scaling", "how the image fills the screen",
			[]string{"stretch", "fill", "fit", "center", "tile", "solid_color"},
			func(d *Draft) *string { return &d.Config.Swaylock.ScalingMode }),
		pText("swaylock.blur", "Blur", "swaylock blur, as \"radiusxpasses\"",
			func(d *Draft) *string { return &d.Config.Swaylock.Blur }),
		pInt("swaylock.indicator_radius", "Indicator radius", "pixels", 0, 500, 5,
			func(d *Draft) *int { return &d.Config.Swaylock.IndicatorRadius }),
		pInt("swaylock.indicator_thickness", "Indicator thickness", "pixels", 0, 100, 1,
			func(d *Draft) *int { return &d.Config.Swaylock.IndicatorThickness }),
		pBool("swaylock.show_failed_attempts", "Show failed attempts", "",
			func(d *Draft) *bool { return &d.Config.Swaylock.ShowFailed }),
		pBool("swaylock.clock", "Clock", "show the time on the lock screen",
			func(d *Draft) *bool { return &d.Config.Swaylock.Clock }),
	}
}
