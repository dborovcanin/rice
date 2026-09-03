# Template reference

Rice renders every application configuration file from a Go `text/template`.
The built-in templates are embedded in the binary; you can override any single
one without forking the rest.

## Overriding one template

Put a file at the same relative path under `~/.config/rice/templates/`:

```
~/.config/rice/templates/
├── waybar/style.css.tmpl        ← yours
└── (everything else)            ← built in
```

Resolution is per file: Rice checks your directory first and falls back to the
embedded copy, so overriding `waybar/style.css.tmpl` leaves
`waybar/config.jsonc.tmpl` untouched.

| Component | Template | Generates |
| --- | --- | --- |
| sway | `sway/config.tmpl` | `sway/config` |
| waybar | `waybar/config.jsonc.tmpl` | `waybar/config.jsonc` |
| waybar | `waybar/style.css.tmpl` | `waybar/style.css` |
| rofi | `rofi/config.rasi.tmpl` | `rofi/config.rasi` |
| foot | `foot/foot.ini.tmpl` | `foot/foot.ini` |
| dunst | `dunst/dunstrc.tmpl` | `dunst/dunstrc` |
| swaylock | `swaylock/config.tmpl` | `swaylock/config` |

To start from the built-in copy, render it and edit from there:

```bash
rice render -c waybar -o /tmp/waybar
# or read the shipped template in the repository under templates/
```

Check the result before committing it:

```bash
rice render -c waybar          # fails loudly if your template is broken
rice apply                     # validation runs again before committing
```

---

## Context

Every template sees the same struct. The theme's parts are promoted to the top
level, so it is `.Colors.Primary`, not `.Theme.Colors.Primary`.

| Field | Contents |
| --- | --- |
| `.Colors` | Semantic palette — see [themes.md](themes.md) |
| `.Terminal` | 16-colour ANSI palette and terminal extras |
| `.UI` | Radius, borders, gaps, padding, opacity, blur, shadows |
| `.Fonts` | UI, mono and bar font families and sizes |
| `.Icons`, `.Cursor`, `.GTK` | Icon theme and paths, cursor theme and size, toolkit hints |
| `.Border` | The desktop's border: `.Width`, `.Color`, `.Focus`, `.Radius`, plus `.Focused` for the focused colour |
| `.Theme` | The whole theme, including `.Theme.Name` and `.Theme.Description` |
| `.Config` | The whole configuration, e.g. `.Config.Components.Waybar` |
| `.Commands` | Programs from `[commands]` |
| `.Sway`, `.Waybar`, `.Rofi`, `.Foot`, `.Dunst`, `.Swaylock` | Per-component settings |
| `.Generation` | The number this render is for |
| `.Version` | Rice version |
| `.Dark` | Whether the theme reads as dark |

Referencing a field that does not exist is an error, not an empty string, so a
typo fails the render instead of silently producing a broken config.

---

## Colour values

A colour renders as `#rrggbb`, or `#rrggbbaa` when it is translucent. Methods
and functions produce the other forms applications need.

| Form | Example output | Where it is needed |
| --- | --- | --- |
| `{{ .Colors.Primary }}` | `#d79921` | Sway, Rofi, Waybar CSS |
| `{{ bare .Colors.Primary }}` | `d79921` | `foot.ini` |
| `{{ bareA (alpha 0.7 .Colors.Surface) }}` | `32302fb3` | swaylock |
| `{{ .Colors.Primary.HexA }}` | `#d79921ff` | Anything wanting explicit alpha |
| `{{ .Colors.Primary.ARGB }}` | `ffd79921` | GTK-style `aarrggbb` |
| `{{ .Colors.Background.RGBA }}` | `rgba(40, 40, 40, 1)` | CSS |

---

## Functions

### Colour

| Function | Meaning |
| --- | --- |
| `hex c` | `#rrggbb`, dropping alpha |
| `bare c` | `rrggbb` |
| `bareA c` | `rrggbbaa` |
| `argb c` | `aarrggbb` |
| `rgba c` | `rgba(r, g, b, a)` |
| `alpha f c` | `c` with alpha set to the fraction `f` |
| `lighten f c` | Move `f` of the way toward white |
| `darken f c` | Move `f` of the way toward black |
| `mix f a b` | Blend `a` toward `b` by `f` |
| `contrast c` | Black or white, whichever is readable on `c` |

Fractions are clamped to 0..1, so `lighten 5` is white rather than an overflow.

### Numbers

| Function | Meaning |
| --- | --- |
| `add a b`, `sub a b`, `mul a b` | Integer arithmetic |
| `div a b` | Integer division; dividing by zero yields `0` |
| `scale f n` | `round(n * f)`, e.g. `scale .UI.Opacity 100` |
| `percentage f` | `0.925` → `92.5%` |
| `float f` | Shortest exact form: `1.0` → `1`, `0.5` → `0.5` |

### Text

`quote`, `join sep list`, `indent n s`, `trim`, `upper`, `lower`,
`replace old new s`, `contains sub s`, `hasPrefix prefix s`,
`default fallback value`, `comment s` (prefixes each line with `# `),
`lines s` (splits on newlines).

Argument order puts the operand last, so functions chain in pipelines.

### Structured output

| Function | Meaning |
| --- | --- |
| `json v` | Compact JSON, without HTML escaping |
| `jsonIndent n v` | Indented JSON, continuation lines padded by `n` spaces |
| `font family size` | `"JetBrainsMono Nerd Font 12"` for Pango |

`json` deliberately does not escape `<` and `>`: Waybar format strings contain
Pango markup, and `<` in a configuration file helps nobody.

---

## Examples

Sway colours and geometry:

```
{{- $border := .Border }}
client.focused {{ $border.Focus }} {{ .Colors.Surface }} {{ .Colors.Foreground }} {{ .Colors.Primary }} {{ $border.Focus }}

gaps inner {{ .UI.GapsInner }}
{{- if gt $border.Radius 0 }}
corner_radius {{ $border.Radius }}
{{- end }}
```

A surface the compositor does not decorate resolves its own override against
that same border, which is the whole of what "one border, everywhere" means:

```
{{- $border := .Rofi.Border.Resolve .Border.Focused }}
window {
    border:        {{ $border.Width }}px;
    border-color:  {{ $border.Color }};
    border-radius: {{ $border.Radius }}px;
}
```

`.Border.Focused` is the border in the colour a focused window gets. A
launcher is the surface you are looking at, so it uses that; a notification is
not, so it uses `.Border` as it stands.

Waybar CSS, deriving a hover state rather than hardcoding one:

```
#workspaces button:hover {
  background: {{ .Colors.SurfaceAlt }};
  color: {{ contrast .Colors.SurfaceAlt }};
}
```

Iterating over configuration:

```
{{ range $ws := .Sway.Workspaces }}
bindsym $mod+{{ $ws.Key }} workspace $ws{{ $ws.Key }}
{{- end }}
```

Branching on an enabled component:

```
{{- if .Config.Components.Waybar }}
exec waybar
{{- end }}
```

---

## Validation

Generated output is validated before a generation is committed, so a broken
template cannot produce a desktop that will not start:

| Component | Check |
| --- | --- |
| sway | Braces balance across the file |
| rofi | Braces balance |
| foot | Every line is a section header or `key=value` |
| dunst | Same, plus a `[global]` section must exist |
| waybar | The bar configuration parses as JSON, with JSONC comments stripped |
| swaylock | Every line is a bare long option, not dash-prefixed |

A generation that fails validation is discarded whole; nothing is left behind
and no generation number is consumed.

---

## Golden files

The repository keeps the full generated output for every bundled theme under
`testdata/golden/`. A template change shows up as a reviewable diff:

```bash
make test        # fails, showing what changed
make golden      # accept the change
git diff         # review it before committing
```
