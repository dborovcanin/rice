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

// LookupField finds a field by key.
func LookupField(key string) (Field, bool) {
	fieldsOnce.Do(buildFields)
	f, ok := byKey[key]
	return f, ok
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
	c := func(key, label string, get func(*theme.Theme) *theme.Color) Field {
		return Field{Key: key, Label: label, Group: GroupColors, Kind: KindColor, color: get}
	}
	return []Field{
		c("colors.background", "Background", func(t *theme.Theme) *theme.Color { return &t.Colors.Background }),
		c("colors.surface", "Surface", func(t *theme.Theme) *theme.Color { return &t.Colors.Surface }),
		c("colors.surface_alt", "Surface alt", func(t *theme.Theme) *theme.Color { return &t.Colors.SurfaceAlt }),
		c("colors.overlay", "Overlay", func(t *theme.Theme) *theme.Color { return &t.Colors.Overlay }),
		c("colors.foreground", "Foreground", func(t *theme.Theme) *theme.Color { return &t.Colors.Foreground }),
		c("colors.muted", "Muted", func(t *theme.Theme) *theme.Color { return &t.Colors.Muted }),
		c("colors.primary", "Primary", func(t *theme.Theme) *theme.Color { return &t.Colors.Primary }),
		c("colors.secondary", "Secondary", func(t *theme.Theme) *theme.Color { return &t.Colors.Secondary }),
		c("colors.accent", "Accent", func(t *theme.Theme) *theme.Color { return &t.Colors.Accent }),
		c("colors.success", "Success", func(t *theme.Theme) *theme.Color { return &t.Colors.Success }),
		c("colors.warning", "Warning", func(t *theme.Theme) *theme.Color { return &t.Colors.Warning }),
		c("colors.error", "Error", func(t *theme.Theme) *theme.Color { return &t.Colors.Error }),
		c("colors.border", "Border", func(t *theme.Theme) *theme.Color { return &t.Colors.Border }),
		c("colors.border_focus", "Border focus", func(t *theme.Theme) *theme.Color { return &t.Colors.BorderFocus }),
	}
}

// ansiNames label the sixteen terminal slots the way terminal documentation
// does, because "regular 4" means nothing on its own.
var ansiNames = [8]string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white"}

func terminalFields() []Field {
	c := func(key, label string, get func(*theme.Theme) *theme.Color) Field {
		return Field{Key: key, Label: label, Group: GroupTerminal, Kind: KindColor, color: get}
	}

	fields := []Field{
		c("terminal.background", "Background", func(t *theme.Theme) *theme.Color { return &t.Terminal.Background }),
		c("terminal.foreground", "Foreground", func(t *theme.Theme) *theme.Color { return &t.Terminal.Foreground }),
	}

	for i := range ansiNames {
		fields = append(fields, c(
			fmt.Sprintf("terminal.regular.%d", i),
			fmt.Sprintf("%d %s", i, ansiNames[i]),
			func(t *theme.Theme) *theme.Color { return &t.Terminal.Regular[i] },
		))
	}
	for i := range ansiNames {
		fields = append(fields, c(
			fmt.Sprintf("terminal.bright.%d", i),
			fmt.Sprintf("%d bright %s", i+8, ansiNames[i]),
			func(t *theme.Theme) *theme.Color { return &t.Terminal.Bright[i] },
		))
	}

	return append(fields,
		c("terminal.selection_foreground", "Selection fg", func(t *theme.Theme) *theme.Color { return &t.Terminal.SelectionForeground }),
		c("terminal.selection_background", "Selection bg", func(t *theme.Theme) *theme.Color { return &t.Terminal.SelectionBackground }),
		c("terminal.cursor", "Cursor", func(t *theme.Theme) *theme.Color { return &t.Terminal.Cursor }),
		c("terminal.url", "URL", func(t *theme.Theme) *theme.Color { return &t.Terminal.URL }),
	)
}

func fontFields() []Field {
	family := func(key, label string, mono bool, get func(*theme.Theme) *string) Field {
		return Field{Key: key, Label: label, Group: GroupFonts, Kind: KindFont, Mono: mono, text: get}
	}
	size := func(key, label string, get func(*theme.Theme) *int) Field {
		return Field{Key: key, Label: label, Group: GroupFonts, Kind: KindInt, Min: 1, Max: 96, Step: 1, num: get}
	}
	return []Field{
		family("fonts.ui_family", "UI family", false, func(t *theme.Theme) *string { return &t.Fonts.UIFamily }),
		size("fonts.ui_size", "UI size", func(t *theme.Theme) *int { return &t.Fonts.UISize }),
		family("fonts.mono_family", "Mono family", true, func(t *theme.Theme) *string { return &t.Fonts.MonoFamily }),
		size("fonts.mono_size", "Mono size", func(t *theme.Theme) *int { return &t.Fonts.MonoSize }),
		family("fonts.bar_family", "Bar family", true, func(t *theme.Theme) *string { return &t.Fonts.BarFamily }),
		size("fonts.bar_size", "Bar size", func(t *theme.Theme) *int { return &t.Fonts.BarSize }),
	}
}

func sizingFields() []Field {
	px := func(key, label string, max float64, get func(*theme.Theme) *int) Field {
		return Field{Key: key, Label: label, Group: GroupSizing, Kind: KindInt, Min: 0, Max: max, Step: 1, num: get}
	}
	frac := func(key, label string, get func(*theme.Theme) *float64) Field {
		return Field{Key: key, Label: label, Group: GroupSizing, Kind: KindFloat, Min: 0, Max: 1, Step: 0.01, frac: get}
	}
	return []Field{
		px("ui.radius", "Radius", 64, func(t *theme.Theme) *int { return &t.UI.Radius }),
		px("ui.border_width", "Border width", 32, func(t *theme.Theme) *int { return &t.UI.BorderWidth }),
		px("ui.gaps_inner", "Gaps inner", 128, func(t *theme.Theme) *int { return &t.UI.GapsInner }),
		px("ui.gaps_outer", "Gaps outer", 128, func(t *theme.Theme) *int { return &t.UI.GapsOuter }),
		px("ui.padding", "Padding", 128, func(t *theme.Theme) *int { return &t.UI.Padding }),
		px("ui.horizontal_padding", "Horizontal padding", 128, func(t *theme.Theme) *int { return &t.UI.HorizontalPadding }),
		frac("ui.opacity", "Opacity", func(t *theme.Theme) *float64 { return &t.UI.Opacity }),
		px("ui.blur_radius", "Blur radius", 64, func(t *theme.Theme) *int { return &t.UI.BlurRadius }),
		px("ui.blur_passes", "Blur passes", 10, func(t *theme.Theme) *int { return &t.UI.BlurPasses }),
		frac("ui.blur_noise", "Blur noise", func(t *theme.Theme) *float64 { return &t.UI.BlurNoise }),
		px("ui.shadow_blur", "Shadow blur", 128, func(t *theme.Theme) *int { return &t.UI.ShadowBlur }),
		frac("ui.dim_inactive", "Dim inactive", func(t *theme.Theme) *float64 { return &t.UI.DimInactive }),
	}
}

func iconFields() []Field {
	return []Field{
		{
			Key: "icons.theme", Label: "Icon theme", Group: GroupIcons, Kind: KindText,
			text: func(t *theme.Theme) *string { return &t.Icons.Theme },
		},
		{
			Key: "icons.size", Label: "Icon size", Group: GroupIcons, Kind: KindInt,
			Min: 8, Max: 256, Step: 2,
			num: func(t *theme.Theme) *int { return &t.Icons.Size },
		},
		{
			Key: "cursor.theme", Label: "Cursor theme", Group: GroupIcons, Kind: KindText,
			text: func(t *theme.Theme) *string { return &t.Cursor.Theme },
		},
		{
			Key: "cursor.size", Label: "Cursor size", Group: GroupIcons, Kind: KindInt,
			Min: 8, Max: 256, Step: 2,
			num: func(t *theme.Theme) *int { return &t.Cursor.Size },
		},
	}
}
