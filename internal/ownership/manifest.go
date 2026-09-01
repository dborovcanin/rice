package ownership

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// ManifestName is the file recording what Rice has adopted. It lives in the
// state directory and is the only thing uninstall trusts.
const ManifestName = "managed.toml"

// Manifest records every adopted path so uninstall can be deterministic.
type Manifest struct {
	Managed []Entry `toml:"managed"`
}

// Entry is one adopted application configuration path.
type Entry struct {
	// Component is the adapter name, e.g. "foot".
	Component string `toml:"component"`
	// Target is the absolute application configuration path.
	Target string `toml:"target"`
	// Source is the path inside a generation, e.g. "foot/foot.ini".
	Source string `toml:"source"`
	// Backup is the saved original, relative to the Rice root. Empty when
	// nothing existed at adoption time.
	Backup string `toml:"backup,omitempty"`
	// AdoptedAt is when the path was adopted.
	AdoptedAt time.Time `toml:"adopted_at"`
}

// HadExisting reports whether adoption displaced a real file.
func (e Entry) HadExisting() bool { return e.Backup != "" }

// LoadManifest reads the adoption manifest. A missing file is an empty
// manifest, not an error: nothing adopted yet is a normal state.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Manifest{}, nil
	}
	if err != nil {
		return Manifest{}, fmt.Errorf("read adoption manifest: %w", err)
	}

	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

// SaveManifest writes the adoption manifest atomically, so an interrupted write
// cannot leave Rice unable to uninstall itself.
func SaveManifest(path string, m Manifest) error {
	data, err := toml.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode adoption manifest: %w", err)
	}
	header := "# Paths Rice has adopted, and where their originals are kept.\n" +
		"# `rice uninstall` reverses exactly what is listed here.\n\n"

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".managed-")
	if err != nil {
		return fmt.Errorf("stage adoption manifest: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(append([]byte(header), data...)); err != nil {
		tmp.Close()
		return fmt.Errorf("write adoption manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write adoption manifest: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("set adoption manifest permissions: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("finalize adoption manifest: %w", err)
	}
	return nil
}

// Find returns the entry for a target path.
func (m Manifest) Find(target string) (Entry, bool) {
	for _, e := range m.Managed {
		if e.Target == target {
			return e, true
		}
	}
	return Entry{}, false
}

// Components returns the adopted component names, without duplicates.
func (m Manifest) Components() []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range m.Managed {
		if !seen[e.Component] {
			seen[e.Component] = true
			out = append(out, e.Component)
		}
	}
	return out
}

// Adopted reports whether any path of a component is adopted.
func (m Manifest) Adopted(component string) bool {
	for _, e := range m.Managed {
		if e.Component == component {
			return true
		}
	}
	return false
}

// Upsert replaces the entry for a target, or appends it.
func (m *Manifest) Upsert(e Entry) {
	for i := range m.Managed {
		if m.Managed[i].Target == e.Target {
			m.Managed[i] = e
			return
		}
	}
	m.Managed = append(m.Managed, e)
}

// Remove drops the entry for a target.
func (m *Manifest) Remove(target string) {
	out := m.Managed[:0]
	for _, e := range m.Managed {
		if e.Target != target {
			out = append(out, e)
		}
	}
	m.Managed = out
}
