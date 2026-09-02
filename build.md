# Rice — SwayFX Desktop Configuration & Theming Tool

---

# Implementation log — 2026-09-02

What was built in one session, newest last. Every item is on `main` and
pushed; `make check` passes at each commit.

| Commit | What |
| --- | --- |
| Add an interactive theme editor | `rice tui`, `internal/session`, sandbox preview, fonts, clipboard, `icons.size` |
| Edit per-program settings in the editor | per-program settings pane, `session.Draft` |
| Generate GTK and Qt configuration | `gtk` and `qt` components, session environment file |
| Check that a theme's assets exist | `internal/doctor`, folded into `rice status` |
| Pick installed themes instead of typing their names | `internal/assets`, pickers for icon/cursor/GTK/Kvantum themes |
| Complete theme, component and generation names | dynamic shell completion, CI workflow |
| Do not mistake an empty configuration for an absent one | correctness fix in `SetBase` |
| Add a live preview that never commits | `rice preview` / `commit` / `cancel` / `status` |
| Derive a theme from an image | `rice theme from-image`, `internal/palette`, `theme.Encode` |
| Apply a derived theme and its wallpaper together | `--apply` and `--wallpaper`, first `internal/cli` tests |
| Add an implementation log to build.md | this section |
| Mark absent themes and view generated output | "not installed" marker, `v` to view generated config |
| Show what applying would change | `rice diff`, `internal/diff`, verified against real `patch` |
| Show the draft's diff from inside the editor | `d` in the editor |
| Give a derived theme everything an image cannot say | inheritance for `from-image`, no empty template values |
| Document adding a program | `docs/adding-a-program.md` |
| Size the editor's value column to its content | layout fix |
| Restructure the editor around global and apps | Terminal moved under foot, SwayFX section, `Store` |

## The one design decision worth knowing about

The editor holds a theme in **source form**, not resolved form — see
section 28. A theme file's unset field means "derive this for me", and keeping
that distinction is what lets an edit to `colors.background` still move
everything derived from it. It cost `theme.ParseSource`, `Store.LoadSource`,
`Theme.Resolved`, `omitempty` on the whole theme model and eventually
`theme.Encode`, and it paid for itself twice: once in the editor, once in
`from-image`, which can leave two thirds of a theme unset because normalization
will fill it in.

## The editor's shape

```text
theme  →  global  →  app by app
```

**Global** is what the whole desktop shares: Colors, Fonts, SwayFX, Icons &
Cursor. **Apps** sit flat below it; grouping them by category comes when there
are enough to need it.

The rule that decides where a setting goes is *who reads it*, not what kind of
value it is. The sixteen ANSI colours are appearance and live in the theme
file, but only a terminal reads them, so they are edited under the terminal.
The compositor is the desktop rather than an application on it, so all of Sway
is in the global SwayFX section and the app list carries only its preview.

That split forced `session.Field.Store`, which says which file a field saves
to. Dirty tracking follows it rather than which table a field was declared in;
without that, a palette edit made under the terminal would not mark the theme
dirty and would be dropped on save. There is a test for exactly that.

`StoreTheme` is the zero value, so the configuration constructors set
`StoreConfig` explicitly rather than a later pass trying to infer it — a field
saved to the wrong file is silently lost rather than loudly wrong.

## Try it in this order

```bash
make build
./build/rice                                    # the editor
./build/rice status                             # what is missing on this machine
./build/rice preview tokyo-night                # live, uncommitted
./build/rice theme from-image ~/Pictures/x.jpg  # a palette from a wallpaper
```

## Two things that need you

* **`rice status` will warn about `QT_QPA_PLATFORMTHEME`** until you log out and
  back in. Rice writes `environment.d/50-rice.conf`, which the systemd user
  manager reads at login; there is no way to set it from inside a Sway config.
* **`icons.size` does not reach GTK or Qt.** Neither toolkit has a global
  icon-size setting, so there is nothing to write. It reaches Rofi and stops
  there. This was asked for and cannot be delivered; it is documented where it
  applies rather than promised everywhere.

## Also built, beyond the plan

`rice diff` — what applying would actually change, as a unified diff. The
documentation previously told you to render into a temporary directory and run
`diff -r` yourself, which is a workaround, not an answer. `internal/diff` is a
small LCS diff; its output is tested by handing it to real `patch` and checking
the result matches byte for byte.

One detail that decides whether the command is useful at all: the new side is
rendered with the **base generation's** number, not the next one. Every
generated file carries its generation number in a header comment, so numbering
the two sides differently reports a change in every file and buries the ones
that matter. The number always changes; that is not news.

## Found by running it end to end

Two defects the unit tests could not see, both caught by walking a full user
journey — init, apply, derive, preview, commit, roll back, check status:

**A derived theme had a palette and nothing else.** An image cannot say what
font to use, so `from-image` produced a theme naming no font, no icon theme and
no cursor theme. It now inherits all of that from an existing theme — by
default the configured one — so the result is that theme wearing the image's
colours. The widget theme is dropped when the variant flips, because a dark GTK
theme on a light palette is worse than none.

**An empty value is not an absent one.** A theme naming no icon set wrote
`gtk-icon-theme-name=` and `XCURSOR_THEME=` with nothing after them, and GTK
and the systemd user manager both take that literally. The templates now omit
the key instead. The golden files moved by one blank line, which is how little
this affects a theme that does name them — and exactly why nothing caught it.

## What is left

* Per-program **list** editing — outputs, workspaces, bindings and window rules
  are still file-only. The scalar settings are done.
* `rice run` desktop utilities (section 47) — **left unstarted deliberately**:
  it conflicts with the Independence invariant, and which way to resolve that
  is a decision for you. See the end of section 35.
* Screenshots for the README.
* A GUI, which stays post-v1.0 (section 38).

---

## 1. Project Overview

**Rice** is a lightweight configuration, theming, and desktop-utility manager for a minimal Wayland environment built around:

* SwayFX
* Waybar
* Rofi
* Foot
* Dunst
* swaylock / swayidle
* GTK
* Qt / Kvantum

Rice provides a coherent desktop experience without introducing a desktop shell, desktop environment, or mandatory runtime daemon.

The project manages:

1. complete application configuration files;
2. consistent themes;
3. transactional configuration generations;
4. safe preview and rollback;
5. installation/adoption of existing configurations;
6. small desktop helper commands and scripts;
7. optionally, a graphical theme editor.

The central principle is:

> Rice owns complete configuration files through explicit symlinks while keeping source configuration, generated output, and application-facing configuration clearly separated.

---

# 2. Goals

## 2.1 Primary Goals

* Consistent theming across the complete SwayFX desktop.
* Complete, understandable configuration files.
* One-command theme switching.
* Safe live preview.
* Atomic configuration application.
* Reliable rollback.
* Explicit ownership of managed files.
* Preservation of existing user configuration during setup.
* Clean uninstall restoring previous configuration.
* Minimal runtime overhead.
* Easy Git/dotfiles usage.
* Reusable daily desktop utilities.
* Single-binary distribution where practical.

## 2.2 Secondary Goals

* Custom theme creation.
* Wallpaper-derived themes.
* Light/dark variants.
* Environment diagnostics.
* Easy extension to additional applications.
* Interactive TUI theme editor.
* Optional GUI editor, after v1.0.
* AUR/package distribution.
* Portable setup across Arch Linux machines.

## 2.3 Non-Goals

Rice should not become:

* a Linux distribution;
* a desktop environment;
* a Wayland shell;
* a compositor;
* an application launcher;
* a notification daemon;
* a replacement for Waybar;
* a replacement for Rofi;
* a package manager;
* a general-purpose system configuration manager;
* a mandatory background daemon.

Rice should orchestrate existing programs rather than replace them.

---

# 3. Target Stack

| Function                   | Component                            |
| -------------------------- | ------------------------------------ |
| Compositor                 | SwayFX                               |
| Bar                        | Waybar                               |
| Launcher / menus           | Rofi                                 |
| Terminal                   | Foot                                 |
| Notifications              | Dunst                                |
| Lock screen                | swaylock                             |
| Idle management            | swayidle                             |
| Wallpaper                  | swaybg                               |
| GTK configuration          | GTK settings / nwg-look              |
| Qt configuration           | qt5ct / qt6ct                        |
| Qt theme                   | Kvantum                              |
| Audio                      | PipeWire / WirePlumber / `wpctl`     |
| Brightness                 | `brightnessctl`                      |
| Clipboard                  | `wl-clipboard` + optional `cliphist` |
| Screenshots                | `grim` + `slurp`                     |
| Screen recording           | `wf-recorder`                        |
| Dynamic palette generation | Optional Matugen                     |

---

# 4. Implementation Technology

## 4.1 Language

Rice will be implemented in:

```text
Go
```

Go is a good fit because Rice is primarily concerned with:

* filesystem operations;
* symlinks;
* atomic renames;
* process execution;
* configuration parsing;
* template rendering;
* file watching;
* IPC wrappers;
* CLI orchestration;
* small desktop utilities.

The project does not require low-level memory management or a large application runtime.

---

## 4.2 Initial Technology Choices

Recommended initial stack:

```text
Language:            Go

CLI:                 Cobra

Configuration:       TOML

TOML library:        BurntSushi/toml or pelletier/go-toml

Templates:           text/template

Filesystem:          Go standard library

Process execution:   os/exec

File watching:       fsnotify

Logging:             log/slog

Tests:               testing package

Snapshot tests:      golden files

TUI:                 Bubble Tea (bubbletea, bubbles, lipgloss)

Font enumeration:    fc-list (no cgo)

Clipboard:           wl-copy, with xclip/xsel fallback

GUI:                 deferred until after v1.0
```

As built, the three Charm modules are the only new direct dependencies.
`bubbles/textinput` pulls `atotto/clipboard` in transitively; it is unused by
Rice, which does its own clipboard work through the `command` package, and it
adds no cgo.

Truecolor detection reads `COLORTERM` rather than importing `termenv`, so the
direct dependency list stays at three.

Dependencies should remain minimal.

Prefer the Go standard library wherever it provides sufficient functionality.

---

# 5. Core Architecture

Rice distinguishes between four kinds of state:

```text
Source configuration
        ↓
    compile
        ↓
Generated generation
        ↓
current symlink
        ↓
Application config symlinks
```

Example:

```text
~/.config/rice/
├── config.toml
├── themes/
├── templates/
├── generations/
│   ├── 000001/
│   ├── 000002/
│   └── 000003/
├── preview/
├── backups/
├── state/
└── current -> generations/000003
```

Application paths:

```text
~/.config/sway/config
    -> ~/.config/rice/current/sway/config

~/.config/waybar/config.jsonc
    -> ~/.config/rice/current/waybar/config.jsonc

~/.config/waybar/style.css
    -> ~/.config/rice/current/waybar/style.css

~/.config/rofi/config.rasi
    -> ~/.config/rice/current/rofi/config.rasi

~/.config/foot/foot.ini
    -> ~/.config/rice/current/foot/foot.ini

~/.config/dunst/dunstrc
    -> ~/.config/rice/current/dunst/dunstrc

~/.config/swaylock/config
    -> ~/.config/rice/current/swaylock/config
```

---

# 6. Ownership Model

Rice manages a configuration file only when the application's standard configuration path points into Rice-managed storage.

Example:

```text
~/.config/foot/foot.ini
    -> ~/.config/rice/current/foot/foot.ini
```

Ownership states:

```text
Missing
RegularFile
RiceManagedSymlink
ExternalSymlink
BrokenSymlink
```

Basic rule:

```text
symlink target inside ~/.config/rice/
    → Rice-managed

regular file
    → user-managed

symlink elsewhere
    → externally managed
```

Rice must not overwrite user-owned or externally managed configuration without explicit adoption.

---

# 7. Why Whole-File Management

Rice should generate complete application configuration files rather than depending on application-specific include mechanisms.

Avoid requiring:

```text
Sway     → include
Rofi     → @import
Waybar   → @import
Foot     → different mechanism
Dunst    → different mechanism
swaylock → different mechanism
```

Whole-file generation provides:

* uniform behavior;
* simpler adapters;
* fewer parser-specific assumptions;
* easier validation;
* atomic generations;
* cleaner rollback;
* predictable deployment.

Generated files should remain valid, readable, normal application configuration files.

Rice should never require application-specific Rice plugins.

---

# 8. Source vs Generated Configuration

## 8.1 Source Configuration

User-editable state:

```text
~/.config/rice/
├── config.toml
├── themes/
│   ├── catppuccin-mocha.toml
│   ├── gruvbox-dark.toml
│   └── custom.toml
│
├── overrides/
│   ├── sway.toml
│   ├── waybar.toml
│   └── ...
│
└── scripts/
```

This is what should normally be stored in Git.

---

## 8.2 Generated Configuration

Rice-generated immutable configurations:

```text
~/.config/rice/generations/000042/
├── manifest.toml
├── sway/
│   └── config
├── waybar/
│   ├── config.jsonc
│   └── style.css
├── rofi/
│   └── config.rasi
├── foot/
│   └── foot.ini
├── dunst/
│   └── dunstrc
└── swaylock/
    └── config
```

Generated generations must not be manually edited.

Any desired change should be made in the source configuration and regenerated.

---

# 9. Generation Model

Every committed configuration state receives a generation number.

Example:

```text
generations/
├── 000039/
├── 000040/
├── 000041/
└── 000042/

current -> generations/000042
```

A generation is immutable once committed.

A generation contains every Rice-managed configuration needed to reproduce that desktop state.

---

# 10. Generation Manifest

Each generation should contain metadata.

Example:

```toml
generation = 42
created_at = "2026-09-01T23:00:00+02:00"

theme = "catppuccin-mocha"

rice_version = "0.1.0"

[components]
sway = true
waybar = true
rofi = true
foot = true
dunst = true
swaylock = true
```

Potential future metadata:

```toml
source_hash = "..."
parent_generation = 41
description = "Changed accent and window radius"
```

---

# 11. Applying Configuration

Normal flow:

```text
source configuration
        ↓
parse
        ↓
validate
        ↓
render complete generation
        ↓
validate rendered configs
        ↓
write temporary generation
        ↓
atomic finalize
        ↓
switch current symlink
        ↓
reload affected programs
```

Commands:

```bash
rice apply
```

or:

```bash
rice theme apply gruvbox-dark
```

---

# 12. Atomic Switching

The active state should be selected through one symlink:

```text
current -> generations/000042
```

Changing configuration should effectively reduce to:

```text
current -> generations/000043
```

The symlink update must be atomic.

Application configuration paths remain unchanged:

```text
~/.config/sway/config
    -> ~/.config/rice/current/sway/config
```

No individual application config needs to be copied during every theme change.

---

# 13. Rollback

Rollback becomes generation switching.

Command:

```bash
rice rollback
```

Example:

```text
before:

current -> generations/000043

after:

current -> generations/000042
```

Then reload affected programs.

Potential commands:

```bash
rice generation list
rice generation current

rice rollback
rice rollback 39
```

Example output:

```text
42  current   Catppuccin Mocha
41            Catppuccin Mocha
40            Gruvbox Dark
39            Tokyo Night
```

---

# 14. Generation Retention

Do not keep unlimited generations by default.

Possible default:

```toml
[generations]
keep = 10
```

Never automatically delete:

* current generation;
* previous generation;
* generation referenced by an active preview;
* explicitly pinned generations.

Potential future command:

```bash
rice generation pin 42
```

Not required for MVP.

---

# 15. Preview Model

Rice has two different previews, and they must not be confused.

```text
rice preview        swaps `current`, changes the live desktop
sandbox preview     renders to /tmp and launches one application,
                    leaving the desktop untouched
```

This section describes the first. The sandbox preview used by the TUI is
described in section 40.

Preview should not create hundreds of committed generations.

Use:

```text
~/.config/rice/preview/
```

During preview:

```text
current -> preview
```

The preview directory may be rewritten continuously.

Before entering preview:

```text
state/preview-parent
    = generations/000042
```

---

# 16. Preview Workflow

Start:

```bash
rice preview gruvbox-dark
```

Flow:

```text
remember current generation
        ↓
render preview/
        ↓
validate
        ↓
current -> preview
        ↓
reload
```

Commit:

```bash
rice preview commit
```

Result:

```text
preview/
    ↓
generations/000043/
    ↓
current -> generations/000043
```

Cancel:

```bash
rice preview cancel
```

Result:

```text
current -> generations/000042
```

followed by reload.

---

# 17. Live Preview

CLI watch mode:

```bash
rice preview custom --live
```

Flow:

```text
theme change
    ↓
debounce
    ↓
render preview
    ↓
validate
    ↓
replace preview files
    ↓
reload affected components
```

Recommended debounce:

```text
50–200 ms
```

Do not create a committed generation for every file edit or every keystroke in
the TUI editor.

Go implementation can use:

```text
fsnotify
```

to watch theme/config changes.

---

# 18. Existing Configuration Adoption

`rice setup` must detect existing files.

Example:

```text
Existing configurations found:

Sway       ~/.config/sway/config
Waybar     ~/.config/waybar/config.jsonc
Foot       ~/.config/foot/foot.ini
Dunst      ~/.config/dunst/dunstrc
```

Existing files must not simply be overwritten.

---

# 19. Backup Model

Backups exist primarily for **adoption and uninstallation**, not normal theme switching.

When Rice adopts:

```text
~/.config/foot/foot.ini
```

the original should be moved or copied to:

```text
~/.config/rice/backups/2026-09-01T230500/foot/foot.ini
```

Then Rice installs the managed symlink.

Avoid:

```text
foot.ini.bkp
foot.ini.bkp2
foot.ini.old
foot.ini.old2
```

Backups should be structured and recorded explicitly.

---

# 20. Adoption Manifest

Store original configuration locations.

Example:

```toml
[[managed]]
component = "foot"
target = "/home/user/.config/foot/foot.ini"
backup = "backups/2026-09-01T230500/foot/foot.ini"

[[managed]]
component = "sway"
target = "/home/user/.config/sway/config"
backup = "backups/2026-09-01T230500/sway/config"
```

This enables deterministic uninstall.

---

# 21. Uninstall

Command:

```bash
rice uninstall
```

Expected behavior:

```text
validate managed links
        ↓
remove Rice symlinks
        ↓
restore original configs
        ↓
remove application integration
```

Do not delete:

* themes;
* backups;
* user scripts;
* source configuration;

unless explicitly requested.

Potential future option:

```bash
rice uninstall --purge
```

---

# 22. Conflict Detection

Before changing an application config, Rice must inspect it.

Possible states:

```text
Missing
RegularFile
RiceManagedSymlink
ExternalSymlink
BrokenSymlink
```

Example conflict:

```text
External symlink detected:

~/.config/foot/foot.ini
  → ~/dotfiles/foot/foot.ini

Rice will not replace externally managed configuration automatically.
```

This should be surfaced clearly by both `setup` and `doctor`.

---

# 23. Go Repository Structure

Recommended structure:

```text
rice/
├── go.mod
├── go.sum
├── README.md
├── PLAN.md
├── LICENSE
│
├── cmd/
│   └── rice/
│       └── main.go
│
├── internal/
│   ├── cli/
│   │   ├── root.go
│   │   ├── setup.go
│   │   ├── apply.go
│   │   ├── tui.go
│   │   ├── preview.go
│   │   ├── rollback.go
│   │   ├── doctor.go
│   │   ├── uninstall.go
│   │   ├── generation.go
│   │   ├── theme.go
│   │   └── run.go
│   │
│   ├── config/
│   │   ├── config.go
│   │   ├── load.go
│   │   └── paths.go
│   │
│   ├── theme/
│   │   ├── theme.go
│   │   ├── parse.go
│   │   ├── validate.go
│   │   └── palette.go
│   │
│   ├── render/
│   │   ├── renderer.go
│   │   └── templates.go
│   │
│   ├── generation/
│   │   ├── generation.go
│   │   ├── builder.go
│   │   ├── manifest.go
│   │   ├── store.go
│   │   ├── switch.go
│   │   └── cleanup.go
│   │
│   ├── ownership/
│   │   ├── ownership.go
│   │   ├── detect.go
│   │   ├── adopt.go
│   │   ├── backup.go
│   │   └── restore.go
│   │
│   ├── adapter/
│   │   ├── adapter.go
│   │   ├── sway/
│   │   │   └── sway.go
│   │   ├── waybar/
│   │   │   └── waybar.go
│   │   ├── rofi/
│   │   │   └── rofi.go
│   │   ├── foot/
│   │   │   └── foot.go
│   │   ├── dunst/
│   │   │   └── dunst.go
│   │   ├── swaylock/
│   │   │   └── swaylock.go
│   │   ├── gtk/
│   │   │   └── gtk.go
│   │   └── qt/
│   │       └── qt.go
│   │
│   ├── reload/
│   │   ├── reload.go
│   │   └── command.go
│   │
│   ├── doctor/
│   │   ├── doctor.go
│   │   ├── dependencies.go
│   │   └── integration.go
│   │
│   ├── runner/
│   │   ├── runner.go
│   │   ├── volume.go
│   │   ├── brightness.go
│   │   ├── screenshot.go
│   │   ├── clipboard.go
│   │   └── power.go
│   │
│   ├── preview/
│   │   ├── preview.go
│   │   └── watcher.go
│   │
│   ├── session/
│   │   ├── session.go      draft theme + config, UI-agnostic
│   │   ├── mutate.go       field-level edits and resets
│   │   ├── sandbox.go      /tmp render + application launch
│   │   └── export.go       save as user theme, copy to clipboard
│   │
│   ├── fonts/
│   │   └── fonts.go        fc-list enumeration and filtering
│   │
│   ├── clipboard/
│   │   └── clipboard.go    wl-copy with fallbacks
│   │
│   └── tui/
│       ├── app.go          root model and routing
│       ├── picker.go       theme picker
│       ├── editor.go       global theme editor
│       ├── color.go        color field and swatches
│       ├── font.go         font picker
│       └── program.go      per-program screens (later)
│
├── templates/
│   ├── sway/
│   ├── waybar/
│   ├── rofi/
│   ├── foot/
│   ├── dunst/
│   └── swaylock/
│
├── themes/
│   ├── catppuccin-mocha.toml
│   ├── gruvbox-dark.toml
│   └── tokyo-night.toml
│
├── scripts/
└── testdata/
    ├── themes/
    ├── golden/
    └── configs/
```

Most implementation packages should remain under:

```text
internal/
```

until there is a concrete reason to expose a public Go API.

---

# 24. Adapter Model

Adapters describe application-specific behavior but should not own deployment.

Possible Go interface:

```go
type Adapter interface {
 Name() string

 ConfigPaths() []ManagedPath

 Detect(ctx context.Context) (Detection, error)

 Render(
  ctx context.Context,
  cfg config.Config,
  theme theme.Theme,
 ) ([]GeneratedFile, error)

 Validate(
  ctx context.Context,
  gen generation.Generation,
 ) error

 Reload(ctx context.Context) error

 ReloadMode() ReloadMode
}
```

The exact interface may evolve during the prototype.

Deployment remains generic:

```text
render
generation creation
ownership
symlink switching
rollback
```

and should not be duplicated in every adapter.

---

# 25. Reload Capabilities

Applications differ in runtime configuration behavior.

Represent this explicitly.

Example:

```go
type ReloadMode int

const (
 ReloadNone ReloadMode = iota
 ReloadHot
 ReloadSignal
 ReloadRestart
 ReloadNewInstancesOnly
)
```

Examples:

```text
SwayFX      → IPC reload
Waybar      → CSS reload / process restart
Rofi        → NewInstancesOnly
Foot        → NewInstancesOnly for arbitrary palettes
Dunst       → Reload
swaylock    → NewInstancesOnly
```

Rice should not pretend every application supports identical reload semantics.

---

# 26. Template Rendering

Use the Go standard library:

```text
text/template
```

initially.

Example Sway template:

```text
client.focused \
    {{ .Colors.Primary }} \
    {{ .Colors.Primary }} \
    {{ .Colors.Foreground }} \
    {{ .Colors.Primary }}

gaps inner {{ .UI.GapsInner }}
gaps outer {{ .UI.GapsOuter }}

corner_radius {{ .UI.Radius }}

blur enable
blur_radius {{ .UI.BlurRadius }}
```

Advantages:

* no mandatory templating dependency;
* straightforward Go integration;
* easy custom template functions;
* templates can be embedded in the binary later.

Potential helper functions:

```go
template.FuncMap{
 "hex":       formatHex,
 "rgba":      formatRGBA,
 "multiply":  multiply,
 "percentage": percentage,
}
```

Only introduce a third-party template engine if `text/template` becomes materially limiting.

---

# 27. Embedded Defaults

Default themes and templates can eventually be embedded into the Go binary using:

```go
//go:embed
```

Example:

```go
//go:embed templates/*
var templatesFS embed.FS

//go:embed themes/*
var themesFS embed.FS
```

This enables:

```text
single rice binary
+
user override files
```

without requiring `/usr/share/rice` for basic operation.

Distribution packages may still install examples/documentation separately.

---

# 28. Theme Format

Example:

```toml
name = "My Dark"

[colors]
background = "#181825"
surface = "#1e1e2e"
surface_alt = "#313244"

foreground = "#cdd6f4"
muted = "#6c7086"

primary = "#89b4fa"
secondary = "#cba6f7"

success = "#a6e3a1"
warning = "#f9e2af"
error = "#f38ba8"

[ui]
radius = 8
border_width = 2

gaps_inner = 6
gaps_outer = 3

opacity = 0.94
blur_radius = 4

[fonts]
ui_family = "Inter"
ui_size = 10

mono_family = "JetBrainsMono Nerd Font"
mono_size = 10

[icons]
theme = "Papirus-Dark"
size = 24

[cursor]
theme = "Bibata-Modern-Ice"
size = 24
```

Application-specific values should be derived from the shared model where practical.

## Source form and resolved form

A theme file is the **source form**: an unset field means "derive this for me".
Normalization produces the **resolved form**, with every derived value filled
in, and that is what renders.

Both forms matter, and the distinction is load-bearing for the editor:

```text
theme.Store.Load        source ──normalize──► resolved   (rendering, `theme show`)
theme.Store.LoadSource  source                           (editing)
```

An editor that held the resolved form could not tell a value the author wrote
from one Rice computed, so editing a semantic color would stop reaching
everything derived from it. The editor therefore edits the source form and
normalizes a copy for display and rendering.

Every optional field is tagged `omitempty`, so writing a theme back out
preserves its holes rather than materializing them. Zero already means "unset"
throughout the format, so the tag encodes a convention that was there already.

---

# 29. Configuration Types in Go

Potential model:

```go
type Theme struct {
 Name   string
 Colors Colors
 UI     UI
 Fonts  Fonts
 Icons  Icons
 Cursor Cursor
}

type Colors struct {
 Background string
 Surface    string
 SurfaceAlt string

 Foreground string
 Muted      string

 Primary   string
 Secondary string

 Success string
 Warning string
 Error   string
}

type UI struct {
 Radius      int
 BorderWidth int

 GapsInner int
 GapsOuter int

 Opacity    float64
 BlurRadius int
}
```

Validate immediately after loading.

Do not let invalid theme values propagate into template rendering.

---

# 30. Application Config Generation

## SwayFX

Generate complete:

```text
sway/config
```

Containing:

* appearance;
* keybindings;
* inputs;
* outputs;
* application rules;
* startup commands;
* helper command bindings.

---

## Waybar

Generate:

```text
waybar/config.jsonc
waybar/style.css
```

---

## Rofi

Generate:

```text
rofi/config.rasi
```

Menus invoked by Rice utilities should automatically inherit this config.

---

## Foot

Generate:

```text
foot/foot.ini
```

Including:

* palette;
* font;
* padding;
* cursor;
* selection;
* shell settings where configured.

---

## Dunst

Generate:

```text
dunst/dunstrc
```

Including:

* layout;
* colors;
* urgency styles;
* icons;
* progress bars;
* optional application rules.

---

## swaylock

Generate:

```text
swaylock/config
```

---

# 31. GTK and Qt

GTK and Qt do not necessarily fit perfectly into the single-config-file symlink model.

Treat toolkit integration separately.

Rice may manage:

```text
GTK settings
icon theme
cursor theme
fonts
dark/light preference

qt5ct
qt6ct
Kvantum
```

Ownership rules must still remain explicit.

Do not overwrite unrelated toolkit settings.

The goal is:

> coherent desktop appearance

rather than:

> pixel-identical GTK and Qt widgets.

---

# 32. CLI

Use:

```text
Cobra
```

for the main CLI once the command tree grows beyond the prototype stage.

Proposed command structure:

```text
rice
├── tui
├── setup
├── apply
├── rollback
├── doctor
├── uninstall
│
├── theme
│   ├── list
│   ├── current
│   ├── apply
│   ├── clone
│   ├── new
│   └── edit
│
├── preview
│   ├── start
│   ├── commit
│   └── cancel
│
├── generation
│   ├── list
│   ├── current
│   └── rollback
│
└── run
    ├── volume
    ├── brightness
    ├── screenshot
    ├── record
    ├── clipboard
    ├── power
    ├── lock
    ├── output
    ├── vpn
    └── project
```

Potential shorter UX:

```bash
rice preview gruvbox-dark
rice rollback
```

should remain available even if internally implemented through subcommands.

`rice` with no arguments on an interactive terminal opens the TUI. Every
subcommand keeps working unchanged, so scripts and pipelines are unaffected.

---

# 33. Doctor

Command:

```bash
rice doctor
```

Example output:

```text
Components

✓ swayfx
✓ waybar
✓ rofi
✓ foot
✓ dunst
✓ swaylock

Ownership

✓ ~/.config/sway/config
  → Rice managed

✓ ~/.config/foot/foot.ini
  → Rice managed

! ~/.config/waybar/config.jsonc
  → external symlink

Generations

✓ current: 42
✓ previous: 41
✓ generation 42 complete

Theme

✓ Catppuccin Mocha
✓ colors valid
✓ fonts available

Utilities

✓ wpctl
✓ brightnessctl
✓ grim
✓ slurp
! wf-recorder missing

Qt

✓ qt6ct
✓ Kvantum
! QT_STYLE_OVERRIDE not configured
```

Dependency detection can initially use:

```go
exec.LookPath(...)
```

## As built

`doctor` is an alias of `status` rather than a second command. The two would
have printed the same configuration, ownership and dependency sections, and a
second command that mostly repeats the first is a maintenance cost with no
user benefit. Whichever name you reach for, you get the whole report.

`internal/assets` enumerates what is installed — icon, cursor, GTK and Kvantum
themes — by scanning the XDG data directories. It backs two things at once: the
doctor checks below, and the editor's pickers, so a theme name is chosen from
what exists rather than typed from memory.

Only real themes count. A directory in the right place is not a theme: an icon
theme carries an `index.theme`, a cursor theme a `cursors/` directory, a GTK
theme a version directory. Fonts stay in `internal/fonts`, because they need
fontconfig to resolve aliases and substitutions; a theme either has a directory
or it does not.

`internal/doctor` adds the part `status` could not answer: whether what a theme
*asks for* exists on the machine.

* fonts, resolved through `fc-list`, with fontconfig aliases such as
  `monospace` accepted rather than looked up;
* icon and cursor themes, looked for across the XDG data directories plus the
  legacy `~/.icons`;
* the GTK theme, with Adwaita and the other built-ins accepted without a
  directory — they ship inside GTK, and warning about the default would flag
  the most likely correct answer;
* the Kvantum theme, when Kvantum is turned on;
* `XCURSOR_THEME`, `XCURSOR_SIZE` and `QT_QPA_PLATFORMTHEME`, compared against
  what Rice generates.

A missing font is a warning, never an error: the desktop still works, it just
does not look like the theme. A check that could not run at all — no
fontconfig — reports unknown rather than missing, because Rice must not claim a
font is absent when it simply could not look.

This closes the loop on toolkit integration: the usual reason a correct
configuration has no effect is that the session has not restarted, and that is
now something Rice says out loud.

---

# 34. Desktop Utilities

Rice should provide small wrappers for common desktop functions.

Interface:

```bash
rice run volume up
rice run volume down
rice run volume mute

rice run brightness up
rice run brightness down

rice run screenshot region
rice run screenshot screen

rice run record region
rice run record stop

rice run clipboard

rice run power

rice run lock

rice run output laptop
rice run output external
rice run output dual

rice run vpn

rice run project
```

Sway bindings stay simple:

```text
bindsym XF86AudioRaiseVolume exec rice run volume up
bindsym XF86AudioLowerVolume exec rice run volume down

bindsym $mod+Shift+s exec rice run screenshot region
bindsym $mod+v exec rice run clipboard
bindsym $mod+p exec rice run power
```

---

# 35. Utility Philosophy

Avoid large shell pipelines directly inside generated Sway configuration.

Prefer:

```text
Sway keybinding
      ↓
rice run ...
      ↓
small implementation
```

Implementation strategy:

### Built-in Go

Use Go for generic, stable functionality where it improves:

* error handling;
* process management;
* structured output;
* integration;
* portability.

### External scripts

Allow custom scripts for workflows that users may want to modify heavily.

Preferred model:

```text
hybrid
```

Rice should support invoking user-provided overrides.

Example:

```toml
[scripts]
project = "~/.config/rice/scripts/project"
```

If configured, Rice can defer to the custom implementation.

## Unresolved: this conflicts with Independence

Section 55 says removing Rice must leave standard applications and standard
configuration formats behind. A generated Sway config whose bindings call
`rice run screenshot` does not satisfy that: uninstall restores backups, it
does not rewrite bindings, so the desktop is left calling a program that is
gone.

The two can be reconciled, but only by choosing:

1. **`rice run` stays opt-in.** The default bindings keep calling `grim`,
   `wpctl` and the rest, and `[commands]` — which already exists — is where
   someone opts in. Independence holds; the shell pipelines stay in the
   default configuration.
2. **`rice run` becomes the default and Independence weakens** to "leaves
   standard configuration formats behind", accepting that some bindings need
   Rice installed.
3. **Uninstall rewrites bindings** back to their plain equivalents. This keeps
   both promises and is the most work, and it means Rice edits a file it
   handed back, which it otherwise never does.

Option 1 is the smallest and keeps every existing promise, but it also makes
`rice run` a nicety rather than the point — which is worth knowing before
building nine utilities. **This decision is yours; phase 5 was left unstarted
because of it, not because of time.**

---

# 36. Process Execution

Centralize process execution rather than scattering `exec.Command` calls throughout adapters.

Potential package:

```text
internal/command
```

Example interface:

```go
type Runner interface {
 Run(
  ctx context.Context,
  name string,
  args ...string,
 ) error

 Output(
  ctx context.Context,
  name string,
  args ...string,
 ) ([]byte, error)
}
```

Benefits:

* testability;
* command mocking;
* logging;
* timeout handling;
* consistent errors.

---

# 37. Logging

Use:

```text
log/slog
```

for structured internal logging.

Normal CLI output should remain concise.

Potential:

```bash
rice apply --verbose
```

could expose structured execution details.

Avoid noisy logging during ordinary use.

---

# 38. Interactive Editor — TUI, Not GUI

Rice needs an interactive way to pick a theme, tweak it, look at the result and
keep it. The CLI stays: it is the right tool for pipelines, scripting and quick
one-shot changes. The interactive editor is an addition, not a replacement.

The decision is:

```text
Interactive editor:  TUI  (Bubble Tea)
GUI:                 deferred, post-v1.0, optional
```

## Why a TUI

* **Colors survive the terminal.** Truecolor ANSI renders real swatches. The
  primary editing task is color selection, and a terminal represents it
  honestly.
* **The binary stays one static file.** Templates and themes are already
  embedded. A GTK4 or Qt GUI drags in cgo and toolkit development headers and
  breaks `make release` cross-compilation. Fyne and Gio avoid cgo-heavy builds
  but produce an application that looks nothing like the desktop it configures.
* **The audience lives in a terminal.** Rice configures a minimal Wayland
  desktop. Its user has a terminal open.
* **It works when the desktop does not.** A TUI runs over SSH and from a bare
  TTY, which is exactly the situation a broken rice creates.
* **The honest preview is not a widget.** Preview means rendering real
  configuration and launching the real application against it. That mechanism
  is identical under a TUI and a GUI, so a GUI would buy a worse, simulated
  preview rather than a better one.

## Where a TUI is genuinely weaker

Font selection. A terminal cannot render an arbitrary system font family, and
a machine typically has thousands installed.

Mitigations:

* enumerate families through `fc-list`, deduplicated and sorted;
* filter monospaced candidates through fontconfig spacing;
* fuzzy filtering, with likely families surfaced first;
* treat the sandbox preview as the font preview — launching Foot against the
  generated `foot.ini` shows the real mono font at the real size, and Waybar or
  Rofi shows the real UI font.

Icon size has the same property: only a real toolkit render tells the truth.

## The GUI is not cancelled

The GUI stays a possible later frontend, not a rewrite, provided the editing
logic never lives inside the TUI. See section 39.

If a GUI is eventually built, the earlier evaluation still holds: a Go backend
with an embedded HTML/CSS/JS frontend served on localhost, assets carried in
`embed.FS`, remains the most plausible option, because the task is visual
editing and CSS is good at it. That decision stays deferred until after v1.0.

---

# 39. Session Layer

The interactive editor must not own state.

```text
             internal/session
          (draft state + operations)
             /              \
     internal/tui        future GUI
```

`internal/session` is UI-agnostic and holds:

* the draft theme, starting from a chosen base theme;
* the draft configuration;
* per-field overrides and which fields are dirty;
* validation of the draft;
* sandbox rendering and preview launching;
* export to clipboard;
* saving the draft as a user theme.

Rules:

* the TUI holds no configuration state of its own — only cursor position,
  focus, filter text and other view concerns;
* every mutation is a `session` method, so it is testable without a terminal;
* `session` reuses the existing `config`, `theme`, `render`, `adapter` and
  `generation` packages without duplicating any of their logic;
* `session` never writes into `~/.config` and never switches `current`; saving
  and applying remain explicit user actions routed through the existing
  generation machinery.

Sketch:

```go
type Session struct {
    Base     theme.Theme
    Draft    theme.Theme
    Config   config.Config
    Dirty    map[string]bool
}

func (s *Session) SetColor(field string, c theme.Color) error
func (s *Session) SetFont(role FontRole, family string, size int) error
func (s *Session) Reset(field string) 
func (s *Session) Validate() []error
func (s *Session) RenderSandbox(component string) (string, error)
func (s *Session) Preview(component string) (*exec.Cmd, error)
func (s *Session) Copy(component string) error
func (s *Session) SaveTheme(name string) (string, error)
```

Saving writes a normal theme file:

```text
~/.config/rice/themes/<name>.toml
```

This reuses the existing rule that a user theme shadows a bundled theme of the
same name. No new persistence mechanism is introduced.

---

# 40. TUI Design

Entry point:

```bash
rice tui
```

`rice` with no arguments and an interactive terminal may also open the TUI,
while every existing subcommand keeps its current behavior.

## Screen flow

```text
Theme picker
     │
     ▼
Global theme editor  ──────►  Sandbox preview
     │                            (launch real app)
     ├──► Colors
     ├──► Fonts
     ├──► Sizing / geometry
     ├──► Icons and cursor
     │
     ▼
Per-program editors            (later; same session layer)
     │
     ▼
Save theme  /  Copy config  /  Apply
```

## Theme picker

Lists bundled and user themes through the existing theme store, marking the
active one. Each row previews its palette as colored blocks. Selecting a theme
seeds the draft.

## Global theme editor

Groups, matching the existing theme model:

```text
Colors      background, surface, surface_alt, overlay,
            foreground, muted,
            primary, secondary, accent,
            success, warning, error,
            border, border_focus

Terminal    16 ANSI colors, cursor, selection, url

Fonts       ui_family / ui_size
            mono_family / mono_size
            bar_family / bar_size

Sizing      radius, border_width, gaps_inner, gaps_outer,
            padding, horizontal_padding,
            opacity, blur_radius, blur_passes, dim_inactive

Icons       icon theme, icon size
Cursor      cursor theme, cursor size
```

Each row shows the value, a swatch where it is a color, and whether it is
inherited from the base theme or overridden.

`icon size` does not exist in the theme model yet and must be added as
`icons.size`, then consumed by the Rofi template and by the GTK/Qt integration
in section 31.

## Color editing

* hex entry with live validation through the existing `theme.Color` parser;
* a swatch beside every field, plus a foreground-on-background contrast sample
  for pairs that matter;
* nudge keys for lightness and saturation, so a palette can be adjusted without
  computing hex by hand;
* reset-to-base on any field.

## Font selection

Source:

```bash
fc-list : family          # all families
fc-list :spacing=100 family   # monospaced candidates
```

Enumerated once per session, deduplicated, cached. Monospaced families are
offered first for the mono and bar roles. Fuzzy filter over the list. Size is a
numeric field. The rendered result is confirmed through the sandbox preview,
not through the picker.

Fontconfig is queried through `fc-list` rather than linked, keeping the binary
free of cgo.

## Sandbox preview

This is distinct from `rice preview` in sections 15–17. That command swaps the
`current` symlink and changes the live desktop. Sandbox preview does not touch
the desktop at all.

```text
draft theme
     ↓
render into /tmp/rice-preview/<pid>/
     ↓
launch the application against those files
     ↓
user closes it; directory is removed
```

Per component:

| Component | Command | Notes |
| --- | --- | --- |
| Foot | `foot -c <dir>/foot/foot.ini` | Clean. |
| Rofi | `rofi -config <dir>/rofi/config.rasi -show drun` | Clean. |
| Waybar | `waybar -c <dir>/waybar/config.jsonc -s <dir>/waybar/style.css` | Second instance overlaps the running bar; force a distinct position or accept the overlap for the preview's lifetime. |
| Sway | nested `sway -c <dir>/sway/config` | Runs as a window inside the running session. |
| Dunst | `dunst -config <dir>/dunst/dunstrc` | Conflicts with the running instance over the D-Bus name. Needs a replace-and-restore dance, or is excluded initially. |
| swaylock | `swaylock -C <dir>/swaylock/config` | Actually locks the screen. Must be opt-in behind an explicit confirmation, and is excluded by default. |

Rules:

* preview never runs unless the component is enabled and its binary exists;
* the sandbox directory is created under `/tmp`, owned by the user, and removed
  when the preview process exits;
* a failed render is reported in the TUI and launches nothing.

## Clipboard export

For users who want Rice as a config generator only, not as a ricing system:

```text
Copy <component> config  →  wl-copy
```

Falls back to `xclip`/`xsel` if `wl-copy` is missing, and to writing the file
path if no clipboard tool exists. The same content is available non-interactively
through the existing `rice render -c <component>`.

## Finishing

From the editor:

```text
Save theme        write ~/.config/rice/themes/<name>.toml
Apply             save, then run the normal apply path
Discard           drop the draft
```

Apply reuses `runApply` unchanged. The TUI does not gain a private route to
`current`.

## Implementation notes

Dependencies added:

```text
charmbracelet/bubbletea
charmbracelet/bubbles
charmbracelet/lipgloss
```

No cgo. This is the first meaningful dependency growth in the module and should
stay at these three.

Degrade honestly: if the terminal reports no truecolor support, say so rather
than rendering misleading swatches.

---

# 41. Dynamic Themes

Optional backend:

```text
Wallpaper
    ↓
Matugen
    ↓
normalized Rice palette
    ↓
normal Rice generation
```

Command:

```bash
rice theme from-image ~/Pictures/wallpaper.jpg
```

Matugen must not become the internal Rice theme representation.

Rice owns the normalized theme model.

## As built: no Matugen

The quantizer is plain Go in `internal/palette`, so this works wherever Rice
does and adds no dependency, external tool or cgo. Matugen would have been an
install step and a second opinion about what a theme is.

The derivation emits a theme in **source form**, with everything derivable left
unset. An image can say what the background and the accents are; it cannot say
what `surface_alt` should be relative to `surface`, or how the sixteen ANSI
slots relate to the palette. Those are better computed than counted out of
pixels, and the source-form work from phase 4 is what makes leaving them out
possible.

Three properties are enforced rather than hoped for:

* **Contrast.** The foreground is walked until it clears a 7:1 ratio against
  the background, measured with the WCAG relative luminance rather than the
  perceived brightness `theme.Color` reports.
* **Meaning.** `success`, `warning` and `error` keep fixed hues and borrow only
  the palette's saturation and lightness.
* **Determinism.** Sampling walks a fixed stride and k-means is seeded from the
  samples sorted by lightness, so the same image always gives the same theme.
  Hue is averaged as a vector, because the mean of 350° and 10° is 0°.

This also forced `theme.Encode`. go-toml cannot omit a fixed-size array, so an
untouched ANSI palette was written as sixteen `#00000000` entries — which
round-trips correctly but reads as broken and invites someone to "fix" it.
`Encode` drops fully-unset arrays and the tables left empty by doing so, and is
now what every path that writes a theme file uses.

---

# 42. Testing Strategy

Go's built-in testing ecosystem should be sufficient for most of the project.

Use:

```text
testing
```

as the default framework.

---

## 42.1 Unit Tests

Test:

* TOML parsing;
* theme validation;
* color validation;
* path handling;
* ownership detection;
* generation metadata;
* reload selection;
* command construction.

---

## 42.2 Table-Driven Tests

Prefer idiomatic Go table-driven tests.

Example:

```go
tests := []struct {
 name     string
 pathType PathType
 want     Ownership
}{
 {
  name:     "regular config",
  pathType: RegularFile,
  want:     UserOwned,
 },
 {
  name:     "rice symlink",
  pathType: RiceSymlink,
  want:     RiceManaged,
 },
}
```

---

## 42.3 Golden Tests

Generated config files are especially suitable for golden testing.

Example:

```text
testdata/
└── golden/
    ├── catppuccin/
    │   ├── sway.config
    │   ├── waybar.css
    │   ├── rofi.rasi
    │   ├── foot.ini
    │   └── dunstrc
```

Test flow:

```text
theme
   ↓
renderer
   ↓
generated config
   ↓
compare with golden file
```

---

## 42.4 Filesystem Tests

Use temporary directories:

```go
t.TempDir()
```

to test:

* backups;
* adoption;
* symlink creation;
* current switching;
* rollback;
* uninstall;
* cleanup.

Avoid tests touching the real user's `$HOME`.

---

## 42.5 Command Tests

Use an injected command runner rather than invoking real desktop tools in unit tests.

Example:

```go
type FakeRunner struct {
 Commands []Command
}
```

Then verify:

```text
swaymsg reload
dunstctl reload
```

were requested correctly.

---

## 42.6 Integration Tests

Optional integration tests may use installed applications to validate generated configuration where practical.

Keep them separate from normal unit tests.

Potential build tag:

```text
integration
```

---

# 43. Phase 0 — Ownership Prototype

**Target: 1–2 engineering days**

* [ ] Initialize Go module.
* [ ] Create `cmd/rice`.
* [ ] Establish runtime paths.
* [ ] Detect config ownership.
* [ ] Back up existing test config.
* [ ] Install Rice-managed symlink.
* [ ] Generate one generation.
* [ ] Switch `current`.
* [ ] Restore original config.
* [ ] Add filesystem tests using `t.TempDir()`.

Acceptance:

```text
regular config
    ↓
rice setup
    ↓
backup preserved
    ↓
config path becomes Rice symlink
    ↓
rice uninstall
    ↓
original config restored
```

Do this before implementing the full theming engine.

---

# 44. Phase 1 — Theme and Configuration Generation

**Target: 3–5 engineering days**

* [ ] Define Go theme structs.
* [ ] Parse TOML.
* [ ] Validate colors.
* [ ] Implement `text/template` renderer.
* [ ] Implement generation builder.
* [ ] Implement generation manifest.
* [ ] Add golden tests.

Adapters:

* [ ] SwayFX
* [ ] Waybar
* [ ] Rofi
* [ ] Foot
* [ ] Dunst
* [ ] swaylock

Acceptance:

```bash
rice apply
```

creates a complete valid generation.

---

# 45. Phase 2 — Deployment and Rollback

**Target: 2–3 engineering days**

* [ ] `current` symlink management.
* [ ] Atomic switch.
* [ ] Reload manager.
* [ ] Generation listing.
* [ ] Previous generation tracking.
* [ ] Rollback.
* [ ] Generation retention.
* [ ] Failure tests.

Acceptance:

```bash
rice apply
rice rollback
```

must reliably alternate between complete desktop states.

---

# 46. Phase 3 — Preview

* [x] Preview directory.
* [x] Preview parent tracking.
* [x] Preview activation.
* [x] Preview update.
* [x] Preview commit.
* [x] Preview cancel.
* [x] Crash recovery.
* [ ] `fsnotify` live watch mode.

Acceptance holds:

```bash
rice preview gruvbox-dark
rice preview cancel
```

returns the exact previous generation, and leaves `previous` — where rollback
goes — untouched.

## As built

**Committing rebuilds rather than promotes.** The preview directory is thrown
away and an ordinary generation is built from the same theme. Rendering is
deterministic, so the content matches — except the generation number stamped
into every generated file, which a real build gets right and a promoted preview
would have had to guess or rewrite.

**A preview is not in the rollback chain.** Starting one does not record a
previous generation, so `rice rollback` still means "the last generation I
committed". `state/preview` holds the theme and the parent instead.

**Conflicting operations are refused, not resolved.** `rice apply` and `rice
rollback` under a running preview would move `current` away and leave the
preview state describing something untrue, so they stop and name the two ways
out. Pruning never removes the generation a preview would cancel back to.

**A failed re-render leaves the running preview alone.** The render goes to a
staging directory and is swapped in, the same way a generation is built.

**Crash recovery is the absence of state, not a repair pass.** A preview whose
state file is missing still cancels; it simply has no parent to return to.

The `fsnotify` watch mode is still open, and is now less obviously worth
building: the interactive editor covers iterating on a theme, and does it
without touching the live desktop.

---

# 47. Phase 4 — TUI

Split so the session layer lands and is tested before any terminal code exists.
Everything but 4e is done; the acceptance run below passes.

## 4a — Session layer

* [x] `internal/session` draft state over a base theme.
* [x] Field-level mutation, dirty tracking and reset-to-base.
* [x] Draft validation reusing `theme.Validate`.
* [x] Save draft as a user theme.
* [x] Unit tests, no terminal involved.

The draft is held in **source form**, not resolved form — see section 28. That
was not in the original plan and is the one design change worth knowing about:
without it, editing `colors.background` could not move the values derived from
it, and the editor could not tell an authored value from a computed one.

Consequences, all shipped:

* `theme.ParseSource`, `theme.Store.LoadSource` and `Theme.Resolved()`.
* `omitempty` on every optional theme field, so a saved theme keeps its holes.
* `Session.Explicit` and `Session.Clear`, exposed in the editor as the
  "derived" marker and the `c` key.

## 4b — Sandbox preview

* [x] Render the draft into a private directory under the temporary directory.
* [x] Launch table per component, guarded by binary presence.
* [x] Cleanup on process exit, including the crash path.
* [x] Foot, Rofi, Waybar and nested Sway first.
* [x] Dunst refused with a reason; swaylock behind a confirmation.

Two details differ from the sketch:

* The sandbox is `os.MkdirTemp` under `/tmp/rice-preview`, mode 0700, not a
  predictable `<pid>` directory. The root is world-writable, so a predictable
  name is a symlink-attack surface for no benefit.
* `command.Runner` gained `Pipe` (stdin, for the clipboard) and `Start` (a
  long-lived process with no timeout, for a preview). A previewed application
  outlives `DefaultTimeout`, so it cannot go through `Run`.

## 4c — Supporting packages

* [x] `internal/fonts` — `fc-list` enumeration, dedupe, mono filter, cache.
* [x] `internal/clipboard` — `wl-copy` with `xclip`/`xsel` fallback.
* [x] `icons.size` added to the theme model and consumed by Rofi.

Enumeration uses `fc-list --format=%{family[0]}\n`, and `:spacing=100` for the
monospace set. GTK/Qt consumption of `icons.size` waits for phase 6.

## 4d — Terminal interface

* [x] `rice tui`, plus bare `rice` on an interactive terminal.
* [x] Theme picker with palette swatches.
* [x] Global editor: colors, terminal palette, fonts, sizing, icons, cursor.
* [x] Color fields with swatches, contrast samples and lightness nudges.
* [x] Font picker with filtering and monospace-first ordering.
* [x] Preview and copy actions per component.
* [x] Save, apply, discard.
* [x] Truecolor detection with an honest message when unsupported.

The editor styles itself from the draft, so the palette being edited is visible
in the interface drawing it. Applying is injected from the command layer as a
function, so `internal/tui` never reaches generations, deployment or reload.

## 4e — Per-program editors

* [x] Program list derived from the adapter registry.
* [x] Preview and copy from inside a program screen.
* [x] Per-program settings on the same session layer.

**Decision: per-program settings stay in `config.toml`.** They are structure,
not appearance — a bar's height and a launcher's width are decisions about the
desktop, and they should survive a change of palette. The alternative, a
per-program block inside the theme, would have made every theme carry someone
else's layout.

That means a draft is two things, so `session.Draft` now holds both:

```go
type Draft struct {
    Theme  theme.Theme
    Config config.Config
}
```

Fields address a `Draft` rather than a theme, so a palette color and a bar
height are the same kind of thing to an interface, while saving still sends
appearance to a theme file and structure to `config.toml`. Writing
`config.toml` is injected from the command layer, which owns its formatting,
and only happens when a program setting actually changed.

The tables are curated, not exhaustive: scalars only. Outputs, workspaces,
bindings, window rules and Waybar's module map are lists, and editing a list
well needs an interface of its own. Until that exists they stay in the file,
where they are already comfortable to edit by hand — that is the next piece of
editor work, if it turns out to be wanted.

Acceptance:

```bash
rice tui
```

picks a theme, changes the background color and the mono font, previews Foot
against the draft, copies the Rofi config to the clipboard, saves the result as
a user theme, and leaves `~/.config` and `current` untouched until apply is
chosen explicitly.

---

# 48. Phase 5 — Desktop Utilities

**Target: 2–4 engineering days**

* [ ] volume
* [ ] brightness
* [ ] screenshot
* [ ] clipboard
* [ ] power
* [ ] lock
* [ ] recording
* [ ] output profiles
* [ ] project launcher
* [ ] optional VPN

Prefer Go implementations for generic utility wrappers.

Allow user scripts where customization is desirable.

---

# 49. Phase 6 — GTK / Qt

* [x] GTK3
* [x] GTK4
* [x] icon settings
* [x] cursor settings
* [x] fonts
* [x] qt5ct
* [x] qt6ct
* [x] Kvantum
* [x] session environment
* [ ] doctor integration — waits on `rice doctor` itself

**Icon size does not reach GTK or Qt.** Neither toolkit has a global
icon-size setting, so there is nothing to write: `icons.size` reaches Rofi and
stops there. This was asked for and cannot be delivered, so it is documented
where it applies rather than promised everywhere.

Three things this phase forced:

**The adapter file set is no longer always fixed.** Whether a Qt 5 platform
theme or a GTK stylesheet should exist is a decision, not a constant. Rather
than push a configuration argument through every adapter, the few that need one
implement an optional extension:

```go
type Configurable interface {
	FilesFor(cfg config.Config) []File
	ConfigPathsFor(cfg config.Config) []ManagedPath
}
```

`adapter.FilesOf` and `adapter.ConfigPathsOf` use it when present. Existing
adapters were untouched; `ownership.BuildPlan` gained a config argument.

**One rendered file can deploy to several places.** GTK 3 and GTK 4 read the
same keys from separate directories, so `gtk/settings.ini` links into both.
Rendering it twice would let them drift.

**The session needs an environment file.** Sway's configuration language cannot
export a variable, and a generated `qt5ct.conf` that nothing points
`QT_QPA_PLATFORMTHEME` at is inert. The Sway adapter therefore writes
`environment.d/50-rice.conf` with the cursor and the platform theme. It is read
by the systemd user manager at login, so it needs a fresh session rather than a
reload — reported honestly as new-instances-only rather than claiming a reload
Rice cannot perform.

The palette-to-libadwaita stylesheet (`gtk.css`) is **off by default**. It is
the one part of toolkit integration that changes how applications look rather
than only which theme they load, and a GTK application fighting its own theme
looks worse than one that ignores Rice.

---

# 50. Phase 7 — Public Release

* [x] README
* [ ] screenshots
* [x] architecture documentation
* [x] setup documentation
* [x] uninstall documentation
* [x] bundled themes
* [x] shell completions
* [x] CI
* [ ] release binaries
* [ ] AUR package
* [ ] checksums
* [ ] tagged releases

Completion is not only Cobra's generated script: it knows what Rice knows.
Theme names come from the store, component names from the enabled set — never
offering one that would error a moment later — and generation numbers come from
disk, newest first, annotated with the theme each was built from. A completion
function runs in its own process before the usual setup, so each resolves the
application itself rather than relying on the shared instance.

## Beyond v1.0

A GUI, if it happens at all, is a second frontend over `internal/session` and
is scheduled only after v1.0 ships. See section 38 for why the TUI comes first
and what a GUI would have to reuse.

---

# 51. Distribution

Go simplifies distribution significantly.

Primary output:

```text
rice
```

single executable.

Potential releases:

```text
rice-linux-amd64
rice-linux-arm64
```

Build:

```bash
go build ./cmd/rice
```

Release build:

```bash
CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o rice \
    ./cmd/rice
```

Releases stay pure Go. The TUI adds no cgo, and fontconfig is queried through
`fc-list` rather than linked, so `CGO_ENABLED=0` remains correct. Only a future
GUI toolkit would challenge this, and that decision is post-v1.0.

---

# 52. Arch Packaging

Eventually provide:

```text
AUR
```

packages such as:

```text
rice
rice-bin
```

Potentially:

```text
rice-git
```

if useful.

The AUR package should primarily install the Rice binary.

Bundled templates and themes may either:

1. be embedded in the binary; or
2. be installed under `/usr/share/rice`.

Embedding defaults is preferred for simplicity.

---

# 53. MVP Definition

MVP requires:

* [ ] Go CLI
* [ ] ownership detection
* [ ] safe config adoption
* [ ] backups
* [ ] symlink deployment
* [ ] complete generated configurations
* [ ] SwayFX
* [ ] Waybar
* [ ] Rofi
* [ ] Foot
* [ ] Dunst
* [ ] swaylock
* [ ] immutable generations
* [ ] atomic `current` switching
* [ ] reload
* [ ] rollback
* [ ] preview
* [ ] clean uninstall
* [ ] basic tests

Primary workflow:

```bash
rice setup

rice theme apply catppuccin-mocha

rice preview gruvbox-dark

rice preview commit
```

or:

```bash
rice preview cancel
```

---

# 54. v1.0 Definition

Rice v1.0 should provide:

* [ ] stable source configuration format
* [ ] stable theme format
* [ ] explicit ownership model
* [ ] safe adoption
* [ ] deterministic uninstall
* [ ] generations
* [ ] atomic switching
* [ ] rollback
* [ ] preview
* [ ] SwayFX
* [ ] Waybar
* [ ] Rofi
* [ ] Foot
* [ ] Dunst
* [ ] swaylock
* [ ] GTK integration
* [ ] Qt/Kvantum integration
* [ ] essential helper utilities
* [ ] interactive TUI theme editor
* [ ] sandbox preview and clipboard export
* [ ] `rice doctor`
* [ ] bundled themes
* [ ] automated tests
* [ ] documentation
* [ ] Arch installation instructions
* [ ] release binary

The GUI does not need to block v1.0. The TUI does: it is the interactive
editor, and without it Rice is a generator with no way to explore a theme.

---

# 55. Critical Invariants

The implementation must preserve these invariants.

## Ownership

> Rice never modifies a user-owned config unless the user explicitly adopts it.

## Recoverability

> Adopting Rice must be reversible.

## Atomicity

> A desktop should never intentionally enter a half-old, half-new configuration state.

## Immutability

> Committed generations are never modified.

## Source of Truth

> Generated files are derived output, not user-editable state.

## Independence

> Removing Rice should leave standard applications and standard configuration formats.

## Runtime Simplicity

> Normal Rice usage must not require a permanently running Rice daemon.

## Implementation Simplicity

> Prefer standard Go facilities and straightforward orchestration over framework-heavy abstractions.

---

# 56. Major Risk — Scope Creep

The main architectural danger is:

```text
config generator
    ↓
theme manager
    ↓
launcher framework
    ↓
shell
    ↓
desktop environment
```

Avoid this.

A feature should generally belong in Rice only if it concerns:

* configuration;
* theming;
* deployment;
* configuration state;
* desktop integration;
* small workflow wrappers.

Do not replace established Wayland applications.

---

# 57. Additional Risk — Premature Framework Design

Avoid creating excessive Go abstractions too early.

Do not begin with:

```text
25 interfaces
complex dependency injection
plugin architecture
generic event bus
service registry
```

The initial codebase should remain concrete.

Introduce interfaces primarily where they provide:

* adapter boundaries;
* command mocking;
* filesystem testing;
* meaningful decoupling.

Keep the implementation boring.

---

# 58. First Sprint

## Goal

Prove the ownership + generation architecture in Go before building the full product.

### Project Setup

* [ ] `go mod init`
* [ ] create `cmd/rice/main.go`
* [ ] add basic CLI command dispatcher
* [ ] establish `internal/` packages

### Ownership

* [ ] determine Rice root
* [ ] inspect config path
* [ ] identify ownership type
* [ ] back up regular files
* [ ] reject external symlinks by default
* [ ] install Rice symlink
* [ ] restore original file

### Generations

* [ ] create generation directory
* [ ] generate manifest
* [ ] finalize generation
* [ ] atomically update `current`
* [ ] determine current generation
* [ ] determine previous generation

### First Adapter

Use Foot initially.

* [ ] define simple theme struct
* [ ] create Foot template
* [ ] generate `foot.ini`
* [ ] create generation 1
* [ ] create generation 2
* [ ] switch between them

### Tests

* [ ] ownership tests
* [ ] symlink tests
* [ ] backup tests
* [ ] generation switching tests
* [ ] golden Foot config test

---

# 59. First Sprint Acceptance Test

Starting state:

```text
~/.config/foot/foot.ini
    = user's existing configuration
```

Run:

```bash
rice setup
```

Expected:

```text
original config backed up

~/.config/foot/foot.ini
    -> ~/.config/rice/current/foot/foot.ini

current
    -> generations/000001
```

Run:

```bash
rice apply
```

Expected:

```text
generations/000002 created

current
    -> generations/000002
```

Run:

```bash
rice rollback
```

Expected:

```text
current
    -> generations/000001
```

Run:

```bash
rice uninstall
```

Expected:

```text
Rice symlink removed

original ~/.config/foot/foot.ini restored
```

No original user configuration may be lost.

---

# 60. Second Sprint

Once the generation model is proven, add the full desktop stack.

Order:

```text
1. SwayFX
2. Waybar
3. Rofi
4. Dunst
5. swaylock
6. Foot refinement
```

Then:

```text
7. Preview
8. Session layer
9. TUI
10. Doctor
11. Desktop utilities
12. GTK / Qt
```

Do not start the TUI before the generation, preview, and rollback models are
stable, and do not start it by writing terminal code: the session layer comes
first, or the editing logic ends up trapped inside the views.

Do not start a GUI before v1.0.

---

# 61. Estimated Effort

## Architecture Prototype

```text
2–4 engineering days
```

Includes:

* Go project;
* ownership model;
* generations;
* symlinks;
* Foot proof of concept.

## CLI MVP

```text
7–12 engineering days total
```

Includes:

* all core desktop adapters;
* theme engine;
* rollback;
* preview;
* doctor;
* safe setup/uninstall.

## Interactive Version

```text
11–19 engineering days total
```

Adds:

* session layer;
* sandbox preview;
* font enumeration and clipboard export;
* TUI theme picker and editor.

## Strong Daily-Use Version

```text
14–24 engineering days total
```

Adds:

* desktop utilities;
* GTK/Qt;
* better testing;
* edge-case handling;
* documentation.

## Public Release

```text
18–30 engineering days total
```

Adds:

* per-program editors;
* wallpaper themes;
* packaging;
* release polish.

A GUI, if built later, is separate from these numbers.

---

# 62. Architectural Summary

The preferred architecture is:

```text
                    USER SOURCE
                        │
                        ▼
              ~/.config/rice/
              themes + settings
                        │
                   Go compiler
                        │
                        ▼
              immutable generation
                        │
                        ▼
                current symlink
                        │
       ┌────────────────┼────────────────┐
       ▼                ▼                ▼
     SwayFX           Waybar            Foot
       ▼                ▼                ▼
      Rofi             Dunst          swaylock
```

Application-facing configuration:

```text
~/.config/<application>/...
          │
          └── symlink → ~/.config/rice/current/...
```

Existing configuration is backed up once during adoption.

Theme/configuration changes create new generations.

Rollback changes one symlink.

Preview uses a temporary mutable generation.

Uninstall restores the original configuration.

The implementation is a small Go application using normal filesystem semantics, standard configuration formats, standard Wayland tools, and minimal dependencies.

This should remain the core architectural model unless implementation experience demonstrates a concrete reason to change it.
