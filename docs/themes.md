# Theme reference

A theme carries the whole appearance of the desktop. One theme drives SwayFX,
Waybar, Rofi, Foot, Dunst and swaylock, which is what keeps them consistent.

Themes are TOML files in `~/.config/rice/themes/`. A user theme shadows a
bundled theme with the same name, so you can start from a bundled one by copying
it under the same name and editing it.

```bash
rice theme list                  # what is available
rice theme show tokyo-night      # values after derivation
```

## The smallest useful theme

Everything not set is derived. These three colours already produce a complete
desktop:

```toml
name = "my-dark"

[colors]
background = "#181825"
foreground = "#cdd6f4"
primary    = "#89b4fa"
```

Use `rice theme show` to see what that derived, then set explicitly whatever you
want exact control over.

---

## Top level

| Key | Type | Meaning |
| --- | --- | --- |
| `name` | string | Theme name. Defaults to the file name. |
| `description` | string | Shown by `rice theme show`. |
| `variant` | string | `"dark"` or `"light"`. Defaults to whichever the background's brightness implies. |

---

## `[colors]`

The semantic palette. Applications never see ANSI indexes through this section —
that is `[terminal]`.

| Key | Required | Derived from, if unset |
| --- | --- | --- |
| `background` | **yes** | — |
| `foreground` | **yes** | — |
| `primary` | **yes** | — |
| `surface` | no | `background` lightened 4% |
| `surface_alt` | no | `surface` lightened 6% |
| `overlay` | no | `surface_alt` lightened 6% |
| `muted` | no | `foreground` mixed 45% into `background` |
| `secondary` | no | `primary` |
| `accent` | no | `secondary` |
| `success` | no | `primary` |
| `warning` | no | `secondary` |
| `error` | no | `#e06c75` |
| `border` | no | `surface_alt` |
| `border_focus` | no | `primary` |

Colours accept `#rgb`, `#rgba`, `#rrggbb` and `#rrggbbaa`, with or without the
leading `#`. Each adapter emits whichever form its application needs.

Validation rejects a theme where `background` and `foreground` are the same
colour, since nothing rendered from it would be readable.

### Where each colour lands

| Colour | Used for |
| --- | --- |
| `background` | Root background, bar background, terminal background |
| `surface` | Focused window background, notification background, bar modules |
| `surface_alt` | Unfocused chrome, selection background |
| `foreground` | Body text everywhere |
| `muted` | Inactive workspaces, window titles, placeholder text |
| `primary` | Focused border, focused workspace, launcher selection, indicators |
| `secondary` | Secondary accents, key highlight on the lock screen |
| `accent` | A third accent, e.g. the disk module |
| `success` / `warning` / `error` | Battery states, urgency levels, urgent windows |

Text drawn on top of `primary` or `error` uses black or white automatically,
whichever is readable.

---

## `[terminal]`

The 16-colour ANSI palette plus terminal extras. Any slot left unset is derived
from `[colors]`, so this section is optional — but a terminal palette is worth
spelling out, because derived ANSI colours are a compromise.

| Key | Type | Derived from, if unset |
| --- | --- | --- |
| `background` | colour | `colors.background` |
| `foreground` | colour | `colors.foreground` |
| `regular` | 8 colours | background, error, success, warning, primary, secondary, accent, darkened foreground |
| `bright` | 8 colours | `regular` lightened 20%, except slot 0 (`muted`) and 7 (`foreground`) |
| `selection_background` | colour | `colors.surface_alt` |
| `selection_foreground` | colour | `colors.foreground` |
| `cursor` | colour | `colors.primary` |
| `url` | colour | `colors.secondary` |

```toml
[terminal]
regular = ["#282828", "#cc241d", "#98971a", "#d79921", "#458588", "#b16286", "#689d6a", "#a89984"]
bright  = ["#928374", "#fb4934", "#b8bb26", "#fabd2f", "#83a598", "#d3869b", "#8ec07c", "#ebdbb2"]
```

Order is the ANSI one: black, red, green, yellow, blue, magenta, cyan, white.

---

## `[ui]`

Geometry shared across the desktop. This is why one theme changes the whole
look: the same radius reaches Sway's corners, Waybar's pills and Rofi's window.

| Key | Type | Default | Reaches |
| --- | --- | --- | --- |
| `radius` | int | `0` | Sway `corner_radius`, Waybar pills, Rofi, Dunst |
| `border_width` | int | `2` | Sway borders, Rofi border, Dunst frame, Waybar tooltip. `-1` draws none |
| `gaps_inner` | int | `0` | Sway `gaps inner` |
| `gaps_outer` | int | `0` | Sway `gaps outer` |
| `padding` | int | `4` | Waybar, Rofi, Dunst padding |
| `horizontal_padding` | int | `8` | Same, horizontally |
| `opacity` | float | `1` | Foot background alpha, Waybar background, Dunst transparency |
| `blur_radius` | int | `0` | SwayFX blur; `0` disables blur entirely |
| `blur_passes` | int | `1` when blur is on | SwayFX |
| `blur_noise` | float | `0` | SwayFX |
| `shadow_blur` | int | `0` | SwayFX shadows; `0` disables them |
| `dim_inactive` | float | `0` | SwayFX `default_dim_inactive` |

Fractions (`opacity`, `blur_noise`, `dim_inactive`) must be between 0 and 1;
integers must not be negative.

`border_width`, `radius` and the two border colours in `[colors]` are one
border: the compositor draws it around every window, and the surfaces it does
not decorate — the bar, the launcher, notifications — draw the same one
themselves. An application can be given a border of its own in `config.toml`;
see [configuration.md](configuration.md#borders).

Zero is "unset" here, as it is everywhere in a theme file: `border_width = 0`
asks for the default of 2 rather than for no border. A desktop with no border
at all is `border_width = -1`, which every surface follows.

`opacity` deliberately does not make every window translucent — it applies to
the terminal background, the bar and notifications, where translucency reads
well.

---

## `[fonts]`

| Key | Type | Default | Used by |
| --- | --- | --- | --- |
| `ui_family` | string | `"sans-serif"` | Sway titles, Rofi, Dunst, swaylock |
| `ui_size` | int | `11` | The same |
| `mono_family` | string | `"monospace"` | Foot |
| `mono_size` | int | `ui_size` | Foot |
| `bar_family` | string | `ui_family` | Waybar |
| `bar_size` | int | `ui_size` | Waybar |

---

## `[icons]`, `[cursor]`, `[gtk]`

```toml
[icons]
theme = "Papirus-Dark"
size = 24
paths = [
  "/usr/share/icons/Papirus-Dark/16x16/status",
  "/usr/share/icons/hicolor/48x48/apps",
]

[cursor]
theme = "BreezeX-Light"
size = 28

[gtk]
theme = "Adwaita-dark"
prefer_dark = true
kvantum_theme = "KvGnomeDark"
qt_style_override = "kvantum"
```

`icons.theme` feeds Dunst, GTK, Qt and, unless `rofi.icon_theme` overrides it,
Rofi. `icons.paths` is Dunst's icon lookup path.

`icons.size` is the pixel size icons are drawn at, and reaches Rofi's launcher
icons; it defaults to 24. It does **not** reach GTK or Qt: neither toolkit has
a global icon-size setting, so there is nothing for Rice to write.

`cursor` becomes the Sway seat cursor theme, the GTK cursor setting, and
`XCURSOR_THEME`/`XCURSOR_SIZE` in the session environment.

`[gtk]` carries the toolkit hints that have no equivalent in the semantic
palette: `theme` is the GTK widget theme, `prefer_dark` the dark preference,
`kvantum_theme` the Kvantum theme, and `qt_style_override` the Qt widget style
— set it to `kvantum` to use Kvantum. Which files these end up in is a
configuration decision: see [`[gtk]`](configuration.md#gtk) and
[`[qt]`](configuration.md#qt).

---

## Deriving a theme from an image

```bash
rice theme from-image ~/Pictures/wallpaper.jpg           # print it
rice theme from-image ~/Pictures/wallpaper.jpg --save    # keep it
```

The image supplies a background, a readable foreground and three accents.
Everything this page describes as derived is left unset, so the result reads
like a hand-written theme and keeps following its own semantic colours.

An image says what colours to use and nothing else, so the fonts, the icon and
cursor themes, the geometry and the toolkit hints are **inherited** from an
existing theme — by default the one `config.toml` selects, or `--from <theme>`.
The result is that theme wearing the image's colours. Without this a derived
theme would have a palette and nothing else, and would name no font at all.

The widget theme is the exception: if the derived variant differs from the base
theme's, the GTK and Kvantum themes are dropped so normalization can pick ones
that match. A dark widget theme on a light palette is worse than none.

Three things are enforced rather than hoped for:

* **Contrast.** The foreground is pushed until it clears a 7:1 ratio against
  the background. A wallpaper is no excuse for an unreadable terminal. Override
  with `--min-contrast`.
* **Meaning.** `success`, `warning` and `error` keep their own hues and borrow
  only the palette's saturation and lightness. A green that is not green is
  worse than one that does not match the wallpaper.
* **Determinism.** The same image always produces the same theme. A palette
  that changes between runs is not a theme.

| Flag | Does |
| --- | --- |
| `--name` | Theme name; defaults to the image's file name |
| `--variant` | Force `dark` or `light`; by default the image decides |
| `--colors` | How many colours to quantize to (default 12) |
| `--min-contrast` | Lowest acceptable foreground contrast ratio (default 7) |
| `--save` | Write into the theme directory instead of printing |
| `--force` | Overwrite an existing theme of the same name |
| `--from` | Theme to inherit fonts, icons and geometry from |
| `--apply` | Save it, select it in `config.toml` and build a generation |
| `--wallpaper` | Point `sway.wallpaper` at the same image |

Together, those last two are the whole job in one command:

```bash
rice theme from-image ~/Pictures/wall.jpg --apply --wallpaper
```

The desktop and its palette then come from one file, which is the point.

PNG and JPEG are supported. The quantizer is plain Go — no matugen, no external
tool — so this works wherever Rice does.

---

## Bundled themes

| Name | Variant | Notes |
| --- | --- | --- |
| `catppuccin-mocha` | dark | Soft pastels, larger radius, two blur passes |
| `catppuccin-latte` | light | The same palette in daylight |
| `dracula` | dark | High contrast, vivid pastel accents |
| `gruvbox-dark` | dark | Warm, low contrast, explicit ANSI palette |
| `gruvbox-light` | light | The same warmth on cream paper |
| `nord` | dark | Arctic, north-bluish, muted frost accents |
| `rose-pine` | dark | Muted plum with rose and pine accents |
| `rose-pine-dawn` | light | The same at daybreak, on warm paper |
| `solarized-dark` | dark | Schoonover's precision palette |
| `solarized-light` | light | The same on its light background |
| `tokyo-night` | dark | Cool blues, high-contrast accents |

Every bundled theme is held to the same standard by the test suite: its variant
must match its background, text must clear the WCAG AA contrast ratio against
what it sits on, surfaces must be distinguishable from the background, and the
whole desktop must render and pass each application's own validator.

Solarized is the one deliberate deviation. It names two text tones, and the
body tone is below the readability line on its light background, so Rice uses
the emphasized one as `foreground` and leaves the ANSI palette canonical.

Copy one as a starting point:

```bash
rice theme show gruvbox-dark          # see the resolved values
rice render --theme catppuccin-mocha  # see what it generates
```

## Writing a theme

```bash
$EDITOR ~/.config/rice/themes/my-dark.toml
rice theme show ./my-dark.toml       # validate and inspect before installing
rice theme apply my-dark             # set it and build a generation
rice rollback                        # if you dislike it
```

Validation collects every problem at once rather than failing on the first, so a
broken theme reports all of its errors in one run.
