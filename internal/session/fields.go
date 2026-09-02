package session

import (
	"fmt"
	"sync"

	"github.com/dborovcanin/rice/internal/assets"
	"github.com/dborovcanin/rice/internal/theme"
)

// Fields returns every editable field, in presentation order. The table is
// built once: it is static, and an editor asks for it on every redraw.
func Fields() []Field {
	fieldsOnce.Do(buildFields)
	return allFields
}

// FieldsIn returns the fields belonging to one group.
func FieldsIn(g Group) []Field {
	fieldsOnce.Do(buildFields)
	return grouped[g]
}

// EveryField is every editable field, global and per-program. Dirty tracking
// walks this: a field's Store says which file it is saved to, and that is not
// the same question as which table it was declared in.
func EveryField() []Field {
	fieldsOnce.Do(buildFields)
	programOnce.Do(buildProgramFields)

	out := make([]Field, 0, len(allFields)+len(programKeys))
	out = append(out, allFields...)
	for _, f := range programKeys {
		out = append(out, f)
	}
	return out
}

// LookupField finds a field by key, in the global theme table or in a
// program's table. Keys are unique across both, so every operation on a field
// works the same way whichever table it came from.
func LookupField(key string) (Field, bool) {
	fieldsOnce.Do(buildFields)
	if f, ok := byKey[key]; ok {
		return f, true
	}
	return lookupProgramField(key)
}

var (
	fieldsOnce sync.Once
	allFields  []Field
	grouped    map[Group][]Field
	byKey      map[string]Field
)

func buildFields() {
	allFields = append(allFields, colorFields()...)
	allFields = append(allFields, fontFields()...)
	allFields = append(allFields, swayFXFields()...)
	allFields = append(allFields, iconFields()...)

	grouped = make(map[Group][]Field, len(Groups()))
	byKey = make(map[string]Field, len(allFields))
	for _, f := range allFields {
		grouped[f.Group] = append(grouped[f.Group], f)
		byKey[f.Key] = f
	}
}

func colorFields() []Field {
	c := func(key, label string, get func(*Draft) *theme.Color) Field {
		return Field{Key: key, Label: label, Group: GroupColors, Kind: KindColor, Derives: true, color: get}
	}
	return []Field{
		c("colors.background", "Background", func(d *Draft) *theme.Color { return &d.Theme.Colors.Background }),
		c("colors.surface", "Surface", func(d *Draft) *theme.Color { return &d.Theme.Colors.Surface }),
		c("colors.surface_alt", "Surface alt", func(d *Draft) *theme.Color { return &d.Theme.Colors.SurfaceAlt }),
		c("colors.overlay", "Overlay", func(d *Draft) *theme.Color { return &d.Theme.Colors.Overlay }),
		c("colors.foreground", "Foreground", func(d *Draft) *theme.Color { return &d.Theme.Colors.Foreground }),
		c("colors.muted", "Muted", func(d *Draft) *theme.Color { return &d.Theme.Colors.Muted }),
		c("colors.primary", "Primary", func(d *Draft) *theme.Color { return &d.Theme.Colors.Primary }),
		c("colors.secondary", "Secondary", func(d *Draft) *theme.Color { return &d.Theme.Colors.Secondary }),
		c("colors.accent", "Accent", func(d *Draft) *theme.Color { return &d.Theme.Colors.Accent }),
		c("colors.success", "Success", func(d *Draft) *theme.Color { return &d.Theme.Colors.Success }),
		c("colors.warning", "Warning", func(d *Draft) *theme.Color { return &d.Theme.Colors.Warning }),
		c("colors.error", "Error", func(d *Draft) *theme.Color { return &d.Theme.Colors.Error }),
	}
}

// ansiNames label the sixteen terminal slots the way terminal documentation
// does, because "regular 4" means nothing on its own.
var ansiNames = [8]string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white"}

// TerminalFields is the shared ANSI palette. It is not a global group: only a
// terminal reads it, so it belongs to the terminal. Any second terminal
// emulator would show these same fields, because there is one palette.
func TerminalFields() []Field {
	c := func(key, label string, get func(*Draft) *theme.Color) Field {
		return Field{Key: key, Label: label, Kind: KindColor, Derives: true, color: get}
	}

	fields := []Field{
		c("terminal.background", "Background", func(d *Draft) *theme.Color { return &d.Theme.Terminal.Background }),
		c("terminal.foreground", "Foreground", func(d *Draft) *theme.Color { return &d.Theme.Terminal.Foreground }),
	}

	for i := range ansiNames {
		fields = append(fields, c(
			fmt.Sprintf("terminal.regular.%d", i),
			fmt.Sprintf("%d %s", i, ansiNames[i]),
			func(d *Draft) *theme.Color { return &d.Theme.Terminal.Regular[i] },
		))
	}
	for i := range ansiNames {
		fields = append(fields, c(
			fmt.Sprintf("terminal.bright.%d", i),
			fmt.Sprintf("%d bright %s", i+8, ansiNames[i]),
			func(d *Draft) *theme.Color { return &d.Theme.Terminal.Bright[i] },
		))
	}

	return append(fields,
		c("terminal.selection_foreground", "Selection fg", func(d *Draft) *theme.Color { return &d.Theme.Terminal.SelectionForeground }),
		c("terminal.selection_background", "Selection bg", func(d *Draft) *theme.Color { return &d.Theme.Terminal.SelectionBackground }),
		c("terminal.cursor", "Cursor", func(d *Draft) *theme.Color { return &d.Theme.Terminal.Cursor }),
		c("terminal.url", "URL", func(d *Draft) *theme.Color { return &d.Theme.Terminal.URL }),
	)
}

func fontFields() []Field {
	family := func(key, label string, mono bool, get func(*Draft) *string) Field {
		return Field{Key: key, Label: label, Group: GroupFonts, Kind: KindFont, Mono: mono, Derives: true, text: get}
	}
	size := func(key, label string, get func(*Draft) *int) Field {
		return Field{Key: key, Label: label, Group: GroupFonts, Kind: KindInt, Min: 1, Max: 96, Step: 1, Derives: true, num: get}
	}
	return []Field{
		family("fonts.ui_family", "UI family", false, func(d *Draft) *string { return &d.Theme.Fonts.UIFamily }),
		size("fonts.ui_size", "UI size", func(d *Draft) *int { return &d.Theme.Fonts.UISize }),
		family("fonts.mono_family", "Mono family", true, func(d *Draft) *string { return &d.Theme.Fonts.MonoFamily }),
		size("fonts.mono_size", "Mono size", func(d *Draft) *int { return &d.Theme.Fonts.MonoSize }),
		family("fonts.bar_family", "Bar family", true, func(d *Draft) *string { return &d.Theme.Fonts.BarFamily }),
		size("fonts.bar_size", "Bar size", func(d *Draft) *int { return &d.Theme.Fonts.BarSize }),
	}
}

// swayFXFields is the compositor: how it looks and how it behaves. The
// geometry is appearance and lives in the theme; the behaviour is structure
// and lives in config.toml. They are one section because they are one
// question — what is the window manager like — and the labels say which is
// which by grouping.
func swayFXFields() []Field {
	px := func(key, label string, max float64, get func(*Draft) *int) Field {
		return Field{Key: key, Label: label, Group: GroupSwayFX, Kind: KindInt, Min: 0, Max: max, Step: 1, Derives: true, num: get}
	}
	frac := func(key, label string, get func(*Draft) *float64) Field {
		return Field{Key: key, Label: label, Group: GroupSwayFX, Kind: KindFloat, Min: 0, Max: 1, Step: 0.01, Derives: true, frac: get}
	}

	// The border colours live here rather than under Colors: they are what a
	// window border looks like, and the width is right next to them. They are
	// still theme values and still save to the theme file.
	border := func(key, label string, get func(*Draft) *theme.Color) Field {
		return Field{Key: key, Label: label, Group: GroupSwayFX, Kind: KindColor, Derives: true, color: get}
	}

	geometry := []Field{
		px("ui.border_width", "Border width", 32, func(d *Draft) *int { return &d.Theme.UI.BorderWidth }),
		border("colors.border", "Border colour", func(d *Draft) *theme.Color { return &d.Theme.Colors.Border }),
		border("colors.border_focus", "Border focus colour", func(d *Draft) *theme.Color { return &d.Theme.Colors.BorderFocus }),
		px("ui.radius", "Radius", 64, func(d *Draft) *int { return &d.Theme.UI.Radius }),
		px("ui.gaps_inner", "Gaps inner", 128, func(d *Draft) *int { return &d.Theme.UI.GapsInner }),
		px("ui.gaps_outer", "Gaps outer", 128, func(d *Draft) *int { return &d.Theme.UI.GapsOuter }),
		px("ui.padding", "Padding", 128, func(d *Draft) *int { return &d.Theme.UI.Padding }),
		px("ui.horizontal_padding", "Horizontal padding", 128, func(d *Draft) *int { return &d.Theme.UI.HorizontalPadding }),
		frac("ui.opacity", "Opacity", func(d *Draft) *float64 { return &d.Theme.UI.Opacity }),
		px("ui.blur_radius", "Blur radius", 64, func(d *Draft) *int { return &d.Theme.UI.BlurRadius }),
		px("ui.blur_passes", "Blur passes", 10, func(d *Draft) *int { return &d.Theme.UI.BlurPasses }),
		frac("ui.blur_noise", "Blur noise", func(d *Draft) *float64 { return &d.Theme.UI.BlurNoise }),
		px("ui.shadow_blur", "Shadow blur", 128, func(d *Draft) *int { return &d.Theme.UI.ShadowBlur }),
		frac("ui.dim_inactive", "Dim inactive", func(d *Draft) *float64 { return &d.Theme.UI.DimInactive }),
	}

	return append(geometry, compositorFields()...)
}

// compositorFields are the window manager's own settings, which live in
// config.toml. Labels are prefixed by what they configure, because "Layout"
// and "Tap" mean nothing on their own in a list this long.
func compositorFields() []Field {
	sway := func(f Field) Field {
		f.Group, f.Store = GroupSwayFX, StoreConfig
		return f
	}

	return []Field{
		sway(pText("sway.mod", "Modifier", "the key every binding hangs off",
			func(d *Draft) *string { return &d.Config.Sway.Mod })),
		sway(pText("sway.wallpaper", "Wallpaper", "path to the background image",
			func(d *Draft) *string { return &d.Config.Sway.Wallpaper })),
		sway(pChoice("sway.wallpaper_mode", "Wallpaper mode", "how the image fills the output",
			[]string{"stretch", "fill", "fit", "center", "tile", "solid_color"},
			func(d *Draft) *string { return &d.Config.Sway.WallpaperMode })),
		sway(pBool("sway.titlebar", "Titlebars", "draw a titlebar on tiled windows",
			func(d *Draft) *bool { return &d.Config.Sway.Titlebar })),
		sway(pBool("sway.smart_borders", "Smart borders", "hide borders when a workspace has one window",
			func(d *Draft) *bool { return &d.Config.Sway.SmartBorders })),
		sway(pBool("sway.smart_gaps", "Smart gaps", "hide gaps when a workspace has one window",
			func(d *Draft) *bool { return &d.Config.Sway.SmartGaps })),
		sway(pBool("sway.focus_follows_mouse", "Focus follows mouse", "",
			func(d *Draft) *bool { return &d.Config.Sway.FocusFollowsMouse })),

		sway(pText("sway.keyboard.layout", "Keyboard layout", "xkb layouts, comma-separated: us,rs",
			func(d *Draft) *string { return &d.Config.Sway.Keyboard.Layout })),
		sway(pText("sway.keyboard.variant", "Keyboard variant", "one per layout: ,latin",
			func(d *Draft) *string { return &d.Config.Sway.Keyboard.Variant })),
		sway(pText("sway.keyboard.options", "Keyboard options", "xkb options, such as grp:alt_shift_toggle",
			func(d *Draft) *string { return &d.Config.Sway.Keyboard.Options })),
		sway(pInt("sway.keyboard.repeat_delay", "Repeat delay", "milliseconds before a key repeats", 0, 2000, 25,
			func(d *Draft) *int { return &d.Config.Sway.Keyboard.RepeatDelay })),
		sway(pInt("sway.keyboard.repeat_rate", "Repeat rate", "repeats per second", 0, 100, 5,
			func(d *Draft) *int { return &d.Config.Sway.Keyboard.RepeatRate })),

		sway(pBool("sway.touchpad.tap", "Touchpad tap", "tap to click",
			func(d *Draft) *bool { return &d.Config.Sway.Touchpad.Tap })),
		sway(pChoice("sway.touchpad.tap_button_map", "Tap buttons", "which button a two- or three-finger tap sends",
			[]string{"lrm", "lmr"},
			func(d *Draft) *string { return &d.Config.Sway.Touchpad.TapButtonMap })),
		sway(pBool("sway.touchpad.natural_scroll", "Natural scroll", "",
			func(d *Draft) *bool { return &d.Config.Sway.Touchpad.NaturalScroll })),
		sway(pBool("sway.touchpad.disable_while_typing", "Disable while typing", "",
			func(d *Draft) *bool { return &d.Config.Sway.Touchpad.DisableWhileTyping })),
		sway(pBool("sway.touchpad.middle_emulation", "Middle emulation", "both buttons together send middle-click",
			func(d *Draft) *bool { return &d.Config.Sway.Touchpad.MiddleEmulation })),
		sway(pBool("sway.touchpad.drag_lock", "Drag lock", "",
			func(d *Draft) *bool { return &d.Config.Sway.Touchpad.DragLock })),
		sway(pChoice("sway.touchpad.accel_profile", "Accel profile", "pointer acceleration curve",
			[]string{"adaptive", "flat"},
			func(d *Draft) *string { return &d.Config.Sway.Touchpad.AccelProfile })),

		sway(pBool("sway.idle.enabled", "Idle enabled", "run swayidle at all",
			func(d *Draft) *bool { return &d.Config.Sway.Idle.Enabled })),
		sway(pInt("sway.idle.lock_after", "Idle lock after", "seconds; 0 never locks", 0, 7200, 60,
			func(d *Draft) *int { return &d.Config.Sway.Idle.LockAfter })),
		sway(pInt("sway.idle.screen_off", "Idle screen off", "seconds; 0 never blanks", 0, 7200, 60,
			func(d *Draft) *int { return &d.Config.Sway.Idle.ScreenOff })),
		sway(pInt("sway.idle.sleep_after", "Idle sleep after", "seconds; 0 never suspends", 0, 28800, 300,
			func(d *Draft) *int { return &d.Config.Sway.Idle.SleepAfter })),
		sway(pBool("sway.idle.lock_on_sleep", "Lock on sleep", "",
			func(d *Draft) *bool { return &d.Config.Sway.Idle.LockOnSleep })),
	}
}

func iconFields() []Field {
	// The theme fields that name something installed on the machine are
	// pickable, for the same reason fonts are: nobody remembers whether the
	// directory is called Papirus-Dark or papirus-dark.
	return []Field{
		{
			Key: "icons.theme", Label: "Icon theme", Group: GroupIcons, Kind: KindText, Derives: true,
			Assets: assets.IconThemes, PicksAssets: true,
			text: func(d *Draft) *string { return &d.Theme.Icons.Theme },
		},
		{
			Key: "icons.size", Label: "Icon size", Group: GroupIcons, Kind: KindInt,
			Min: 8, Max: 256, Step: 2, Derives: true,
			num: func(d *Draft) *int { return &d.Theme.Icons.Size },
		},
		{
			Key: "cursor.theme", Label: "Cursor theme", Group: GroupIcons, Kind: KindText, Derives: true,
			Assets: assets.CursorThemes, PicksAssets: true,
			text: func(d *Draft) *string { return &d.Theme.Cursor.Theme },
		},
		{
			Key: "gtk.theme", Label: "GTK theme", Group: GroupIcons, Kind: KindText, Derives: true,
			Assets: assets.GTKThemes, PicksAssets: true,
			text: func(d *Draft) *string { return &d.Theme.GTK.Theme },
		},
		{
			Key: "gtk.kvantum_theme", Label: "Kvantum theme", Group: GroupIcons, Kind: KindText, Derives: true,
			Assets: assets.KvantumThemes, PicksAssets: true,
			text: func(d *Draft) *string { return &d.Theme.GTK.KvantumTheme },
		},
		{
			Key: "gtk.qt_style_override", Label: "Qt style", Group: GroupIcons, Kind: KindText, Derives: true,
			text: func(d *Draft) *string { return &d.Theme.GTK.QtStyleOverride },
		},
		{
			Key: "cursor.size", Label: "Cursor size", Group: GroupIcons, Kind: KindInt,
			Min: 8, Max: 256, Step: 2, Derives: true,
			num: func(d *Draft) *int { return &d.Theme.Cursor.Size },
		},
	}
}
