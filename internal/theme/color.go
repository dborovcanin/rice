package theme

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Color is an RGB color with an optional alpha channel. It is stored as
// separate 8-bit components so templates can derive variants (lighter,
// darker, translucent) without re-parsing text.
type Color struct {
	R, G, B, A uint8
}

// ParseColor accepts "#rgb", "#rrggbb" and "#rrggbbaa", with or without the
// leading '#'. Missing alpha means fully opaque.
func ParseColor(s string) (Color, error) {
	raw := strings.TrimSpace(s)
	raw = strings.TrimPrefix(raw, "#")

	switch len(raw) {
	case 3, 4, 6, 8:
	default:
		return Color{}, fmt.Errorf("color %q: want 3, 4, 6 or 8 hex digits, got %d", s, len(raw))
	}

	for _, r := range raw {
		if !isHexDigit(r) {
			return Color{}, fmt.Errorf("color %q: invalid hex digit %q", s, r)
		}
	}

	if len(raw) <= 4 {
		var expanded strings.Builder
		for _, r := range raw {
			expanded.WriteRune(r)
			expanded.WriteRune(r)
		}
		raw = expanded.String()
	}

	v, err := strconv.ParseUint(raw, 16, 64)
	if err != nil {
		return Color{}, fmt.Errorf("color %q: %w", s, err)
	}

	c := Color{A: 0xff}
	if len(raw) == 8 {
		c.R = uint8(v >> 24)
		c.G = uint8(v >> 16)
		c.B = uint8(v >> 8)
		c.A = uint8(v)
		return c, nil
	}

	c.R = uint8(v >> 16)
	c.G = uint8(v >> 8)
	c.B = uint8(v)
	return c, nil
}

// MustParseColor is ParseColor for compile-time-known literals.
func MustParseColor(s string) Color {
	c, err := ParseColor(s)
	if err != nil {
		panic(err)
	}
	return c
}

// UnmarshalText lets colors be decoded straight out of TOML strings.
func (c *Color) UnmarshalText(b []byte) error {
	parsed, err := ParseColor(string(b))
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// MarshalText writes the canonical "#rrggbb" (or "#rrggbbaa") form.
func (c Color) MarshalText() ([]byte, error) {
	return []byte(c.String()), nil
}

// String is the default template rendering: "#rrggbb", or "#rrggbbaa" when
// the color is translucent.
func (c Color) String() string {
	if c.A == 0xff {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

// Hex is "#rrggbb", dropping any alpha.
func (c Color) Hex() string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// Bare is "rrggbb" with no leading '#', the form foot.ini expects.
func (c Color) Bare() string { return fmt.Sprintf("%02x%02x%02x", c.R, c.G, c.B) }

// BareA is "rrggbbaa" with no leading '#', the form swaylock expects.
func (c Color) BareA() string { return fmt.Sprintf("%02x%02x%02x%02x", c.R, c.G, c.B, c.A) }

// HexA is "#rrggbbaa", always including the alpha channel.
func (c Color) HexA() string { return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A) }

// ARGB is "aarrggbb", the form swaylock and some GTK settings expect.
func (c Color) ARGB() string { return fmt.Sprintf("%02x%02x%02x%02x", c.A, c.R, c.G, c.B) }

// RGB is the CSS "rgb(r, g, b)" form.
func (c Color) RGB() string { return fmt.Sprintf("rgb(%d, %d, %d)", c.R, c.G, c.B) }

// RGBA is the CSS "rgba(r, g, b, a)" form with alpha as a 0..1 fraction.
func (c Color) RGBA() string {
	return fmt.Sprintf("rgba(%d, %d, %d, %s)", c.R, c.G, c.B, trimFloat(float64(c.A)/255))
}

// Alpha returns the color with alpha set to the given 0..1 fraction.
func (c Color) Alpha(f float64) Color {
	c.A = uint8(math.Round(clamp01(f) * 255))
	return c
}

// Opaque returns the color with the alpha channel dropped.
func (c Color) Opaque() Color {
	c.A = 0xff
	return c
}

// Lighten moves each channel the given 0..1 fraction toward white.
func (c Color) Lighten(f float64) Color { return c.Mix(Color{R: 0xff, G: 0xff, B: 0xff, A: c.A}, f) }

// Darken moves each channel the given 0..1 fraction toward black.
func (c Color) Darken(f float64) Color { return c.Mix(Color{A: c.A}, f) }

// Mix blends toward other by the given 0..1 fraction; 0 returns c unchanged.
func (c Color) Mix(other Color, f float64) Color {
	f = clamp01(f)
	return Color{
		R: mixChannel(c.R, other.R, f),
		G: mixChannel(c.G, other.G, f),
		B: mixChannel(c.B, other.B, f),
		A: mixChannel(c.A, other.A, f),
	}
}

// Luminance is the perceived brightness on a 0..1 scale (ITU-R BT.601).
func (c Color) Luminance() float64 {
	return (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255
}

// IsDark reports whether the color is dark enough to want light text on top.
func (c Color) IsDark() bool { return c.Luminance() < 0.5 }

// Contrast returns black or white, whichever is readable against c.
func (c Color) Contrast() Color {
	if c.IsDark() {
		return Color{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}
	return Color{A: 0xff}
}

// IsZero reports whether the color was never set.
func (c Color) IsZero() bool { return c == Color{} }

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func mixChannel(a, b uint8, f float64) uint8 {
	return uint8(math.Round(float64(a)*(1-f) + float64(b)*f))
}

func clamp01(f float64) float64 {
	return math.Min(1, math.Max(0, f))
}

func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
