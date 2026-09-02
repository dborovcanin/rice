# Architecture

Rice separates four kinds of state, and most of its behaviour follows from that
separation:

```
source configuration        config.toml + themes/      you edit this
        │
     compile                text/template
        ▼
generated generation        generations/000042/        immutable, validated
        │
   current symlink          current -> generations/000042
        ▼
application configuration   ~/.config/<app>/...        symlinks, once adopted
```

Because application paths point at `current/` rather than at a numbered
generation, switching generations is one rename and touches nothing under
`~/.config`.

## Why whole files

Rice generates complete application configuration files rather than fragments
included by hand-written ones. Every application has a different include
mechanism — Sway has `include`, Rofi has `@import`, Waybar has `@import` in CSS
only, Foot and Dunst have neither — and depending on them would mean a different
integration story per application.

Whole-file generation gives uniform behaviour, simpler adapters, validation that
can look at the finished file, atomic generations and clean rollback. The output
is ordinary, readable application configuration: removing Rice leaves standard
files behind.

## Packages

```
internal/theme        palette model, colour maths, parsing, normalization, validation
internal/config       source configuration, defaults, path resolution
internal/render       template engine, function set, template context
internal/adapter      per-application file declarations and output validation
internal/generation   builder, manifest, generation store, current symlink
internal/ownership    detection, deployment planning, adoption, backups, restore
internal/reload       telling running applications to re-read their config
internal/command      process execution, with a fake for tests
internal/cli          command tree
templates/, themes/   embedded defaults, overridable per file
```

Dependencies run one way: `theme` and `config` know nothing about rendering,
`render` knows nothing about generations, and `generation` knows nothing about
the CLI. `ownership` and `reload` depend on `adapter` for declarations only —
an adapter still never deploys or reloads anything itself.

Process execution is centralized in `internal/command` behind a `Runner`
interface, so the reload path is tested against a fake and no test ever touches
a real desktop session.

## Themes and configuration

Appearance is in a theme, structure is in `config.toml`. That split is what
makes a theme switch a single command: nothing about your outputs, bindings or
window rules changes when the palette does.

Both are normalized before validation, so validation describes the *effective*
value rather than the file, and both collect every problem in one pass rather
than failing on the first.

Themes derive aggressively — three colours are enough for a complete desktop —
because a theme that requires forty values is a theme nobody writes.

## Adapters

An adapter declares what one application needs and nothing more:

```go
type Adapter interface {
    Name() string
    Files() []File               // template -> path inside a generation
    ConfigPaths() []ManagedPath  // generated file -> application config path
    ReloadMode() ReloadMode      // how the application picks up changes
    Validate(dir string) error   // check the generated output
}
```

Adapters do not deploy, reload or own files. Deployment is generic and lives in
the generation machinery, so adding an application means adding a template and a
short adapter, not a new integration path.

`ReloadMode` records that applications genuinely differ — Sway reloads in place,
Dunst reloads on a signal, Rofi and Foot only affect new instances — instead of
pretending they behave alike. It is recorded per file in the manifest and will
drive the reload step once deployment exists.

## Generations

A generation is a directory of complete configuration files plus a
`manifest.toml` recording the theme, the parent generation, the Rice version, a
description and every file with its SHA-256 and reload mode. Committed
generations are never modified.

Building is transactional:

```
render everything in memory      one bad template fails the whole build
        ↓
write into a staging directory   generations/.build-XXXX
        ↓
validate through the adapters
        ↓
rename into generations/000042   atomic
        ↓
switch current                   atomic
```

A failure at any step removes the staging directory and consumes no generation
number.

Both switches are renames: `current` is created as a sibling symlink and renamed
over the old one, so there is no moment where `current` does not exist. It is a
relative link, which keeps the whole tree movable.

Retention keeps the newest N generations and never removes the current or
previous one, whatever the limit says.

## Rollback

Rollback moves one symlink. It rebuilds nothing, so it returns to the exact
bytes a generation was committed with, and it records where it came from —
rolling back twice returns to where you started.

## Deployment

Deployment is the only part of Rice that touches files it did not create, so it
is deliberately conservative: `rice setup` is a dry run unless told otherwise,
an existing file is copied into `backups/` before it is replaced, and the
adoption is recorded so uninstall can reverse it exactly. `rice apply` repairs
and reloads only what was already adopted; it never adopts on its own.

[deployment.md](deployment.md) documents the ownership states and the
guarantees in full.

## The interactive editor

`rice tui` is a view over `internal/session`, which holds a draft theme and the
operations on it. The editor itself holds only cursor position, focus and
filter text; every value the user changes goes through the session, which is
where that logic is tested.

A draft is both the theme being edited and the configuration it renders
against, so a palette color and a bar's height are the same kind of thing to
the interface. They are saved to different files: appearance to a theme,
structure to `config.toml`.

The session keeps the theme in **source form** — exactly as a theme file is
written, with unset fields still unset — and normalizes a copy for rendering.
That is what makes editing behave like editing the file: a value the theme
leaves derived keeps following whatever it is derived from, and a value the
theme spells out does not move on its own. The editor shows which is which.

Previewing renders the draft into a private directory under the system
temporary directory and runs the real application against it. That is separate
from `rice preview`: a sandbox never moves `current` and never touches
`~/.config`, so the running desktop is unaffected until the draft is applied.

A future graphical editor would be a second view over the same session, not a
second implementation.

## Not implemented yet

* **Preview** — a mutable `preview/` generation for live editing, committed or
  cancelled explicitly, so slider movements do not create hundreds of
  generations. The editor's sandbox preview is a different thing: it never
  moves `current`.
* **Desktop utilities** — `rice run volume|brightness|screenshot|...`, so
  bindings stop pointing at ad-hoc shell scripts.
* **`rice theme from-image`** — deriving a palette from a wallpaper.
* **GUI** — a graphical editor over the same session layer.

## Invariants

These hold today and must keep holding:

* **Ownership** — Rice never modifies a user-owned config without explicit
  adoption.
* **Recoverability** — adopting Rice is reversible.
* **Atomicity** — the desktop never lands in a half-old, half-new state.
* **Immutability** — committed generations are never modified.
* **Source of truth** — generated files are output, not editable state.
* **Independence** — removing Rice leaves standard applications and standard
  configuration formats behind.
* **Runtime simplicity** — normal use needs no running daemon.
