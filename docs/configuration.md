# config.toml reference

`config.toml` holds **structure**: which components Rice generates, what the
compositor does, and which programs the bindings launch. Appearance is not here
— it lives in a [theme](themes.md).

The file is merged onto complete defaults, so list only what you change. Unknown
keys are an error, and the message names the offending key.

`rice init` writes every default out explicitly; this page explains what those
keys mean.

---

## Top level

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `theme` | string | `"gruvbox-dark"` | Theme name, or a path to a `.toml` file. |

---

## `[components]`

Which applications Rice generates configuration for. Disabling a component skips
its templates entirely.

| Key | Default |
| --- | --- |
| `sway` | `true` |
| `waybar` | `true` |
| `rofi` | `true` |
| `foot` | `true` |
| `dunst` | `true` |
| `swaylock` | `true` |
| `gtk` | `true` |
| `qt` | `true` |

Disabling `waybar` makes the Sway template emit its own themed `bar {}` block
instead, so you never end up with no bar at all.

`gtk` and `qt` are toolkit integration rather than applications: they make GTK
and Qt programs follow the theme instead of looking like they belong to another
desktop. See [`[gtk]`](#gtk) and [`[qt]`](#qt).

---

## `[generations]`

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `keep` | int | `10` | How many generations to retain. The current and previous generations are never pruned, whatever the limit. |

---

## `[commands]`

The programs bindings launch. Each becomes a Sway variable, so templates never
hardcode a binary name and you can change a terminal in one place.

| Key | Sway variable | Default |
| --- | --- | --- |
| `terminal` | `$terminal` | `footclient` |
| `terminal_spawn` | `$terminal_spawn` | `foot` |
| `launcher` | `$launcher` | `rofi -show drun` |
| `browser` | `$browser` | `brave` |
| `editor` | `$editor` | `codium` |
| `lock` | `$lock` | `swaylock` |
| `screenshot` | `$screenshot` | `grim -g "$(slurp)" - \| wl-copy` |
| `clipboard` | `$clipboard` | `cliphist list \| rofi -dmenu \| cliphist decode \| wl-copy` |
| `emoji` | `$emoji` | unset |
| `volume_up` | `$volume_up` | `wpctl set-volume -l 1.5 @DEFAULT_AUDIO_SINK@ 5%+` |
| `volume_down` | `$volume_down` | `wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%-` |
| `volume_mute` | `$volume_mute` | `wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle` |
| `mic_mute` | `$mic_mute` | `wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle` |
| `brightness_up` | `$brightness_up` | `brightnessctl set 5%+` |
| `brightness_down` | `$brightness_down` | `brightnessctl set 5%-` |

Commands run through `sh -c`, so pipes, quotes and `$(...)` work.

---

## `[sway]`

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `mod` | string | `"Mod4"` | Modifier key. |
| `wallpaper` | path | unset | Default background for every output. `~` and `$VARS` expand. |
| `wallpaper_mode` | string | `"fill"` | `stretch`, `fill`, `fit`, `center`, `tile`. |
| `smart_borders` | bool | `true` | Hide borders when a workspace holds one window. |
| `smart_gaps` | bool | `true` | Same, for gaps. |
| `focus_follows_mouse` | bool | `false` | |
| `titlebar` | bool | `false` | `false` uses `default_border pixel`, taking its width from the theme. |
| `extra` | string | unset | Appended verbatim to the generated Sway config. |

Geometry — gaps, border width, corner radius, blur, shadows — comes from the
theme's `[ui]` section, not from here.

### `[[sway.outputs]]`

| Key | Type | Meaning |
| --- | --- | --- |
| `name` | string | Output name, e.g. `eDP-1`. Required. |
| `resolution` | string | e.g. `1920x1200`. Omit to leave it to Sway. |
| `position` | string | e.g. `1920 0`. |
| `scale` | float | Defaults to `1`. |
| `transform` | string | `normal`, `90`, `flipped`, … |
| `wallpaper` | path | Per-output override of `sway.wallpaper`. |
| `disabled` | bool | Emits `disable` instead of a layout. |

```toml
[[sway.outputs]]
name = "eDP-1"
resolution = "1920x1200"
position = "0 0"
scale = 1
```

### `[[sway.workspaces]]`

| Key | Type | Meaning |
| --- | --- | --- |
| `key` | string | The key `$mod+<key>` switches to this workspace. |
| `name` | string | Workspace name, shown in the bar. |
| `output` | string | Pin the workspace to an output. |

Each entry generates a `$mod+key` switch binding and a `$mod+Shift+key` move
binding, so workspace bindings do not have to be written out by hand.

### `[[sway.bindings]]`

| Key | Type | Meaning |
| --- | --- | --- |
| `keys` | string | e.g. `$mod+Return`. Required. |
| `command` | string | Any Sway command, not only `exec`. Required. |
| `comment` | string | Emitted as a comment above the binding. |

```toml
[[sway.bindings]]
keys = "$mod+Shift+s"
command = "exec $screenshot"
comment = "screenshot region"
```

### `[[sway.modes]]`

| Key | Type | Meaning |
| --- | --- | --- |
| `name` | string | Mode name. Required. |
| `enter` | string | Binding that enters the mode. |
| `bindings` | list | Same shape as `sway.bindings`. |

`Return` and `Escape` are added automatically to leave the mode, and any binding
whose command starts with `exec` also returns to the default mode, so a system
menu behaves the way you expect.

```toml
[[sway.modes]]
name = "system"
enter = "$mod+Escape"

  [[sway.modes.bindings]]
  keys = "l"
  command = "exec $lock"
```

### `[[sway.window_rules]]` and `[[sway.assigns]]`

```toml
[[sway.window_rules]]
criteria = '[app_id="sticky_term"]'
commands = "floating enable, sticky enable, resize set 1400 1000"

[[sway.assigns]]
criteria = '[app_id="Slack"]'
workspace = "1"          # the key of a sway.workspaces entry
```

### `[[sway.startup]]`

| Key | Type | Meaning |
| --- | --- | --- |
| `command` | string | The program to launch. |
| `always` | bool | `true` emits `exec_always`, re-running it on every reload. |
| `comment` | string | Emitted as a comment. |

Waybar, Dunst, the Foot server and swayidle are started from their own component
settings, so they do not need entries here.

### Session environment

```toml
[sway]
write_environment = true

[sway.environment]
MOZ_ENABLE_WAYLAND = "1"
```

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `write_environment` | bool | `true` | Write `environment.d/50-rice.conf`. |
| `environment` | table | empty | Extra variables to put in that file. |

Sway's configuration language cannot export a variable, so the cursor theme and
the Qt platform theme have to be set somewhere the session reads at startup.
`environment.d` is that place: the systemd user manager reads it at login.

Rice always writes `XCURSOR_THEME` and `XCURSOR_SIZE` there, and
`QT_QPA_PLATFORMTHEME` when the `qt` component is enabled.

**A change to this file needs a fresh session, not a Sway reload.**

---

### `[sway.keyboard]`

| Key | Type | Default |
| --- | --- | --- |
| `layout` | string | `"us,rs,rs"` |
| `variant` | string | `",latin,"` |
| `options` | string | `"grp:alt_shift_toggle"` |
| `repeat_delay` | int | unset |
| `repeat_rate` | int | unset |

### `[sway.touchpad]`

| Key | Type | Default |
| --- | --- | --- |
| `tap` | bool | `true` |
| `tap_button_map` | string | `"lrm"` |
| `natural_scroll` | bool | `false` |
| `disable_while_typing` | bool | `true` |
| `middle_emulation` | bool | `true` |
| `drag_lock` | bool | `false` |
| `accel_profile` | string | unset |
| `pointer_accel` | float | unset |

### `[sway.idle]`

Timeouts in seconds; `0` disables one.

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `enabled` | bool | `true` | Emit a swayidle invocation at all. |
| `lock_after` | int | `300` | Lock the screen. |
| `screen_off` | int | `600` | Power the outputs off. |
| `sleep_after` | int | `0` | Suspend. |
| `lock_on_sleep` | bool | `true` | Lock before sleeping. |

---

## Borders

The desktop has one border: a width, a colour, a focused colour and a corner
radius. It comes from the theme — `ui.border_width`, `ui.radius`,
`colors.border` and `colors.border_focus` — and SwayFX draws it around every
window.

The surfaces the compositor does not decorate draw the same border themselves,
so the bar, the launcher and notifications match the windows around them
without being told about them twice. A theme switch moves all four together.

An application that draws its own frame can be given a different one:

```toml
[rofi.border]
width = 4
color = "#d79921"
radius = 16
```

`width`, `color` and `radius` are independent: whatever is left unset follows
the desktop, so a theme switch still moves it. Zero and empty mean unset — as
they do throughout the theme format — so zero is not how a border is removed:
that is `-1`, which is a decision rather than an absence.

```toml
[rofi.border]
width = -1   # no frame at all
radius = -1  # square corners, however round the desktop is
```

The desktop itself works the same way: `ui.border_width = -1` in the theme
leaves every window unbordered, and Sway is told `default_border none` rather
than asked for a border zero pixels wide.

`[waybar.border]`, `[rofi.border]` and `[dunst.border]` are the three that
exist. There is no `[foot.border]`: a terminal is an ordinary window, SwayFX
draws its border, its colour and its corner radius, and a setting here would
change nothing. The rule is which surface does the drawing — a layer-shell
surface has its own frame, a window has the compositor's.

Where each one lands:

| Section | Reaches |
| --- | --- |
| `[waybar.border]` | The tooltip frame and the corners of the modules and workspace pills |
| `[rofi.border]` | The launcher's window frame and its inner corners |
| `[dunst.border]` | The notification frame, its corners and the separator |

Rofi and Waybar follow the focused colour, because they are surfaces you are
looking at; Dunst follows the resting one, because a notification is something
you glance at while looking elsewhere. Dunst's urgent frames keep their own
colours either way: an urgent notification should not look like a quiet one.

---

## `[waybar]`

| Key | Type | Default |
| --- | --- | --- |
| `design` | string | `"powerline"` — the bar's shape; see below |
| `position` | string | `"top"` |
| `layer` | string | `"top"` |
| `height` | int | `32` |
| `spacing` | int | `4` |
| `modules_left` | list | `["sway/workspaces", "sway/mode"]` |
| `modules_center` | list | `["sway/window"]` |
| `modules_right` | list | `["pulseaudio", "cpu", "memory", "disk", "battery", "clock", "tray"]` |
| `border.width`, `border.color`, `border.radius` | int, colour, int | unset — follow the desktop's border |
| `extra_css` | string | unset — appended to `style.css` |

### Designs

A design is the bar's **shape**: what a module sits on, what separates it from
the next one, how round it is. It is never its colour. Every design is a
template written against the theme, so switching designs keeps the palette and
switching themes keeps the shape.

```toml
[waybar]
design = "islands"
```

| Design | Look |
| --- | --- |
| `powerline` | Flush segments separated by arrows. The default; needs a Nerd Font on the bar |
| `pills` | Every module a rounded pill on a solid bar |
| `islands` | No bar at all: each group floats as its own rounded island |
| `capsule` | One long capsule, floating clear of the screen edge |
| `split` | The two ends carry everything and the middle is left alone |
| `solid` | One opaque strip, ruled between modules |
| `minimal` | No chrome: colour and spacing carry the bar |
| `compact` | As little bar as the modules need |
| `outline` | Outlined modules with the wallpaper showing through |
| `underline` | Flat, with a rule under whatever is worth looking at |
| `shadow` | Modules lifted off a translucent bar by their shadows |
| `mono` | One colour, until something needs attention |
| `retro` | Hard boxes and square corners, from before rounding |
| `neon` | Dark, with everything that matters glowing in its own colour |

Most designs are a stylesheet and nothing else, so they work with whatever
modules are configured. `powerline` is the exception: an arrow between two
segments is a module rather than a shape — CSS cannot cut a corner off a Waybar
module — so that design brings its own `config.jsonc` as well, wrapping the
configured modules in arrows and forcing `spacing` to zero. The module lists,
and everything under `[waybar.modules]`, are still yours.

The other things the theme decides still apply on top: `ui.radius` rounds a
design that rounds, `ui.opacity` sets how much of the wallpaper shows through,
and `[waybar.border]` overrides the desktop's border for the bar alone.

A design's template can be replaced the way any template can — put your own
`templates/waybar/designs/islands.css.tmpl` in `~/.config/rice` and that design
renders from your copy. The list of names is built in, so a new name needs a
new entry in Rice rather than only a file.

### `[waybar.modules.<name>]`

Per-module settings are passed straight through to the generated JSON, so any
Waybar option works without Rice modelling it:

```toml
[waybar.modules."clock"]
format = " {:%H:%M}"
tooltip-format = "<tt>{calendar}</tt>"

[waybar.modules."battery".states]
warning = 30
critical = 15
```

Module colours come from the theme; only behaviour belongs here.

The default formats lead with a Nerd Font glyph, because a row of bare
percentages does not say which one is the disk and which one is the battery.
They are the icons the bundled themes' bar font carries; a bar font without
them draws blank boxes, so either install one — the themes ask for
`JetBrainsMono Nerd Font` — or set `format` for those modules here without the
glyph.

---

## `[rofi]`

| Key | Type | Default |
| --- | --- | --- |
| `width` | string | `"30%"` |
| `lines` | int | `10` |
| `columns` | int | `1` |
| `show_icons` | bool | `true` |
| `icon_theme` | string | unset — falls back to the theme's icon theme |
| `font_family` | string | unset — falls back to the theme's interface font |
| `font_size` | int | `0` — falls back to the theme's interface size |
| `icon_size` | int | `0` — falls back to the theme's `icons.size` |
| `border.width`, `border.color`, `border.radius` | int, colour, int | unset — follow the desktop's border |
| `modes` | list | `["drun", "run", "window"]` |
| `display_drun` | string | `"Run"` |
| `extra` | string | Appended verbatim to `config.rasi`. |

A launcher is read at a glance from across the screen, so it often wants to be
larger than the rest of the desktop. These three override the theme for the
launcher alone and leave everything else at the theme's size:

```toml
[rofi]
font_family = "JetBrainsMono Nerd Font"
font_size = 16
icon_size = 40
```

Unset — empty or zero — means the theme decides, so a theme switch still moves
the launcher with everything else.

---

## `[foot]`

| Key | Type | Default |
| --- | --- | --- |
| `server` | bool | `true` — start `foot --server` from the Sway config |
| `shell` | string | unset |
| `term` | string | `"foot"` |
| `scrollback_lines` | int | `250000` |
| `pad_x`, `pad_y` | int | `8` |
| `cursor_style` | string | `"beam"` |
| `cursor_blink` | bool | `true` |
| `font_family` | string | unset — falls back to the theme's monospaced font |
| `font_size` | int | `0` — falls back to the theme's monospaced size |
| `extra` | string | Appended verbatim to `foot.ini`. |

Palette and background opacity come from the theme, and so does the font until
the terminal is given one of its own. A terminal is read for hours at a time,
and the size that suits it is not always the size that suits a bar or a
launcher:

```toml
[foot]
font_family = "Iosevka"
font_size = 13
```

Unset — empty or zero — means the theme decides, so a theme switch still moves
the terminal with everything else.

---

## `[dunst]`

| Key | Type | Default |
| --- | --- | --- |
| `origin` | string | `"top-center"` |
| `width` | int | `420` |
| `height` | int | `200` |
| `offset` | string | `"0x10"` |
| `gap_size` | int | `10` |
| `follow` | string | `"mouse"` |
| `timeout_low` | int | `5` |
| `timeout_normal` | int | `5` |
| `timeout_critical` | int | `0` (never expires) |
| `max_icon_size` | int | `64` |
| `font_family` | string | unset — falls back to the theme's interface font |
| `font_size` | int | `0` — falls back to the theme's interface size |
| `border.width`, `border.color`, `border.radius` | int, colour, int | unset — follow the desktop's border |
| `extra` | string | Appended verbatim to `dunstrc`. |

A notification is read from wherever you happen to be looking, in the seconds
before it expires, so it often wants to be larger than the interface around it:

```toml
[dunst]
font_family = "JetBrainsMono Nerd Font"
font_size = 14
```

Unset — empty or zero — means the theme decides, so a theme switch still moves
notifications with everything else.

---

## `[swaylock]`

| Key | Type | Default |
| --- | --- | --- |
| `image` | path | Falls back to `sway.wallpaper`; with neither, the theme background colour is used. |
| `scaling_mode` | string | `"fill"` |
| `blur` | string | unset, e.g. `"7x5"` |
| `indicator_radius` | int | `100` |
| `indicator_thickness` | int | `8` |
| `show_failed_attempts` | bool | `true` |
| `clock` | bool | `true` |
| `extra` | string | Appended verbatim. |

---

## `[gtk]`

```toml
[gtk]
settings = true
css = false
extra_css = ""
```

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `settings` | bool | `true` | Write `settings.ini` into both `gtk-3.0/` and `gtk-4.0/`: widget theme, icon theme, interface font, cursor, dark preference. |
| `css` | bool | `false` | Write a `gtk.css` mapping the palette onto libadwaita's named colours, so GTK 4 applications pick up the accent and window background. |
| `extra_css` | string | `""` | Appended verbatim to the generated `gtk.css`. |

`css` is off by default. It is the one part of toolkit integration that changes
how applications look rather than only which theme they load, and a GTK
application fighting its own theme looks worse than one that ignores Rice. Turn
it on when the palette matters more than the theme's own design.

Which widget theme, icon theme and Kvantum theme are used comes from the
theme's `[icons]`, `[cursor]` and `[gtk]` blocks, not from here: those are
appearance. See [themes.md](themes.md).

GTK has no global icon-size setting, so the theme's `icons.size` does not reach
it.

---

## `[qt]`

```toml
[qt]
qt5ct = true
qt6ct = true
kvantum = true
platform_theme = "qt5ct"
```

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `qt5ct` | bool | `true` | Write `qt5ct/qt5ct.conf`. |
| `qt6ct` | bool | `true` | Write `qt6ct/qt6ct.conf`. |
| `kvantum` | bool | `true` | Write `Kvantum/kvantum.kvconfig` selecting the theme's Kvantum theme. No effect unless the Kvantum style is installed. |
| `platform_theme` | string | `"qt5ct"` | What `QT_QPA_PLATFORMTHEME` is set to in the session environment. |

The widget style comes from the theme's `gtk.qt_style_override`, falling back
to `Fusion`. Set it to `kvantum` to use the Kvantum theme.

`platform_theme` only does anything because Rice writes it into
`environment.d/50-rice.conf` — see `sway.write_environment` below. Without that
variable, a generated `qt5ct.conf` is inert.

Qt has no global icon-size setting either.

---

## Escape hatches

Every component takes an `extra` string appended verbatim to its generated file,
for anything the templates do not model:

```toml
[sway]
extra = """
for_window [app_id="mpv"] floating enable
bindsym $mod+F1 exec my-script
"""
```

If you need to change the *shape* of a file rather than add to it, override its
template instead — see [templates.md](templates.md).
