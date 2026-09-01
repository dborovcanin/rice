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
application configuration   ~/.config/<app>/...        not wired up yet
```

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
internal/cli          command tree
templates/, themes/   embedded defaults, overridable per file
```

Dependencies run one way: `theme` and `config` know nothing about rendering,
`render` knows nothing about generations, and `generation` knows nothing about
the CLI.

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

## Not implemented yet

Deployment into `~/.config/<app>` is deliberately absent. That step needs its
own model, since it touches files Rice did not create:

* **Ownership detection** — missing, regular file, Rice symlink, external
  symlink, broken symlink.
* **Adoption** — back up an existing file into `backups/<timestamp>/`, record it
  in an adoption manifest, then install the symlink.
* **Uninstall** — reverse the manifest exactly, restoring the original files.
* **Reload** — use each adapter's reload mode after a switch.
* **Preview** — a mutable `preview/` generation for live editing, committed or
  cancelled explicitly, so slider movements do not create hundreds of
  generations.
* **Doctor** — dependency and ownership diagnostics.
* **Desktop utilities** — `rice run volume|brightness|screenshot|...`.

Until then Rice writes only inside its own root, and nothing outside
`~/.config/rice` is touched.

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
