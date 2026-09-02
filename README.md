# Rice

Rice generates complete configuration files for a minimal SwayFX desktop from a
single theme and a single source configuration.

```
themes/*.toml  +  config.toml
                 │
                 ▼
          text/template
                 │
                 ▼
     generations/000042/          immutable, validated
                 │
                 ▼
             current              one symlink
```

One theme drives SwayFX, Waybar, Rofi, Foot, Dunst, swaylock and the GTK/Qt
toolkits at once, so the whole desktop stays consistent. Generated files are ordinary, readable
application configuration: nothing depends on Rice being installed to work.

## Status

Working: themes, templates, adapters, output validation, generations, `current`
switching, rollback, ownership detection, adoption with backups, deployment by
symlink, reload, uninstall, GTK/Qt integration, and the interactive editor.

Not yet: live preview as a committed generation, the `rice run` desktop
utilities, and a GUI.

Nothing under `~/.config` is touched until you run `rice setup --adopt`, and
that command is a dry run without the flag.

## Requirements

* Go 1.27 or newer to build.
* At runtime, whichever of SwayFX, Waybar, Rofi, Foot, Dunst and swaylock you
  enable. Rice generates configuration for them; it does not install them.

## Build

```bash
make build            # -> build/rice
make install          # -> ~/.local/bin/rice   (override with PREFIX=/usr/local)
```

`make help` lists every target:

| Target | Does |
| --- | --- |
| `build` | Compile the CLI into `build/` |
| `release` | Cross-compile linux/amd64 and linux/arm64 plus `SHA256SUMS` |
| `install` / `uninstall` | Install into `$(PREFIX)/bin`, default `~/.local` |
| `test` / `test-race` | Run the test suite |
| `cover` | Coverage profile into `build/coverage.out` |
| `golden` | Accept intentional template changes |
| `check` | `vet`, tests and a formatting check |
| `fmt` / `vet` / `tidy` | Formatting, vetting, module pruning |
| `demo` | Build a generation in `/tmp/rice-test` and list it |
| `clean` | Remove `build/` |

Without make:

```bash
go build -o build/rice ./cmd/rice
```

The binary is self-contained: templates and themes are embedded, so no
`/usr/share/rice` is needed.

## Quick start

```bash
make build
./build/rice                       # pick and tweak a theme interactively
./build/rice init                  # write ~/.config/rice/config.toml
./build/rice apply                 # build a generation and make it current
./build/rice setup                 # show what deploying would do — changes nothing
./build/rice setup --adopt         # back up what exists, then link
```

The result:

```
~/.config/rice/current/
├── dunst/dunstrc
├── foot/foot.ini
├── gtk/settings.ini
├── qt/qt5ct.conf, qt6ct.conf, kvantum.kvconfig
├── rofi/config.rasi
├── sway/config, environment.conf
├── swaylock/config
└── waybar/config.jsonc, style.css
```

To try everything without touching your real configuration, point `--root`
somewhere else — every command honours it:

```bash
make demo                          # does exactly this in /tmp/rice-test
./build/rice --root /tmp/rice-test theme apply tokyo-night
```

Deployment replaces each application's config path with a symlink into
`current/`, after copying anything that was there into `backups/`. Both the
original location and its backup are recorded, so `rice uninstall` puts
everything back. See [docs/deployment.md](docs/deployment.md) for exactly what
is and is not touched.

## The interactive editor

```bash
rice                             # or: rice tui
```

Pick a theme, adjust its colors, fonts, sizing, icons and cursor, then preview
any component by running the real application against the draft — Foot, Rofi,
Waybar or a nested Sway, rendered into a private temporary directory. Nothing
under `~/.config` is touched and `current` does not move unless you apply.

Per-program settings are editable too — bar height, launcher width, terminal
padding, notification timeouts — and are saved to `config.toml` rather than the
theme, so they survive a change of palette.

The editor also copies a single program's generated configuration to the
clipboard, so Rice is usable as a configuration generator without adopting the
rest of the ricing system.

Full key reference in [docs/editor.md](docs/editor.md).

## Commands

```bash
rice tui                         # the interactive editor (also bare `rice`)
rice init                        # write config.toml (--force to overwrite)
rice apply                       # build a generation, switch, redeploy, reload
rice apply -m "bigger gaps"      # record a description in the manifest
rice apply --no-switch           # build without changing current
rice apply --no-reload           # deploy without poking running applications
rice apply --theme tokyo-night   # build with another theme, once

rice status                      # theme, generations, ownership, dependencies,
                                 # and whether the theme's assets exist
rice doctor                      # same thing, under the name you reached for
rice setup                       # what deploying would do; changes nothing
rice setup --adopt               # back up existing files, then link
rice setup --adopt --force       # also take over symlinks Rice does not own
rice uninstall                   # what restoring would do; changes nothing
rice uninstall --yes             # remove links, restore the originals

rice render                      # print every generated file to stdout
rice render -c foot              # only one component
rice render -o /tmp/preview      # write files to a directory instead

rice theme list                  # bundled and user themes, * marks the active one
rice theme show                  # resolved values of the active theme
rice theme show tokyo-night      # ... or of any other theme
rice theme current
rice theme apply tokyo-night     # set the theme in config.toml and build

rice generation list             # history, newest first
rice generation current
rice generation show 42          # manifest: theme, parent, files, hashes

rice rollback                    # back to the previous generation
rice rollback 39                 # or to a specific one
```

Global flags: `--root DIR` overrides the Rice root, otherwise `$RICE_HOME`, then
`$XDG_CONFIG_HOME/rice`, then `~/.config/rice`. `--config-dir DIR` overrides
where applications keep their configuration, otherwise `$XDG_CONFIG_HOME`, then
`~/.config` — useful for rehearsing deployment somewhere harmless.

Every command carries its own help and examples:

```bash
rice --help
rice apply --help
rice theme show --help
rice completion zsh > ~/.zfunc/_rice    # shell completions
```

## Layout

```
~/.config/rice/
├── config.toml          source configuration (structure, not appearance)
├── themes/              user themes, shadowing bundled ones by name
├── templates/           user template overrides, per file
├── generations/000042/  generated output plus manifest.toml
├── state/               previous-generation tracking
└── current -> generations/000042
```

Generations are immutable. Edits belong in `config.toml` or a theme, followed by
`rice apply`; anything written into a generation is lost on the next one.

## Documentation

| Page | Covers |
| --- | --- |
| [docs/editor.md](docs/editor.md) | The interactive editor, key by key |
| [docs/deployment.md](docs/deployment.md) | Ownership, adoption, backups, reload, uninstall |
| [docs/configuration.md](docs/configuration.md) | Every `config.toml` key |
| [docs/themes.md](docs/themes.md) | The theme format and what gets derived |
| [docs/templates.md](docs/templates.md) | Overriding templates, context, functions |
| [docs/architecture.md](docs/architecture.md) | How the pieces fit, and what is missing |

The sections below are the short version.

## Themes vs configuration

**Appearance lives in a theme.** A theme carries a semantic palette, a 16-color
ANSI palette, geometry, fonts, icons and cursor:

```toml
name = "my-dark"

[colors]
background = "#181825"
foreground = "#cdd6f4"
primary    = "#89b4fa"

[ui]
radius = 8
gaps_inner = 8
opacity = 0.94
blur_radius = 4

[fonts]
mono_family = "JetBrainsMono Nerd Font"
mono_size = 12
```

Everything omitted is derived, so those three colors already render a complete
desktop: surfaces, borders, muted text and all sixteen ANSI slots are computed
from them. Set them explicitly whenever you want exact control —
[docs/themes.md](docs/themes.md) lists every key and what it derives from.

Drop the file in `~/.config/rice/themes/` and it appears in `rice theme list`; a
user theme shadows a bundled theme of the same name.

Bundled: `gruvbox-dark`, `catppuccin-mocha`, `tokyo-night`.

**Structure lives in `config.toml`**: which components to generate, outputs,
workspaces, key bindings, binding modes, window rules, workspace assignments,
startup programs, input settings, idle timeouts, and per-application options.
`rice init` writes every default out explicitly, so the file documents itself;
[docs/configuration.md](docs/configuration.md) explains what each key means.

A partial `config.toml` is merged onto the defaults — list only what you change:

```toml
theme = "tokyo-night"

[components]
waybar = false          # Rice then generates Sway's own bar block instead

[sway]
mod = "Mod4"

[[sway.bindings]]
keys = "$mod+Return"
command = "exec $terminal"
```

Every component also accepts an `extra` string, appended verbatim to the
generated file, for anything the templates do not model:

```toml
[sway]
extra = """
for_window [app_id="mpv"] floating enable
"""
```

## Overriding a template

Copy a built-in template into `~/.config/rice/templates/` under the same
relative path and edit it. Rice prefers your copy for that one file and keeps
using the built-ins for everything else.

```
~/.config/rice/templates/waybar/style.css.tmpl
```

Templates are `text/template` with a colour-aware function set: `bare`, `hex`,
`bareA`, `rgba`, `alpha`, `lighten`, `darken`, `mix`, `contrast`, plus
arithmetic, `json`, `font`, `indent`, `quote` and `default`. Rendering fails
loudly on an unknown field rather than emitting an empty value.

The full context and function reference is in
[docs/templates.md](docs/templates.md).

## Safety

Rice validates its own output before committing anything: brace balance for Sway
and Rofi, key/value shape for Foot and Dunst, a real JSON parse for the Waybar
bar configuration, option shape for swaylock. A generation that fails validation
is never committed.

Generations are assembled in a staging directory and renamed into place, and
`current` is replaced by rename as well. A failed build leaves nothing behind,
and switching generations is atomic.

Where Rice touches files it did not create, it is deliberately conservative:

* `rice setup` and `rice uninstall` are dry runs without `--adopt` / `--yes`.
* An existing file is copied into `backups/` and flushed to disk **before** the
  original is removed, so an interrupted adoption cannot lose it.
* A symlink Rice does not own is refused; `--force` replaces the link and never
  reads or modifies what it pointed at.
* A directory standing where a file belongs is never touched, forced or not.
* `rice apply` only redeploys paths already adopted. It never adopts on its own.
* Uninstall skips any path that is no longer the symlink it installed.
* Rice refuses to run as root, and never deletes a backup.

## Development

```bash
make check             # vet, tests, formatting
make golden            # accept intentional template changes
```

Golden files in `testdata/golden/` hold the full generated output for every
bundled theme, so a template change shows up as a reviewable diff. Review it
before committing.

Package layout and the reasoning behind it are in
[docs/architecture.md](docs/architecture.md).

## License

See [LICENSE](LICENSE).
