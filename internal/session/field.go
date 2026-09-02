package session

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/dborovcanin/rice/internal/theme"
)

// Group is one section of the theme editor. Groups exist so an interface can
// present sixty fields without asking the user to scroll through all of them
// at once.
type Group int

const (
	GroupColors Group = iota
	GroupTerminal
	GroupFonts
	GroupSizing
	GroupIcons
)

// Groups returns every group in presentation order.
func Groups() []Group {
	return []Group{GroupColors, GroupTerminal, GroupFonts, GroupSizing, GroupIcons}
}

func (g Group) String() string {
	switch g {
	case GroupColors:
		return "Colors"
	case GroupTerminal:
		return "Terminal"
	case GroupFonts:
		return "Fonts"
	case GroupSizing:
		return "Sizing"
	case GroupIcons:
		return "Icons & Cursor"
	}
	return "Unknown"
}

// Kind is what sort of value a field holds, which decides how it is parsed,
// displayed and edited.
type Kind int

const (
	// KindColor is a hex color.
	KindColor Kind = iota
	// KindInt is a whole number, such as a pixel size.
	KindInt
	// KindFloat is a fraction, such as an opacity.
	KindFloat
	// KindText is a free string, such as an icon theme name.
	KindText
	// KindFont is a font family, which an interface may offer to pick from
	// the installed families rather than type.
	KindFont
)

// Field is one editable value in a theme. Access is through pointer accessors
// rather than reflection, so every editable value is spelled out once and the
// compiler checks it.
type Field struct {
	// Key is the stable identifier, matching the theme file: "colors.primary".
	Key string
	// Label is the human-readable name shown in an editor.
	Label string
	// Group is the section this field belongs to.
	Group Group
	// Kind decides parsing and presentation.
	Kind Kind
	// Mono marks a font field that should offer monospaced families first.
	Mono bool
	// Min and Max bound numeric fields. Max of zero means unbounded.
	Min, Max float64
	// Step is how far one nudge moves the value.
	Step float64

	color func(*theme.Theme) *theme.Color
	num   func(*theme.Theme) *int
	frac  func(*theme.Theme) *float64
	text  func(*theme.Theme) *string
}

// Display renders the field's current value the way it appears in a theme file.
func (f Field) Display(t theme.Theme) string {
	switch f.Kind {
	case KindColor:
		return f.color(&t).String()
	case KindInt:
		return strconv.Itoa(*f.num(&t))
	case KindFloat:
		return strconv.FormatFloat(*f.frac(&t), 'f', -1, 64)
	default:
		return *f.text(&t)
	}
}

// Color returns the field's color, and false when the field is not a color.
func (f Field) Color(t theme.Theme) (theme.Color, bool) {
	if f.Kind != KindColor {
		return theme.Color{}, false
	}
	return *f.color(&t), true
}

// Set parses raw and writes it into the theme. The theme is left untouched
// when the value does not parse or falls outside the field's bounds.
func (f Field) Set(t *theme.Theme, raw string) error {
	raw = strings.TrimSpace(raw)

	switch f.Kind {
	case KindColor:
		c, err := theme.ParseColor(raw)
		if err != nil {
			return err
		}
		*f.color(t) = c

	case KindInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("%s: %q is not a whole number", f.Key, raw)
		}
		if err := f.checkBounds(float64(n)); err != nil {
			return err
		}
		*f.num(t) = n

	case KindFloat:
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("%s: %q is not a number", f.Key, raw)
		}
		if err := f.checkBounds(v); err != nil {
			return err
		}
		*f.frac(t) = v

	default:
		if raw == "" {
			return fmt.Errorf("%s: value is empty", f.Key)
		}
		*f.text(t) = raw
	}
	return nil
}

// Nudge moves a numeric field by steps, or a color's lightness for colors.
// It is what arrow-key editing calls, so a palette can be adjusted without
// computing hex by hand.
//
// The current value is read from resolved and written into src, so nudging a
// value that was being derived materializes it at the value the user can
// actually see rather than at zero.
func (f Field) Nudge(src *theme.Theme, resolved theme.Theme, steps int) error {
	if steps == 0 {
		return nil
	}

	switch f.Kind {
	case KindColor:
		c := *f.color(&resolved)
		amount := 0.05 * math.Abs(float64(steps))
		if steps > 0 {
			c = c.Lighten(amount)
		} else {
			c = c.Darken(amount)
		}
		*f.color(src) = c
		return nil

	case KindInt:
		step := f.Step
		if step == 0 {
			step = 1
		}
		next := f.clamp(float64(*f.num(&resolved)) + step*float64(steps))
		*f.num(src) = int(math.Round(next))
		return nil

	case KindFloat:
		step := f.Step
		if step == 0 {
			step = 0.01
		}
		next := f.clamp(*f.frac(&resolved) + step*float64(steps))
		// Re-round to the step so repeated nudges do not accumulate noise.
		*f.frac(src) = math.Round(next/step) * step
		return nil
	}
	return fmt.Errorf("%s: cannot be nudged", f.Key)
}

// Explicit reports whether a source theme spells this field out. A field that
// is not explicit is derived by normalization, and follows whatever it is
// derived from.
//
// Zero means "unset" throughout the theme format, so that is what this checks.
func (f Field) Explicit(src theme.Theme) bool {
	switch f.Kind {
	case KindColor:
		return !f.color(&src).IsZero()
	case KindInt:
		return *f.num(&src) != 0
	case KindFloat:
		return *f.frac(&src) != 0
	default:
		return *f.text(&src) != ""
	}
}

// Clear removes an explicit value, handing the field back to normalization.
func (f Field) Clear(src *theme.Theme) {
	switch f.Kind {
	case KindColor:
		*f.color(src) = theme.Color{}
	case KindInt:
		*f.num(src) = 0
	case KindFloat:
		*f.frac(src) = 0
	default:
		*f.text(src) = ""
	}
}

// Same reports whether the field holds the same value in two themes, which is
// how the editor decides a value has been overridden.
func (f Field) Same(a, b theme.Theme) bool {
	switch f.Kind {
	case KindColor:
		return *f.color(&a) == *f.color(&b)
	case KindInt:
		return *f.num(&a) == *f.num(&b)
	case KindFloat:
		return *f.frac(&a) == *f.frac(&b)
	default:
		return *f.text(&a) == *f.text(&b)
	}
}

// CopyFrom writes the field's value from src into dst.
func (f Field) CopyFrom(dst *theme.Theme, src theme.Theme) {
	switch f.Kind {
	case KindColor:
		*f.color(dst) = *f.color(&src)
	case KindInt:
		*f.num(dst) = *f.num(&src)
	case KindFloat:
		*f.frac(dst) = *f.frac(&src)
	default:
		*f.text(dst) = *f.text(&src)
	}
}

func (f Field) checkBounds(v float64) error {
	if v < f.Min {
		return fmt.Errorf("%s: %s is below the minimum of %s", f.Key, trim(v), trim(f.Min))
	}
	if f.Max != 0 && v > f.Max {
		return fmt.Errorf("%s: %s is above the maximum of %s", f.Key, trim(v), trim(f.Max))
	}
	return nil
}

func (f Field) clamp(v float64) float64 {
	if v < f.Min {
		return f.Min
	}
	if f.Max != 0 && v > f.Max {
		return f.Max
	}
	return v
}

func trim(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
