package ownership

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dborovcanin/rice/internal/config"
)

// RestoreAction is one planned step of an uninstall.
type RestoreAction struct {
	Entry Entry
	// Status is what sits at the target now.
	Status Status
	// Restore is the absolute path of the backup to put back, if any.
	Restore string
	// Skip explains why the entry will be left alone, and is empty otherwise.
	Skip string
}

// RestorePlan is what uninstall would do, before anything is done.
type RestorePlan struct {
	Actions []RestoreAction
}

// Changes returns the entries uninstall would act on.
func (p RestorePlan) Changes() []RestoreAction {
	var out []RestoreAction
	for _, a := range p.Actions {
		if a.Skip == "" {
			out = append(out, a)
		}
	}
	return out
}

// Skipped returns the entries uninstall would leave alone.
func (p RestorePlan) Skipped() []RestoreAction {
	var out []RestoreAction
	for _, a := range p.Actions {
		if a.Skip != "" {
			out = append(out, a)
		}
	}
	return out
}

// BuildRestorePlan works out how to reverse every adoption, without modifying
// anything. An entry is skipped whenever the path is no longer the Rice symlink
// the manifest describes: something else now owns it, and Rice will not
// second-guess that.
func BuildRestorePlan(manifest Manifest, paths config.Paths) (RestorePlan, error) {
	var plan RestorePlan

	for _, entry := range manifest.Managed {
		action := RestoreAction{Entry: entry}

		status, err := Detect(entry.Target, paths.Root)
		if err != nil {
			return RestorePlan{}, err
		}
		action.Status = status

		switch status.State {
		case RiceManaged:
			// Expected: our symlink is still there.
		case Missing:
			if !entry.HadExisting() {
				action.Skip = "already gone"
			}
		default:
			action.Skip = fmt.Sprintf("no longer a Rice symlink (%s)", status.State)
		}

		if entry.HadExisting() {
			backup := filepath.Join(paths.Root, filepath.FromSlash(entry.Backup))
			if _, err := os.Stat(backup); err != nil {
				action.Skip = fmt.Sprintf("backup missing: %s", entry.Backup)
			} else {
				action.Restore = backup
			}
		}

		plan.Actions = append(plan.Actions, action)
	}
	return plan, nil
}

// RestoreResult reports what an uninstall actually did.
type RestoreResult struct {
	// Restored holds targets whose original file was put back.
	Restored []string
	// Removed holds targets whose symlink was removed, with nothing to restore.
	Removed []string
	// Skipped holds the entries left alone.
	Skipped []RestoreAction
}

// Changed reports whether anything was modified.
func (r RestoreResult) Changed() bool { return len(r.Restored)+len(r.Removed) > 0 }

// Restore carries out an uninstall plan and prunes the adoption manifest as it
// goes, so an interrupted uninstall can be resumed rather than repeated.
//
// Backups are copied back rather than moved: the backup directory survives an
// uninstall, and removing it stays an explicit, separate decision.
func Restore(plan RestorePlan, paths config.Paths) (RestoreResult, error) {
	var result RestoreResult

	manifestPath := filepath.Join(paths.StateDir, ManifestName)
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return result, err
	}

	for _, action := range plan.Actions {
		if action.Skip != "" {
			result.Skipped = append(result.Skipped, action)
			continue
		}

		if action.Status.State == RiceManaged {
			if err := os.Remove(action.Entry.Target); err != nil {
				return result, fmt.Errorf("remove %s: %w", action.Entry.Target, err)
			}
		}

		if action.Restore != "" {
			if err := copyFile(action.Restore, action.Entry.Target); err != nil {
				return result, err
			}
			result.Restored = append(result.Restored, action.Entry.Target)
		} else {
			result.Removed = append(result.Removed, action.Entry.Target)
		}

		manifest.Remove(action.Entry.Target)
		if err := SaveManifest(manifestPath, manifest); err != nil {
			return result, err
		}
	}
	return result, nil
}
