package theme

import (
	"errors"
	"fmt"
	"strings"
)

// ValidationError collects every problem found in a theme so the user can fix
// them in one pass instead of one error per run.
type ValidationError struct {
	Theme    string
	Problems []string
}

func (e *ValidationError) Error() string {
	name := e.Theme
	if name == "" {
		name = "theme"
	}
	return fmt.Sprintf("%s: %s", name, strings.Join(e.Problems, "; "))
}

// Validate reports whether the theme is renderable. Call Normalize first:
// validation describes the effective theme, not the source file.
func (t Theme) Validate() error {
	var problems []string

	if strings.TrimSpace(t.Name) == "" {
		problems = append(problems, "name is empty")
	}
	switch t.Variant {
	case "", "dark", "light":
	default:
		problems = append(problems, fmt.Sprintf("variant %q is not \"dark\" or \"light\"", t.Variant))
	}

	required := []struct {
		name  string
		color Color
	}{
		{"colors.background", t.Colors.Background},
		{"colors.foreground", t.Colors.Foreground},
		{"colors.primary", t.Colors.Primary},
	}
	for _, r := range required {
		if r.color.IsZero() {
			problems = append(problems, r.name+" is not set")
		}
	}

	if t.Colors.Background.Opaque() == t.Colors.Foreground.Opaque() {
		problems = append(problems, "colors.background and colors.foreground are identical")
	}

	if t.UI.Opacity < 0 || t.UI.Opacity > 1 {
		problems = append(problems, fmt.Sprintf("ui.opacity %.2f is outside 0..1", t.UI.Opacity))
	}
	if t.UI.DimInactive < 0 || t.UI.DimInactive > 1 {
		problems = append(problems, fmt.Sprintf("ui.dim_inactive %.2f is outside 0..1", t.UI.DimInactive))
	}
	if t.UI.BlurNoise < 0 || t.UI.BlurNoise > 1 {
		problems = append(problems, fmt.Sprintf("ui.blur_noise %.2f is outside 0..1", t.UI.BlurNoise))
	}

	nonNegative := []struct {
		name  string
		value int
	}{
		{"ui.radius", t.UI.Radius},
		{"ui.border_width", t.UI.BorderWidth},
		{"ui.gaps_inner", t.UI.GapsInner},
		{"ui.gaps_outer", t.UI.GapsOuter},
		{"ui.padding", t.UI.Padding},
		{"ui.horizontal_padding", t.UI.HorizontalPadding},
		{"ui.blur_radius", t.UI.BlurRadius},
		{"ui.blur_passes", t.UI.BlurPasses},
		{"ui.shadow_blur", t.UI.ShadowBlur},
		{"icons.size", t.Icons.Size},
	}
	for _, n := range nonNegative {
		if n.value < 0 {
			problems = append(problems, fmt.Sprintf("%s %d is negative", n.name, n.value))
		}
	}

	if t.Fonts.UISize <= 0 {
		problems = append(problems, "fonts.ui_size must be positive")
	}
	if t.Fonts.MonoSize <= 0 {
		problems = append(problems, "fonts.mono_size must be positive")
	}
	if t.Fonts.BarSize < 0 {
		problems = append(problems, "fonts.bar_size is negative")
	}
	if t.Cursor.Size <= 0 {
		problems = append(problems, "cursor.size must be positive")
	}

	if len(problems) == 0 {
		return nil
	}
	return &ValidationError{Theme: t.Name, Problems: problems}
}

// Problems returns the validation messages for err, or nil if err is not a
// validation error.
func Problems(err error) []string {
	var v *ValidationError
	if errors.As(err, &v) {
		return v.Problems
	}
	return nil
}
