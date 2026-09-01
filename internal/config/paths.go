package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Paths holds every filesystem location Rice uses. Everything is derived from
// a single root so tests can point Rice at a temporary directory.
type Paths struct {
	Root string

	ConfigFile   string
	ThemesDir    string
	TemplatesDir string
	ScriptsDir   string

	GenerationsDir string
	PreviewDir     string
	BackupsDir     string
	StateDir       string

	Current string
}

// NewPaths derives all paths from a Rice root directory.
func NewPaths(root string) Paths {
	return Paths{
		Root:           root,
		ConfigFile:     filepath.Join(root, "config.toml"),
		ThemesDir:      filepath.Join(root, "themes"),
		TemplatesDir:   filepath.Join(root, "templates"),
		ScriptsDir:     filepath.Join(root, "scripts"),
		GenerationsDir: filepath.Join(root, "generations"),
		PreviewDir:     filepath.Join(root, "preview"),
		BackupsDir:     filepath.Join(root, "backups"),
		StateDir:       filepath.Join(root, "state"),
		Current:        filepath.Join(root, "current"),
	}
}

// DefaultPaths resolves the Rice root from the environment: $RICE_HOME wins,
// then $XDG_CONFIG_HOME/rice, then ~/.config/rice.
func DefaultPaths() (Paths, error) {
	if root := os.Getenv("RICE_HOME"); root != "" {
		return NewPaths(root), nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return NewPaths(filepath.Join(xdg, "rice")), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}
	return NewPaths(filepath.Join(home, ".config", "rice")), nil
}

// EnsureDirs creates the directories Rice writes into. It does not create the
// config file; that is the caller's decision.
func (p Paths) EnsureDirs() error {
	dirs := []string{
		p.Root,
		p.ThemesDir,
		p.TemplatesDir,
		p.ScriptsDir,
		p.GenerationsDir,
		p.BackupsDir,
		p.StateDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

// Generation returns the directory for a generation number.
func (p Paths) Generation(n int) string {
	return filepath.Join(p.GenerationsDir, FormatGeneration(n))
}

// FormatGeneration renders a generation number as a zero-padded directory name.
func FormatGeneration(n int) string {
	return fmt.Sprintf("%06d", n)
}

// ExpandPath expands a leading ~ and any environment variables in a
// user-supplied path.
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	expanded := os.ExpandEnv(path)
	if expanded == "~" || strings.HasPrefix(expanded, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(expanded, "~"))
		}
	}
	return expanded
}
