# Deployment, adoption and uninstall

Generating configuration is one thing; pointing applications at it is another,
and it is the only part of Rice that touches files Rice did not create. This
page describes exactly what it will and will not do.

The rule everything else follows:

> Rice never replaces a file it did not create without copying it into
> `backups/` first and recording where it came from.

## Ownership states

Every application configuration path is classified before anything happens:

| State | Meaning | What Rice does |
| --- | --- | --- |
| missing | Nothing is there | Creates the symlink |
| regular file | Your own file | **Adopts**: backs it up, then links |
| rice-managed | A symlink into the Rice root | Repairs it if it points at the wrong place |
| external symlink | A symlink somewhere Rice does not own | **Refuses**, unless `--force` |
| broken symlink | A symlink whose target is gone | Replaces it |
| directory | A directory where a file belongs | **Refuses**, always |

A symlink into the Rice root counts as ours even when it currently dangles:
that is the normal state after a generation is pruned, not a conflict.

A directory standing where a file belongs is never touched, forced or not.
Removing it could destroy a tree, and no flag is worth that.

## Dry run first

`rice setup` shows the plan and changes nothing:

```bash
rice setup
```

```
Application configuration under /home/user/.config

+ /home/user/.config/dunst/dunstrc        adopt     regular file
+ /home/user/.config/foot/foot.ini        adopt     regular file
! /home/user/.config/rofi/config.rasi     conflict  points at /home/user/dotfiles/rofi.rasi, which Rice does not own
+ /home/user/.config/sway/config          adopt     regular file
+ /home/user/.config/waybar/config.jsonc  link      missing

3 existing file(s) would be backed up before being replaced:
  ...

Dry run: nothing was changed. Re-run with --adopt to apply.
```

`+` would change something, `!` is a refusal, `=` is already correct.

To rehearse against a directory that is not your real configuration:

```bash
rice setup --config-dir /tmp/fake-config --adopt
```

## Adoption

```bash
rice setup --adopt
```

For each path with an existing file:

1. Copy it to `backups/<timestamp>/<component>/<file>`, and flush the copy to
   disk **before** the original is touched.
2. Remove the original.
3. Create the symlink into `current/`.
4. Record the target, its source inside a generation, and the backup path in
   `state/managed.toml`.

An interrupted adoption therefore cannot lose a file: the backup is complete
before anything is removed.

Adoption is idempotent. Running setup again over an adopted tree changes
nothing and takes no second backup.

### Conflicts

An external symlink is left alone and reported. If you want Rice to take it
over:

```bash
rice setup --adopt --force
```

Force replaces the *link*. Whatever it pointed at is not read, not copied and
not modified — Rice has no claim on a file in someone else's tree.

## The adoption manifest

`state/managed.toml` is the only thing uninstall trusts:

```toml
[[managed]]
component = "foot"
target = "/home/user/.config/foot/foot.ini"
source = "foot/foot.ini"
backup = "backups/2026-09-02T100000Z/foot/foot.ini"
adopted_at = 2026-09-02T10:00:00Z
```

An entry with no `backup` means nothing was there before, so uninstall removes
the symlink rather than restoring anything.

It is written atomically, so an interrupted write cannot leave Rice unable to
uninstall itself.

## Applying after adoption

`rice apply` redeploys and reloads whatever is already adopted:

```
generation built and committed
        ↓
current -> generations/000043
        ↓
repair the symlinks of adopted paths
        ↓
reload the adopted components
```

Apply **never adopts anything on its own**. A path you have not handed to Rice
through `rice setup` is not touched, no matter what changes. If nothing is
adopted, apply behaves exactly as it did before deployment existed.

Because application paths point at `current/` rather than at a specific
generation, a theme switch or a rollback needs no relinking at all — the
symlinks are already correct. The repair step exists for the cases where they
are not: a link deleted by hand, or one left pointing at a pruned generation.

## Reload

Applications differ in what "reload" means, and Rice does not pretend
otherwise. Each adapter declares a mode, and only the adopted components are
touched:

| Component | Mode | Command |
| --- | --- | --- |
| sway | hot | `swaymsg reload` |
| dunst | signal | `dunstctl reload` |
| waybar | signal | `pkill -SIGUSR2 -x waybar` |
| rofi, foot, swaylock, gtk, qt | new instances only | nothing |

A component that is not running is reported, not treated as a failure. Rice
checks with `pgrep` first; without `pgrep` it attempts the reload rather than
skipping it.

```bash
rice apply --no-reload      # deploy without poking anything
```

Foot deserves a note: a running terminal keeps its palette, and the foot server
only applies a new one to terminals opened afterwards. Open a new terminal to
see a theme change, or restart the server.

GTK and Qt deserve a longer one. Their configuration is read when an
application starts, so a running program keeps the theme it launched with —
restart it. The session environment goes further: `environment.d/50-rice.conf`
is read by the systemd user manager at login, so `XCURSOR_THEME`,
`XCURSOR_SIZE` and `QT_QPA_PLATFORMTHEME` only change after a fresh session.
Rice reports these as new-instances-only rather than claiming a reload it
cannot perform.

Toolkit integration also adds paths outside the one-file-per-application
pattern the rest of Rice follows:

| Source in a generation | Deployed to |
| --- | --- |
| `gtk/settings.ini` | `gtk-3.0/settings.ini` **and** `gtk-4.0/settings.ini` |
| `gtk/gtk.css` | `gtk-3.0/gtk.css` **and** `gtk-4.0/gtk.css` |
| `qt/qt5ct.conf` | `qt5ct/qt5ct.conf` |
| `qt/qt6ct.conf` | `qt6ct/qt6ct.conf` |
| `qt/kvantum.kvconfig` | `Kvantum/kvantum.kvconfig` |
| `sway/environment.conf` | `environment.d/50-rice.conf` |

One rendered file linking into two places is deliberate: GTK 3 and GTK 4 read
the same keys from separate directories, and generating the file twice would
let them drift.

Which of these exist at all depends on `[gtk]` and `[qt]` in `config.toml`, so
turning `gtk.css` off removes it from the plan rather than leaving a stale
symlink behind.

## Uninstall

```bash
rice uninstall              # dry run
rice uninstall --yes        # do it
```

Uninstall walks the manifest, removes the symlinks Rice installed, and copies
the backed-up originals back into place. It is a copy rather than a move, so
the backup directory survives.

A path that is no longer the Rice symlink the manifest describes is skipped and
reported: something else owns it now, and Rice will not second-guess that.

Nothing else is deleted. Themes, generations, backups and `config.toml` stay
where they are, so uninstalling and setting up again is cheap.

The manifest is pruned entry by entry as the work proceeds, so an interrupted
uninstall can be resumed rather than repeated.

## Checking state

`rice status` (also `rice doctor`) reads and reports, and changes nothing:

* the active theme and generation, and how many generations are stored;
* every managed path with its ownership state, including refusals;
* whether the supporting programs are installed.

It is the safe way to find out where things stand before running anything.

## What is deliberately absent

* Rice never runs as root; `setup --adopt` and `uninstall --yes` refuse.
* Rice never installs packages. `rice status` reports what is missing.
* Rice never edits a file in place. It replaces whole files through symlinks,
  or it does nothing.
* Rice never deletes a backup. Removing `backups/` is your decision.
