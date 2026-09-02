// Package assets enumerates the themes installed on the machine: icon themes,
// cursor themes, GTK themes and Kvantum themes.
//
// Everything here is a directory scan of the XDG data directories. Unlike
// fonts, which need fontconfig to resolve aliases and substitutions, a theme
// either has a directory or it does not.
package assets

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Kind is a family of installed theme.
type Kind int

const (
	// IconThemes are directories under icons/ carrying an index.theme.
	IconThemes Kind = iota
	// CursorThemes are directories under icons/ carrying a cursors/ directory.
	CursorThemes
	// GTKThemes are directories under themes/ carrying a gtk-3.0 or gtk-4.0
	// directory.
	GTKThemes
	// KvantumThemes are directories under Kvantum/.
	KvantumThemes
)

func (k Kind) String() string {
	switch k {
	case IconThemes:
		return "icon theme"
	case CursorThemes:
		return "cursor theme"
	case GTKThemes:
		return "gtk theme"
	case KvantumThemes:
		return "kvantum theme"
	}
	return "theme"
}

// builtinGTKThemes ship inside GTK rather than as a directory, so a scan never
// finds them even though they work. Adwaita is the default on most systems.
var builtinGTKThemes = []string{"Adwaita", "Adwaita-dark", "HighContrast", "HighContrastInverse"}

// Builtin reports whether a name works without being installed anywhere.
func Builtin(kind Kind, name string) bool {
	if kind != GTKThemes {
		return false
	}
	for _, b := range builtinGTKThemes {
		if strings.EqualFold(b, strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

// List returns every installed theme of a kind, sorted and deduplicated. A
// name found in more than one directory is listed once: the first match wins
// at lookup time, and which one that is does not change the name.
func List(kind Kind) []string {
	seen := map[string]bool{}

	for _, root := range Roots(kind) {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !isDir(root, entry.Name(), entry) {
				continue
			}
			if qualifies(kind, filepath.Join(root, entry.Name())) {
				seen[entry.Name()] = true
			}
		}
	}

	if kind == GTKThemes {
		// These have no directory anywhere, and Adwaita is the default on most
		// systems, so leaving them out would hide the likeliest answer.
		for _, b := range builtinGTKThemes {
			seen[b] = true
		}
	}

	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

// Installed reports whether one named theme exists.
func Installed(kind Kind, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if Builtin(kind, name) {
		return true
	}
	for _, root := range Roots(kind) {
		if qualifies(kind, filepath.Join(root, name)) {
			return true
		}
	}
	return false
}

// isDir reports whether an entry is a directory, following symlinks: theme
// directories are commonly symlinked into place.
func isDir(root, name string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}
	info, err := os.Stat(filepath.Join(root, name))
	return err == nil && info.IsDir()
}

// qualifies reports whether a directory really is a theme of this kind, rather
// than any directory that happens to sit in the right place.
func qualifies(kind Kind, dir string) bool {
	switch kind {
	case IconThemes:
		return exists(filepath.Join(dir, "index.theme"))
	case CursorThemes:
		return isDirectory(filepath.Join(dir, "cursors"))
	case GTKThemes:
		return isDirectory(filepath.Join(dir, "gtk-3.0")) || isDirectory(filepath.Join(dir, "gtk-4.0"))
	case KvantumThemes:
		return isDirectory(dir)
	}
	return false
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Roots are the directories a kind of theme is looked for in, in lookup order:
// the user's own first, then the system's.
func Roots(kind Kind) []string {
	switch kind {
	case IconThemes, CursorThemes:
		roots := dataDirs("icons")
		if home, err := os.UserHomeDir(); err == nil {
			// The legacy location, still in use.
			roots = append(roots, filepath.Join(home, ".icons"))
		}
		return roots
	case GTKThemes:
		return dataDirs("themes")
	case KvantumThemes:
		var roots []string
		if cfg := os.Getenv("XDG_CONFIG_HOME"); cfg != "" {
			roots = append(roots, filepath.Join(cfg, "Kvantum"))
		} else if home, err := os.UserHomeDir(); err == nil {
			roots = append(roots, filepath.Join(home, ".config", "Kvantum"))
		}
		return append(roots, dataDirs("Kvantum")...)
	}
	return nil
}

// dataDirs returns the XDG data directories with a subdirectory appended.
func dataDirs(sub string) []string {
	var roots []string

	if home := os.Getenv("XDG_DATA_HOME"); home != "" {
		roots = append(roots, home)
	} else if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, ".local", "share"))
	}

	dirs := os.Getenv("XDG_DATA_DIRS")
	if dirs == "" {
		dirs = "/usr/local/share:/usr/share"
	}
	roots = append(roots, filepath.SplitList(dirs)...)

	out := make([]string, 0, len(roots))
	for _, root := range roots {
		out = append(out, filepath.Join(root, sub))
	}
	return out
}
