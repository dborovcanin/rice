# The interactive editor

```bash
rice tui                      # open the editor
rice tui --theme tokyo-night  # start from a theme other than the configured one
rice                          # same thing, when stdout is a terminal
```

The editor writes nothing until you ask it to. Previews render into a private
temporary directory, nothing under `~/.config` is touched, and `current` does
not move unless you apply.

The CLI is unaffected: every subcommand still works, and `rice` in a pipeline
still prints help rather than opening an interface.

---

## Screens

```text
Theme picker  ──enter──►  Editor  ──g──►  Programs
                            │                │
                            │                ├── p  preview
                            │                └── y  copy
                            │
                            ├── s  save as a user theme
                            └── a  save and apply
```

`q` or `esc` goes back a screen; `q` on the picker leaves.

---

## Picker

Every bundled and user theme, with its palette drawn beside it. Choosing a
theme makes it the base for the draft.

Switching base **discards** an unsaved draft, so the editor asks first. An
override that made sense against one palette rarely makes sense against
another, and carrying edits across silently would be worse than losing them
loudly.

---

## Editor

Groups on the left, fields on the right. `tab` moves between them.

| Group | Holds |
| --- | --- |
| Colors | The semantic palette: background, surface, foreground, primary, … |
| Terminal | The 16 ANSI slots, cursor, selection and URL colors |
| Fonts | UI, mono and bar families and sizes |
| Sizing | Radius, borders, gaps, padding, opacity, blur, dim |
| Icons & Cursor | Icon theme and size, cursor theme and size |

| Key | Does |
| --- | --- |
| `↑` `↓` | Move |
| `enter` | Edit the field |
| `←` `→` | Nudge: lightness for a color, one step for a number |
| `r` | Reset the field to the base theme |
| `c` | Clear the field so it is derived again |
| `R` | Discard every change |
| `y` | Copy the draft theme file to the clipboard |
| `s` | Save as a user theme |
| `a` | Save and apply |
| `g` | Programs |
| `t` | Back to the picker |

The editor draws itself in the palette being edited, so a change to the
background or the primary color is visible immediately in its own chrome.

### Changed and derived

Each field is marked:

* **changed** — it differs from the base theme.
* **derived** — the theme does not spell it out, so normalization fills it in
  from something else. `terminal.background` with no value follows
  `colors.background`; `colors.surface` with no value is a lightened
  `colors.background`.

This matters when editing. If a theme leaves the terminal palette derived, then
changing `colors.background` moves the terminal background too. If the theme
spells the terminal palette out — as all three bundled themes do — then it does
not, because the theme's author said what they wanted.

`c` clears a value and hands it back to derivation. `r` puts back whatever the
base theme had, which may itself be nothing.

### Colors

Type a hex value in any form `theme.Color` accepts: `#rgb`, `#rrggbb` or
`#rrggbbaa`. A swatch appears beside the input as you type, and the field row
shows the color against the theme background and the theme foreground against
the color, so a pair can be judged rather than guessed at.

`←` and `→` move lightness in 5% steps, which is usually faster than computing
hex by hand.

### Fonts

`enter` on a font family opens the installed families, read from `fc-list`.
Type to filter; monospaced families are offered first for the mono and bar
roles.

A terminal cannot render an arbitrary family, so the picker shows names, not
specimens. To see the real font, preview the program that uses it: Foot for the
mono font, Waybar or Rofi for the UI font.

Without fontconfig installed, the filter box becomes a plain text field and the
family can still be typed.

---

## Programs

Programs on the left, that program's settings on the right. `tab` moves
between them.

| Key | Does |
| --- | --- |
| `p` | Preview: render the draft and run the real program against it |
| `y` | Copy that program's generated configuration to the clipboard |
| `x` | Stop a running preview |
| `enter` | Edit the setting; a switch or a fixed choice cycles in place |
| `←` `→` | Change the setting without opening a prompt |
| `r` | Reset the setting to what is in `config.toml` |
| `s` | Save |

### What lives here, and where it is saved

These settings are **structure**, so they live in `config.toml`, not in the
theme: a bar's height and a launcher's width are decisions about the desktop,
and they should survive a change of palette. Changing the theme in the picker
therefore keeps them; saving writes them to `config.toml` while the palette
goes to the theme file.

`config.toml` is only rewritten when a program setting actually changed.

The set is curated rather than exhaustive. Outputs, workspaces, bindings,
window rules and Waybar's module options are lists, and editing a list well
needs an interface of its own; until that exists they stay in `config.toml`,
where they are already comfortable to edit by hand.

| Program | Settings |
| --- | --- |
| `sway` | Modifier, wallpaper and mode, smart borders and gaps, focus follows mouse, titlebars |
| `waybar` | Position, layer, height, spacing |
| `rofi` | Width, lines, columns, icons and icon theme, drun label |
| `foot` | Server mode, shell, TERM, scrollback, padding, cursor style and blink |
| `dunst` | Origin, size, offset, gap, follow, the three timeouts, max icon size |
| `swaylock` | Image and scaling, blur, indicator radius and thickness, failed attempts, clock |

A preview renders the whole draft into a private directory under the system
temporary directory, validates it exactly as a real build would, and launches
one program pointed at those files. The directory is removed when the program
exits.

| Component | Preview |
| --- | --- |
| `foot` | Clean. |
| `rofi` | Clean. |
| `waybar` | A second bar appears over the running one until the preview closes. |
| `sway` | Runs nested, as a window inside the current session. |
| `dunst` | Not available: it would take the D-Bus name the running notification daemon owns. |
| `swaylock` | Asks first, because it locks the screen. |

Copying is there for using Rice as a configuration generator without the rest
of the ricing system: take the Rofi theme, paste it into your own setup, and
never run `rice setup`. The same output is available non-interactively through
`rice render -c rofi`.

Copying uses `wl-copy`, falling back to `xclip` and `xsel`.

---

## Saving and applying

`s` writes the draft to `~/.config/rice/themes/<name>.toml`. A user theme
shadows a bundled theme of the same name, so that is how a bundled theme is
customized in place — and why saving a bundled theme suggests a `-custom` name
rather than shadowing it by accident.

The saved file keeps derived values derived, so it reads like a theme someone
wrote rather than a dump of every resolved value.

Saving also writes any changed program settings to `config.toml`.

`a` saves and then applies: it writes the theme name into `config.toml`, builds
a generation, switches `current` to it, and repairs and reloads whatever is
already adopted. That is the same path `rice theme apply` takes — the editor
has no private route to `current`.

---

## Terminal support

Swatches need 24-bit color. Rice checks `COLORTERM` and says so when the
terminal does not advertise it, rather than quantizing every swatch to sixteen
colors and letting you pick a palette you cannot actually see.
