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

One theme drives SwayFX, Waybar, Rofi, Foot, Dunst and swaylock at once, so the
whole desktop stays consistent. Generated files are ordinary, readable
application configuration: nothing depends on Rice being installed to work.

## Status

Configuration generation is implemented: themes, templates, adapters,
validation, generations, `current` switching and rollback.

Deployment into `~/.config/<app>` (ownership detection, adoption, backups,
symlinks, reload) is not wired up yet, so Rice currently writes only inside its
own root.

## Install

```bash
go build -o rice ./cmd/rice
```

## Use

```bash
rice init                        # write ~/.config/rice/config.toml
rice apply                       # build a generation and make it current
rice render -c foot              # preview one component on stdout
rice theme list                  # bundled and user themes
rice theme show tokyo-night      # resolved values, including derived colors
rice theme apply tokyo-night     # switch theme and build a generation
rice generation list             # history, newest first
rice rollback                    # back to the previous generation
```

`--root` points the whole tree somewhere else, which is useful for trying
changes without touching the real configuration:

```bash
rice --root /tmp/rice-test init
rice --root /tmp/rice-test apply
```

## Layout

```
~/.config/rice/
├── config.toml          source configuration (structure, not appearance)
├── themes/              user themes, shadowing the bundled ones by name
├── templates/           user template overrides, per file
├── generations/000042/  generated output plus manifest.toml
├── state/               previous-generation tracking
└── current -> generations/000042
```

## Themes vs configuration

Appearance lives in a theme; structure lives in `config.toml`. A theme carries a
semantic palette (`background`, `surface`, `primary`, `error`, …), a 16-color
ANSI palette, geometry, fonts, icons and cursor. Anything omitted is derived:
a theme that sets only `background`, `foreground` and `primary` still renders a
complete desktop.

`config.toml` carries outputs, workspaces, key bindings, modes, window rules,
startup programs, input settings and per-application behaviour. Every component
also takes an `extra` string that is appended verbatim, for the cases a template
does not model.

## Overriding a template

Copy the built-in template into your own tree and edit it; Rice prefers the user
copy for that one file and keeps using the built-ins for everything else.

```bash
mkdir -p ~/.config/rice/templates/waybar
rice render -c waybar -o /tmp/waybar-preview
```

## Development

```bash
go test ./...          # unit, validation and golden tests
go test . -update      # accept intentional template changes
```

Golden files in `testdata/golden/` hold the full generated output for every
bundled theme, so a template change shows up as a reviewable diff.
