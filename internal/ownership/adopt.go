package ownership

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dborovcanin/rice/internal/config"
)

// Options controls how a plan is carried out.
type Options struct {
	// Force allows replacing an external symlink. It never applies to a
	// directory standing where a file belongs.
	Force bool
	// Now is injected so tests get stable backup directory names.
	Now func() time.Time
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// Result reports what applying a plan actually did.
type Result struct {
	// Linked, Adopted and Relinked hold the targets in each category.
	Linked   []string
	Adopted  []string
	Relinked []string
	// Skipped holds the conflicts that were refused.
	Skipped []Action
	// BackupDir is where displaced files were saved, relative to the Rice
	// root. Empty when nothing was displaced.
	BackupDir string
}

// Changed reports whether anything was modified.
func (r Result) Changed() bool {
	return len(r.Linked)+len(r.Adopted)+len(r.Relinked) > 0
}

// Apply carries out a plan and records what it did in the adoption manifest.
//
// Order matters: an existing file is copied into the backup directory and the
// copy is verified before the original is replaced, so an interrupted adoption
// can lose nothing.
func Apply(plan Plan, paths config.Paths, opts Options) (Result, error) {
	var result Result

	manifestPath := filepath.Join(paths.StateDir, ManifestName)
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return result, err
	}

	stamp := opts.now().UTC().Format("2006-01-02T150405Z")
	backupRoot := filepath.Join(paths.BackupsDir, stamp)

	for _, action := range plan.Actions {
		switch action.Kind {
		case KindNone:
			continue

		case KindConflict:
			if !(opts.Force && action.Forceable()) {
				result.Skipped = append(result.Skipped, action)
				continue
			}
			// A forced external symlink is removed, not backed up: the file it
			// pointed at is somebody else's and stays untouched.
			if err := os.Remove(action.Target); err != nil {
				return result, fmt.Errorf("remove %s: %w", action.Target, err)
			}
			if err := link(action); err != nil {
				return result, err
			}
			manifest.Upsert(entryFor(action, "", opts.now()))
			result.Relinked = append(result.Relinked, action.Target)

		case KindAdopt:
			rel := filepath.Join(stamp, action.Component, filepath.Base(action.Target))
			backup := filepath.Join(paths.BackupsDir, rel)
			if err := copyFile(action.Target, backup); err != nil {
				return result, err
			}
			if err := os.Remove(action.Target); err != nil {
				return result, fmt.Errorf("remove %s: %w", action.Target, err)
			}
			if err := link(action); err != nil {
				return result, err
			}
			manifest.Upsert(entryFor(action, filepath.Join("backups", rel), opts.now()))
			result.Adopted = append(result.Adopted, action.Target)
			result.BackupDir = backupRoot

		case KindRelink:
			if err := os.Remove(action.Target); err != nil {
				return result, fmt.Errorf("remove %s: %w", action.Target, err)
			}
			if err := link(action); err != nil {
				return result, err
			}
			existing, _ := manifest.Find(action.Target)
			manifest.Upsert(entryFor(action, existing.Backup, opts.now()))
			result.Relinked = append(result.Relinked, action.Target)

		case KindLink:
			if err := link(action); err != nil {
				return result, err
			}
			manifest.Upsert(entryFor(action, "", opts.now()))
			result.Linked = append(result.Linked, action.Target)
		}
	}

	if result.Changed() {
		if err := SaveManifest(manifestPath, manifest); err != nil {
			return result, err
		}
	}
	return result, nil
}

// Relink repairs the symlinks of already-adopted paths. It never adopts
// anything new, which is what makes it safe to run after every apply.
func Relink(plan Plan, paths config.Paths) (Result, error) {
	var result Result

	manifest, err := LoadManifest(filepath.Join(paths.StateDir, ManifestName))
	if err != nil {
		return result, err
	}

	for _, action := range plan.Actions {
		if _, ok := manifest.Find(action.Target); !ok {
			continue
		}
		switch action.Kind {
		case KindRelink:
			if err := os.Remove(action.Target); err != nil {
				return result, fmt.Errorf("remove %s: %w", action.Target, err)
			}
			if err := link(action); err != nil {
				return result, err
			}
			result.Relinked = append(result.Relinked, action.Target)
		case KindLink:
			// The link was adopted before but has since disappeared.
			if err := link(action); err != nil {
				return result, err
			}
			result.Linked = append(result.Linked, action.Target)
		case KindConflict:
			result.Skipped = append(result.Skipped, action)
		}
	}
	return result, nil
}

func entryFor(a Action, backup string, now time.Time) Entry {
	return Entry{
		Component: a.Component,
		Target:    a.Target,
		Source:    a.Source,
		Backup:    backup,
		AdoptedAt: now.UTC(),
	}
}

// link creates the symlink, creating the parent directory when the application
// has never been configured before.
func link(a Action) error {
	if err := os.MkdirAll(filepath.Dir(a.Target), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(a.Target), err)
	}
	if err := os.Symlink(a.LinkTo, a.Target); err != nil {
		return fmt.Errorf("link %s: %w", a.Target, err)
	}
	return nil
}

// copyFile copies src to dst, creating parents and preserving the mode. The
// copy is fully written and closed before the caller touches the original.
func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dst), err)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("write backup %s: %w", dst, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("write backup %s: %w", dst, err)
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return fmt.Errorf("flush backup %s: %w", dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("write backup %s: %w", dst, err)
	}
	return nil
}
