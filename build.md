# Rice — SwayFX Desktop Configuration & Theming Tool

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
* Optional GUI editor.
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

GUI:                 deferred
```

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

Do not create a committed generation for every file edit or GUI slider movement.

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
│   └── preview/
│       ├── preview.go
│       └── watcher.go
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

[cursor]
theme = "Bibata-Modern-Ice"
size = 24
```

Application-specific values should be derived from the shared model where practical.

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

# 38. GUI Theme Editor

The GUI remains post-MVP.

Do not couple the core architecture to a particular GUI toolkit.

Potential architecture:

```text
            Go core
           /       \
        CLI         GUI
```

The GUI must use the same:

* config parser;
* theme model;
* renderer;
* validator;
* preview engine;
* generation store.

No duplicate theming logic.

---

# 39. GUI Options

## Option A — Native Go GUI

Possible frameworks:

* Fyne
* Gio
* Wails with native/web hybrid architecture

Evaluate only when GUI work begins.

---

## Option B — Embedded Web UI

Potentially preferred.

Architecture:

```text
rice gui
   ↓
Go HTTP server
   ↓
localhost
   ↓
HTML/CSS/JS theme editor
```

Assets can be embedded through:

```go
embed.FS
```

Advantages:

* excellent CSS-based visual previews;
* easy color controls;
* simple layout controls;
* good representation of bars, launchers, terminals, and notifications;
* frontend remains inside the Rice binary.

The GUI technology decision should be deferred until after the CLI/core architecture is stable.

---

# 40. GUI Live Preview

The GUI modifies preview state:

```text
GUI field
   ↓
theme model
   ↓
preview generation
   ↓
reload
```

Controls:

* colors;
* radius;
* gaps;
* borders;
* opacity;
* blur;
* UI font;
* terminal font;
* icon theme;
* cursor theme.

Preview components:

* Sway window;
* Waybar;
* Rofi;
* Foot;
* Dunst;
* swaylock.

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

**Target: 1–2 engineering days**

* [ ] Preview directory.
* [ ] Preview parent tracking.
* [ ] Preview activation.
* [ ] Preview update.
* [ ] Preview commit.
* [ ] Preview cancel.
* [ ] Crash recovery.
* [ ] `fsnotify` live watch mode.

Acceptance:

```bash
rice preview gruvbox-dark
rice preview cancel
```

returns the exact previous generation.

---

# 47. Phase 4 — Desktop Utilities

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

# 48. Phase 5 — GTK / Qt

**Target: 1–3 engineering days**

* [ ] GTK3
* [ ] GTK4 where appropriate
* [ ] icon settings
* [ ] cursor settings
* [ ] fonts
* [ ] qt5ct
* [ ] qt6ct
* [ ] Kvantum
* [ ] environment validation
* [ ] doctor integration

---

# 49. Phase 6 — GUI

**Target: 5–8 engineering days**

* [ ] evaluate native Go vs embedded web UI
* [ ] palette editor
* [ ] layout controls
* [ ] typography controls
* [ ] component previews
* [ ] live preview integration
* [ ] save
* [ ] save as
* [ ] revert

Preferred implementation direction to evaluate first:

```text
Go backend + embedded HTML/CSS/JS frontend
```

because the primary GUI task is visual theme editing.

---

# 50. Phase 7 — Public Release

* [ ] README
* [ ] screenshots
* [ ] architecture documentation
* [ ] setup documentation
* [ ] uninstall documentation
* [ ] bundled themes
* [ ] shell completions
* [ ] CI
* [ ] release binaries
* [ ] AUR package
* [ ] checksums
* [ ] tagged releases

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

Do not force `CGO_ENABLED=0` if a future GUI dependency requires CGO.

CLI releases should ideally remain pure Go.

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
* [ ] `rice doctor`
* [ ] bundled themes
* [ ] automated tests
* [ ] documentation
* [ ] Arch installation instructions
* [ ] release binary

The GUI does not need to block v1.0.

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
8. Doctor
9. Desktop utilities
10. GTK / Qt
```

Do not start the GUI before the generation, preview, and rollback models are stable.

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

## Strong Daily-Use Version

```text
10–15 engineering days
```

Adds:

* desktop utilities;
* GTK/Qt;
* better testing;
* edge-case handling;
* documentation.

## GUI + Public Release

```text
20–30 engineering days total
```

Adds:

* GUI theme editor;
* live visual editing;
* wallpaper themes;
* packaging;
* release polish.

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
