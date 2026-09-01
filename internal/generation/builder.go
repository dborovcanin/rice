package generation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/render"
	"github.com/dborovcanin/rice/internal/theme"
)

// Builder renders a full set of application configuration files. It writes
// into a directory it is handed and never touches the live desktop.
type Builder struct {
	Engine   *render.Engine
	Registry *adapter.Registry
	Version  string
	// Now is injected so golden tests produce stable manifests.
	Now func() time.Time
}

// NewBuilder returns a builder over an engine and adapter registry.
func NewBuilder(engine *render.Engine, registry *adapter.Registry, version string) *Builder {
	return &Builder{Engine: engine, Registry: registry, Version: version, Now: time.Now}
}

// Rendered is one generated file held in memory, before anything is written.
type Rendered struct {
	Component string
	Path      string
	Content   []byte
	Mode      os.FileMode
	Reload    adapter.ReloadMode
}

// Render produces every enabled component's files without writing to disk.
// Rendering never partially succeeds: one bad template fails the whole build.
func (b *Builder) Render(cfg config.Config, th theme.Theme, generation int) ([]Rendered, error) {
	names := cfg.Components.Names()
	adapters, err := b.Registry.Select(names)
	if err != nil {
		return nil, err
	}

	ctx := render.NewContext(cfg, th, generation, b.Version)

	var out []Rendered
	for _, a := range adapters {
		for _, f := range a.Files() {
			content, err := b.Engine.Render(f.Template, ctx)
			if err != nil {
				return nil, fmt.Errorf("component %s: %w", a.Name(), err)
			}
			out = append(out, Rendered{
				Component: a.Name(),
				Path:      f.Path,
				Content:   content,
				Mode:      f.FileMode(),
				Reload:    a.ReloadMode(),
			})
		}
	}
	return out, nil
}

// Build renders into dir, runs each adapter's validation over the result and
// writes the manifest. dir must already exist and should be empty.
func (b *Builder) Build(dir string, cfg config.Config, th theme.Theme, generation int, opts BuildOptions) (Manifest, error) {
	files, err := b.Render(cfg, th, generation)
	if err != nil {
		return Manifest{}, err
	}

	for _, f := range files {
		dest := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return Manifest{}, fmt.Errorf("create %s: %w", filepath.Dir(dest), err)
		}
		if err := os.WriteFile(dest, f.Content, f.Mode); err != nil {
			return Manifest{}, fmt.Errorf("write %s: %w", dest, err)
		}
	}

	names := cfg.Components.Names()
	adapters, err := b.Registry.Select(names)
	if err != nil {
		return Manifest{}, err
	}
	for _, a := range adapters {
		if err := a.Validate(dir); err != nil {
			return Manifest{}, err
		}
	}

	manifest := Manifest{
		Generation:  generation,
		CreatedAt:   b.now(),
		Theme:       th.Name,
		ThemeSource: opts.ThemeSource,
		RiceVersion: b.Version,
		Parent:      opts.Parent,
		Description: opts.Description,
		Components:  names,
	}
	for _, f := range files {
		sum := sha256.Sum256(f.Content)
		manifest.Files = append(manifest.Files, File{
			Path:   f.Path,
			SHA256: hex.EncodeToString(sum[:]),
			Reload: f.Reload.String(),
		})
	}
	sort.Slice(manifest.Files, func(i, j int) bool {
		return manifest.Files[i].Path < manifest.Files[j].Path
	})

	if err := WriteManifest(dir, manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// BuildOptions carries the metadata a build cannot derive on its own.
type BuildOptions struct {
	Parent      int
	Description string
	ThemeSource string
}

func (b *Builder) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}
