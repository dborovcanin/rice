package session

import (
	"fmt"
	"sync"

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
	allFields = append(allFields, terminalFields()...)
	allFields = append(allFields, fontFields()...)
	allFields = append(allFields, sizingFields()...)
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
		c("colors.border", "Border", func(d *Draft) *theme.Color { return &d.Theme.Colors.Border }),
		c("colors.border_focus", "Border focus", func(d *Draft) *theme.Color { return &d.Theme.Colors.BorderFocus }),
	}
}

// ansiNames label the sixteen terminal slots the way terminal documentation
// does, because "regular 4" means nothing on its own.
var ansiNames = [8]string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white"}

func terminalFields() []Field {
	c := func(key, label string, get func(*Draft) *theme.Color) Field {
		return Field{Key: key, Label: label, Group: GroupTerminal, Kind: KindColor, Derives: true, color: get}
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

func sizingFields() []Field {
	px := func(key, label string, max float64, get func(*Draft) *int) Field {
		return Field{Key: key, Label: label, Group: GroupSizing, Kind: KindInt, Min: 0, Max: max, Step: 1, Derives: true, num: get}
	}
	frac := func(key, label string, get func(*Draft) *float64) Field {
		return Field{Key: key, Label: label, Group: GroupSizing, Kind: KindFloat, Min: 0, Max: 1, Step: 0.01, Derives: true, frac: get}
	}
	return []Field{
		px("ui.radius", "Radius", 64, func(d *Draft) *int { return &d.Theme.UI.Radius }),
		px("ui.border_width", "Border width", 32, func(d *Draft) *int { return &d.Theme.UI.BorderWidth }),
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
}

func iconFields() []Field {
	return []Field{
		{
			Key: "icons.theme", Label: "Icon theme", Group: GroupIcons, Kind: KindText, Derives: true,
			text: func(d *Draft) *string { return &d.Theme.Icons.Theme },
		},
		{
			Key: "icons.size", Label: "Icon size", Group: GroupIcons, Kind: KindInt,
			Min: 8, Max: 256, Step: 2, Derives: true,
			num: func(d *Draft) *int { return &d.Theme.Icons.Size },
		},
		{
			Key: "cursor.theme", Label: "Cursor theme", Group: GroupIcons, Kind: KindText, Derives: true,
			text: func(d *Draft) *string { return &d.Theme.Cursor.Theme },
		},
		{
			Key: "cursor.size", Label: "Cursor size", Group: GroupIcons, Kind: KindInt,
			Min: 8, Max: 256, Step: 2, Derives: true,
			num: func(d *Draft) *int { return &d.Theme.Cursor.Size },
		},
	}
}
