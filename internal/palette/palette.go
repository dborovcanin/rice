// Package palette derives a theme from an image.
//
// It is deliberately self-contained: no matugen, no external quantizer, no
// cgo. Rice owns the normalized theme model, and a wallpaper palette is just
// another way to fill it in.
//
// The result is a theme in *source* form, with everything derivable left
// unset. An image supplies a background, a foreground and a few accents; the
// sixteen ANSI slots, the surfaces and the borders are better computed from
// those than guessed at from pixel counts.
package palette

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"sort"

	"github.com/dborovcanin/rice/internal/theme"
)

// Options control how a palette is read out of an image.
type Options struct {
	// Name is the theme's name.
	Name string
	// Variant is "dark" or "light". Empty picks whichever the image is more
	// naturally suited to.
	Variant string
	// Clusters is how many colors to quantize to. Fewer than about six leaves
	// nothing to choose accents from; many more mostly splits hairs.
	Clusters int
	// MinContrast is the lowest acceptable contrast ratio between foreground
	// and background. Text has to be readable even when the image is not.
	MinContrast float64
	// Base supplies everything an image cannot: fonts, the icon and cursor
	// themes, geometry and the toolkit hints. Without it a derived theme has
	// a palette and nothing else, and Rice writes empty font and icon-theme
	// settings out to the toolkits.
	Base theme.Theme
}

// Defaults fills in the options a caller did not set.
func (o Options) Defaults() Options {
	if o.Clusters == 0 {
		o.Clusters = 12
	}
	if o.MinContrast == 0 {
		// The WCAG AA threshold for body text. A wallpaper is not an excuse
		// for an unreadable terminal.
		o.MinContrast = 7
	}
	return o
}

// maxSamples caps how many pixels are examined. A wallpaper is millions of
// pixels and the palette is a dozen colors; sampling changes the answer by
// less than the eye can see and turns seconds into milliseconds.
const maxSamples = 20000

// FromReader decodes an image and derives a theme from it.
func FromReader(r io.Reader, opts Options) (theme.Theme, error) {
	img, format, err := image.Decode(r)
	if err != nil {
		return theme.Theme{}, fmt.Errorf("decode image: %w (only PNG and JPEG are supported)", err)
	}
	th, err := FromImage(img, opts)
	if err != nil {
		return theme.Theme{}, err
	}
	if th.Description == "" {
		th.Description = "Derived from a " + format + " image."
	}
	return th, nil
}

// FromImage derives a theme from a decoded image.
//
// The result is deterministic: the same image and options always produce the
// same theme. Sampling walks a fixed stride and the clustering is seeded from
// the sampled colors themselves rather than from a random source, because a
// theme that changes every time you generate it is not a theme.
func FromImage(img image.Image, opts Options) (theme.Theme, error) {
	opts = opts.Defaults()

	samples := sample(img)
	if len(samples) == 0 {
		return theme.Theme{}, fmt.Errorf("image has no pixels to read")
	}

	clusters := quantize(samples, opts.Clusters)
	if len(clusters) == 0 {
		return theme.Theme{}, fmt.Errorf("image yielded no colors")
	}

	dark := opts.Variant != "light"
	if opts.Variant == "" {
		dark = meanLightness(clusters) < 0.5
	}

	// Start from the base, so the result is the base theme wearing the image's
	// colours rather than a palette with nothing around it.
	th := opts.Base
	th.Name = opts.Name
	th.Description = ""
	th.Colors = theme.Colors{}
	th.Terminal = theme.Terminal{}

	if dark {
		th.Variant = "dark"
	} else {
		th.Variant = "light"
	}
	if th.Variant != opts.Base.Variant {
		// A dark widget theme on a light palette is worse than none: let
		// normalization pick one that matches.
		th.GTK.Theme = ""
		th.GTK.KvantumTheme = ""
	}

	background := pickBackground(clusters, dark)
	foreground := pickForeground(clusters, background, dark, opts.MinContrast)

	th.Colors.Background = background.color()
	th.Colors.Foreground = foreground.color()

	accents := pickAccents(clusters, background, dark)
	th.Colors.Primary = accents[0].color()
	th.Colors.Secondary = accents[1].color()
	th.Colors.Accent = accents[2].color()

	// The semantic status colors have to read as themselves: a green that is
	// not green is worse than a green that does not match the wallpaper. Their
	// hue is fixed and only the saturation and lightness follow the image.
	th.Colors.Success = statusColor(120, accents[0], dark)
	th.Colors.Warning = statusColor(45, accents[0], dark)
	th.Colors.Error = statusColor(0, accents[0], dark)

	// Everything else — surfaces, muted, borders, the sixteen ANSI slots — is
	// left unset on purpose, so normalization derives it. Those relationships
	// are better computed than counted out of pixels.
	return th, nil
}

// hsl is a color in a space where "more colorful" and "lighter" are single
// numbers, which is what choosing a palette needs.
type hsl struct {
	h, s, l float64
	// weight is how much of the image this color accounts for, 0..1.
	weight float64
}

func (c hsl) color() theme.Color {
	r, g, b := hslToRGB(c.h, c.s, c.l)
	return theme.Color{R: r, G: g, B: b, A: 0xff}
}

// sample walks the image on a fixed stride, returning at most maxSamples
// colors. Fully transparent pixels are skipped: they are not part of what
// anyone sees.
func sample(img image.Image) []hsl {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width == 0 || height == 0 {
		return nil
	}

	stride := 1
	if total := width * height; total > maxSamples {
		stride = max(int(math.Sqrt(float64(total)/float64(maxSamples))), 1)
	}

	var out []hsl
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stride {
		for x := bounds.Min.X; x < bounds.Max.X; x += stride {
			r, g, b, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			h, s, l := rgbToHSL(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			out = append(out, hsl{h: h, s: s, l: l})
		}
	}
	return out
}

// quantize groups samples into at most k clusters by k-means, returning them
// sorted by weight, heaviest first.
//
// Hue is circular, so distances are measured on the shorter way round. Seeds
// are taken at even intervals through the samples sorted by lightness, which
// spreads them across the range the image actually covers and makes the result
// reproducible.
func quantize(samples []hsl, k int) []hsl {
	if len(samples) == 0 {
		return nil
	}
	if k > len(samples) {
		k = len(samples)
	}

	ordered := make([]hsl, len(samples))
	copy(ordered, samples)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].l != ordered[j].l {
			return ordered[i].l < ordered[j].l
		}
		if ordered[i].h != ordered[j].h {
			return ordered[i].h < ordered[j].h
		}
		return ordered[i].s < ordered[j].s
	})

	centres := make([]hsl, k)
	for i := range centres {
		centres[i] = ordered[i*len(ordered)/k]
	}

	assignment := make([]int, len(ordered))
	for range 16 {
		moved := false
		for i, s := range ordered {
			best, bestDist := 0, math.Inf(1)
			for c, centre := range centres {
				if d := distance(s, centre); d < bestDist {
					best, bestDist = c, d
				}
			}
			if assignment[i] != best {
				assignment[i] = best
				moved = true
			}
		}

		next := make([]hsl, k)
		counts := make([]int, k)
		// Hue is an angle, so it averages as a vector rather than a number:
		// the mean of 350° and 10° is 0°, not 180°.
		sinSum := make([]float64, k)
		cosSum := make([]float64, k)
		for i, s := range ordered {
			c := assignment[i]
			counts[c]++
			next[c].s += s.s
			next[c].l += s.l
			sinSum[c] += math.Sin(s.h * math.Pi / 180)
			cosSum[c] += math.Cos(s.h * math.Pi / 180)
		}
		for c := range centres {
			if counts[c] == 0 {
				next[c] = centres[c]
				continue
			}
			n := float64(counts[c])
			h := math.Atan2(sinSum[c]/n, cosSum[c]/n) * 180 / math.Pi
			if h < 0 {
				h += 360
			}
			next[c] = hsl{h: h, s: next[c].s / n, l: next[c].l / n, weight: n / float64(len(ordered))}
		}
		centres = next

		if !moved {
			break
		}
	}

	out := make([]hsl, 0, k)
	for _, c := range centres {
		if c.weight > 0 {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].weight > out[j].weight })
	return out
}

// distance is how unlike two colors are. Hue counts for less when either color
// is washed out, because the hue of a near-grey is noise.
func distance(a, b hsl) float64 {
	dh := math.Abs(a.h - b.h)
	if dh > 180 {
		dh = 360 - dh
	}
	saturation := math.Min(a.s, b.s)

	dl := a.l - b.l
	ds := a.s - b.s
	return (dh/180)*(dh/180)*saturation + dl*dl*2 + ds*ds
}

func meanLightness(clusters []hsl) float64 {
	var sum, weight float64
	for _, c := range clusters {
		sum += c.l * c.weight
		weight += c.weight
	}
	if weight == 0 {
		return 0.5
	}
	return sum / weight
}

// pickBackground takes the heaviest cluster at the right end of the lightness
// range and pushes it further, so the desktop has somewhere quiet to sit.
func pickBackground(clusters []hsl, dark bool) hsl {
	best := clusters[0]
	bestScore := math.Inf(-1)

	for _, c := range clusters {
		// Weight matters: the background should be a colour the image is
		// actually made of, not its rarest corner.
		score := c.weight
		if dark {
			score += (1 - c.l) * 0.6
		} else {
			score += c.l * 0.6
		}
		if score > bestScore {
			best, bestScore = c, score
		}
	}

	// A background keeps a hint of the image's hue and none of its noise.
	best.s = math.Min(best.s, 0.25)
	if dark {
		best.l = clampFloat(best.l*0.35, 0.06, 0.16)
	} else {
		best.l = clampFloat(1-(1-best.l)*0.35, 0.90, 0.97)
	}
	return best
}

// pickForeground finds readable text for the chosen background, and forces the
// contrast up if the image did not offer any.
func pickForeground(clusters []hsl, bg hsl, dark bool, minContrast float64) hsl {
	best := hsl{h: bg.h, s: math.Min(bg.s, 0.15)}
	if dark {
		best.l = 0.9
	} else {
		best.l = 0.15
	}

	// Prefer a colour from the image if one is already readable.
	bgColor := bg.color()
	for _, c := range clusters {
		candidate := c
		candidate.s = math.Min(candidate.s, 0.35)
		if contrast(candidate.color(), bgColor) >= minContrast {
			if dark && candidate.l > best.l-0.2 {
				return candidate
			}
			if !dark && candidate.l < best.l+0.2 {
				return candidate
			}
		}
	}

	// Otherwise walk lightness until it is readable. This always terminates:
	// pure white and pure black bracket every background.
	for range 100 {
		if contrast(best.color(), bgColor) >= minContrast {
			break
		}
		if dark {
			best.l = math.Min(1, best.l+0.01)
		} else {
			best.l = math.Max(0, best.l-0.01)
		}
	}
	return best
}

// pickAccents chooses three colorful, distinguishable colors, falling back to
// hue-shifted variants when the image is too monochrome to supply them.
func pickAccents(clusters []hsl, bg hsl, dark bool) [3]hsl {
	candidates := make([]hsl, 0, len(clusters))
	for _, c := range clusters {
		if c.s < 0.15 {
			continue // a grey is not an accent
		}
		candidates = append(candidates, readable(c, dark))
	}

	// Colourfulness first, weight as the tie-break: an accent should be worth
	// noticing, but not a colour that appears in four pixels.
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].s*0.7+candidates[i].weight*0.3 >
			candidates[j].s*0.7+candidates[j].weight*0.3
	})

	var out [3]hsl
	chosen := 0
	for _, c := range candidates {
		if chosen == 3 {
			break
		}
		// Two accents 20° apart are one accent.
		distinct := true
		for i := range chosen {
			if hueDistance(out[i].h, c.h) < 25 {
				distinct = false
				break
			}
		}
		if distinct {
			out[chosen] = c
			chosen++
		}
	}

	// A monochrome image still needs three accents, so rotate the hue.
	base := out[0]
	if chosen == 0 {
		base = readable(hsl{h: bg.h, s: 0.6, l: 0.5}, dark)
		out[0] = base
		chosen = 1
	}
	for i := chosen; i < 3; i++ {
		shifted := base
		shifted.h = math.Mod(base.h+float64(i)*40, 360)
		out[i] = shifted
	}
	return out
}

// statusColor is a fixed hue wearing the palette's saturation and lightness,
// so success stays green while still belonging to the theme.
func statusColor(hue float64, reference hsl, dark bool) theme.Color {
	c := hsl{
		h: hue,
		s: clampFloat(reference.s, 0.45, 0.85),
		l: clampFloat(reference.l, 0.45, 0.70),
	}
	if !dark {
		c.l = clampFloat(c.l, 0.35, 0.55)
	}
	return c.color()
}

// readable pulls a color into the band that stays legible against the
// background this variant uses, and gives it enough saturation to read as an
// accent rather than as a tinted grey.
func readable(c hsl, dark bool) hsl {
	if dark {
		c.l = clampFloat(c.l, 0.55, 0.80)
	} else {
		c.l = clampFloat(c.l, 0.30, 0.55)
	}
	c.s = clampFloat(c.s, 0.35, 0.95)
	return c
}

func hueDistance(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 180 {
		return 360 - d
	}
	return d
}

// contrast is the WCAG contrast ratio, from 1 (identical) to 21 (black on
// white).
func contrast(a, b theme.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// relativeLuminance is the WCAG definition, which is not the same as the
// perceived brightness theme.Color reports: it is the one contrast ratios are
// defined against.
func relativeLuminance(c theme.Color) float64 {
	channel := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.04045 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

func rgbToHSL(r, g, b uint8) (h, s, l float64) {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))

	l = (max + min) / 2
	if max == min {
		return 0, 0, l
	}

	d := max - min
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}

	switch max {
	case rf:
		h = (gf - bf) / d
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/d + 2
	default:
		h = (rf-gf)/d + 4
	}
	return h * 60, s, l
}

func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	if s == 0 {
		v := uint8(math.Round(l * 255))
		return v, v, v
	}

	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q

	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	hk := h / 360

	channel := func(t float64) uint8 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		switch {
		case t < 1.0/6:
			return uint8(math.Round((p + (q-p)*6*t) * 255))
		case t < 1.0/2:
			return uint8(math.Round(q * 255))
		case t < 2.0/3:
			return uint8(math.Round((p + (q-p)*(2.0/3-t)*6) * 255))
		default:
			return uint8(math.Round(p * 255))
		}
	}
	return channel(hk + 1.0/3), channel(hk), channel(hk - 1.0/3)
}

func clampFloat(v, min, max float64) float64 {
	return math.Min(max, math.Max(min, v))
}
