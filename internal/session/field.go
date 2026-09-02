package session

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/dborovcanin/rice/internal/assets"
	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/theme"
)

// Group is one section of the theme editor. Groups exist so an interface can
// present sixty fields without asking the user to scroll through all of them
// at once.
type Group int

const (
	GroupColors Group = iota
	GroupFonts
	GroupSwayFX
	GroupIcons
)

// Groups returns every group in presentation order. These are the settings
// that belong to the desktop as a whole; anything that belongs to one
// application is in that application's own table instead.
func Groups() []Group {
	return []Group{GroupColors, GroupFonts, GroupSwayFX, GroupIcons}
}

func (g Group) String() string {
	switch g {
	case GroupColors:
		return "Colors"
	case GroupFonts:
		return "Fonts"
	case GroupSwayFX:
		return "SwayFX"
	case GroupIcons:
		return "Icons & Cursor"
	}
	return "Unknown"
}

// Draft is everything an editing session can change: appearance in the theme,
// structure in the configuration. Fields address it rather than a bare theme,
// so a per-program setting and a palette color are the same kind of thing to
// an interface.
type Draft struct {
	Theme  theme.Theme
	Config config.Config
}

// Store is which file a field's value ends up in.
type Store int

const (
	// StoreTheme is saved to the theme file: appearance.
	StoreTheme Store = iota
	// StoreConfig is saved to config.toml: structure.
	StoreConfig
)

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
	// KindBool is a switch.
	KindBool
	// KindChoice is a string from a fixed set.
	KindChoice
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
	// Assets names the kind of installed theme this field selects, when it
	// selects one. An interface offers those instead of asking the user to
	// remember an exact directory name.
	Assets assets.Kind
	// PicksAssets reports whether Assets means anything, since the zero Kind
	// is a real value.
	PicksAssets bool
	// Derives marks a field that normalization fills in when it is left
	// unset, which is true of much of the theme and none of the
	// configuration.
	Derives bool
	// Store says which file the field is saved to. A group may hold fields
	// from both — SwayFX mixes the theme's geometry with the compositor's
	// behaviour — so dirty tracking follows this rather than which table a
	// field was declared in.
	Store Store
	// Choices, when set, are the only accepted values.
	Choices []string
	// Help is a one-line explanation shown beside the field.
	Help string
	// Min and Max bound numeric fields. Max of zero means unbounded.
	Min, Max float64
	// Step is how far one nudge moves the value.
	Step float64

	// fallback says what an unset override resolves to. It is display only:
	// editing still works on the stored value, so opening an unset override
	// does not silently make it explicit.
	fallback func(Draft) string

	color func(*Draft) *theme.Color
	num   func(*Draft) *int
	frac  func(*Draft) *float64
	text  func(*Draft) *string
	flag  func(*Draft) *bool
}

// Display renders the field's current value the way it appears in a theme file.
func (f Field) Display(d Draft) string {
	switch f.Kind {
	case KindColor:
		return f.color(&d).String()
	case KindInt:
		return strconv.Itoa(*f.num(&d))
	case KindFloat:
		return strconv.FormatFloat(*f.frac(&d), 'f', -1, 64)
	case KindBool:
		return strconv.FormatBool(*f.flag(&d))
	default:
		return *f.text(&d)
	}
}

// Effective is what the value actually resolves to: the stored value, or what
// the theme gives when an override is unset.
func (f Field) Effective(d Draft) string {
	if f.fallback != nil && !f.Explicit(d) {
		return f.fallback(d)
	}
	return f.Display(d)
}

// Inherited reports whether the field is showing a value it does not hold —
// an unset override following the theme.
func (f Field) Inherited(d Draft) bool { return f.fallback != nil && !f.Explicit(d) }

// Color returns the field's color, and false when the field is not a color.
func (f Field) Color(d Draft) (theme.Color, bool) {
	if f.Kind != KindColor {
		return theme.Color{}, false
	}
	return *f.color(&d), true
}

// Set parses raw and writes it into the theme. The theme is left untouched
// when the value does not parse or falls outside the field's bounds.
func (f Field) Set(d *Draft, raw string) error {
	raw = strings.TrimSpace(raw)

	switch f.Kind {
	case KindColor:
		c, err := theme.ParseColor(raw)
		if err != nil {
			return err
		}
		*f.color(d) = c

	case KindInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("%s: %q is not a whole number", f.Key, raw)
		}
		if err := f.checkBounds(float64(n)); err != nil {
			return err
		}
		*f.num(d) = n

	case KindFloat:
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("%s: %q is not a number", f.Key, raw)
		}
		if err := f.checkBounds(v); err != nil {
			return err
		}
		*f.frac(d) = v

	case KindBool:
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("%s: %q is not true or false", f.Key, raw)
		}
		*f.flag(d) = v

	default:
		if raw == "" {
			return fmt.Errorf("%s: value is empty", f.Key)
		}
		if err := f.checkChoice(raw); err != nil {
			return err
		}
		*f.text(d) = raw
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
func (f Field) Nudge(src *Draft, resolved Draft, steps int) error {
	if steps == 0 {
		return nil
	}

	switch f.Kind {
	case KindBool:
		// Nudging a switch is the only sensible reading of "change it".
		*f.flag(src) = !*f.flag(&resolved)
		return nil

	case KindChoice:
		return f.cycle(src, resolved, steps)

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
func (f Field) Explicit(src Draft) bool {
	if !f.Derives {
		return true
	}
	switch f.Kind {
	case KindColor:
		return !f.color(&src).IsZero()
	case KindInt:
		return *f.num(&src) != 0
	case KindFloat:
		return *f.frac(&src) != 0
	case KindBool:
		return true
	default:
		return *f.text(&src) != ""
	}
}

// Clear removes an explicit value, handing the field back to normalization.
func (f Field) Clear(src *Draft) {
	if !f.Derives {
		return
	}
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
func (f Field) Same(a, b Draft) bool {
	switch f.Kind {
	case KindColor:
		return *f.color(&a) == *f.color(&b)
	case KindInt:
		return *f.num(&a) == *f.num(&b)
	case KindFloat:
		return *f.frac(&a) == *f.frac(&b)
	case KindBool:
		return *f.flag(&a) == *f.flag(&b)
	default:
		return *f.text(&a) == *f.text(&b)
	}
}

// CopyFrom writes the field's value from src into dst.
func (f Field) CopyFrom(dst *Draft, src Draft) {
	switch f.Kind {
	case KindColor:
		*f.color(dst) = *f.color(&src)
	case KindInt:
		*f.num(dst) = *f.num(&src)
	case KindFloat:
		*f.frac(dst) = *f.frac(&src)
	case KindBool:
		*f.flag(dst) = *f.flag(&src)
	default:
		*f.text(dst) = *f.text(&src)
	}
}

// cycle moves a choice field to the next or previous accepted value.
func (f Field) cycle(src *Draft, resolved Draft, steps int) error {
	if len(f.Choices) == 0 {
		return fmt.Errorf("%s: no choices", f.Key)
	}

	current := *f.text(&resolved)
	at := 0
	for i, c := range f.Choices {
		if c == current {
			at = i
		}
	}

	next := (at + steps) % len(f.Choices)
	if next < 0 {
		next += len(f.Choices)
	}
	*f.text(src) = f.Choices[next]
	return nil
}

// checkChoice rejects a value a field does not accept.
func (f Field) checkChoice(raw string) error {
	if len(f.Choices) == 0 {
		return nil
	}
	for _, c := range f.Choices {
		if c == raw {
			return nil
		}
	}
	return fmt.Errorf("%s: %q is not one of %s", f.Key, raw, strings.Join(f.Choices, ", "))
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
