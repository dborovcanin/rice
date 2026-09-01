// Package ownership decides which application configuration files Rice may
// touch. Every destructive step in Rice goes through this package, and the
// rule it enforces is simple: a file Rice did not create is never replaced
// without an explicit adoption that backs it up first.
package ownership

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// State is what currently sits at an application configuration path.
type State int

const (
	// Missing means nothing exists at the path.
	Missing State = iota
	// RegularFile means a real file the user owns.
	RegularFile
	// Directory means a directory sits where a file belongs.
	Directory
	// RiceManaged means a symlink already pointing into the Rice root.
	RiceManaged
	// ExternalSymlink means a symlink pointing somewhere Rice does not own.
	ExternalSymlink
	// BrokenSymlink means a symlink whose target does not exist.
	BrokenSymlink
)

func (s State) String() string {
	switch s {
	case RegularFile:
		return "regular file"
	case Directory:
		return "directory"
	case RiceManaged:
		return "rice-managed"
	case ExternalSymlink:
		return "external symlink"
	case BrokenSymlink:
		return "broken symlink"
	default:
		return "missing"
	}
}

// Owned reports whether Rice may replace this path without backing anything up.
func (s State) Owned() bool { return s == RiceManaged }

// NeedsBackup reports whether replacing this path would destroy user data.
func (s State) NeedsBackup() bool { return s == RegularFile }

// Status describes one application configuration path.
type Status struct {
	// Path is the application configuration path itself.
	Path string
	// State is what sits there now.
	State State
	// LinkTarget is the symlink target, resolved to an absolute path, for the
	// symlink states.
	LinkTarget string
}

// Detect inspects a path and classifies it relative to the Rice root. It never
// modifies anything.
func Detect(path, riceRoot string) (Status, error) {
	status := Status{Path: path}

	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return status, nil
	case err != nil:
		return status, fmt.Errorf("inspect %s: %w", path, err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		if info.IsDir() {
			status.State = Directory
			return status, nil
		}
		status.State = RegularFile
		return status, nil
	}

	target, err := os.Readlink(path)
	if err != nil {
		return status, fmt.Errorf("read link %s: %w", path, err)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	status.LinkTarget = filepath.Clean(target)

	// A link into the Rice root is ours even when it currently dangles, which
	// is the normal state after a generation is pruned.
	if within(status.LinkTarget, riceRoot) {
		status.State = RiceManaged
		return status, nil
	}
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		status.State = BrokenSymlink
		return status, nil
	} else if err != nil {
		return status, fmt.Errorf("resolve %s: %w", path, err)
	}

	status.State = ExternalSymlink
	return status, nil
}

// within reports whether path is inside root.
func within(path, root string) bool {
	if root == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
