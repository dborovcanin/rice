// Package adapter describes what each application needs generated and where
// that output belongs. Adapters do not deploy, reload or own files: they only
// declare intent, so the generation machinery stays generic.
package adapter

import (
	"fmt"
	"io/fs"
	"sort"

	"github.com/dborovcanin/rice/internal/config"
)

// ReloadMode records how an application picks up new configuration. Rice does
// not pretend every application behaves the same way.
type ReloadMode int

const (
	// ReloadNone means the application has no runtime configuration.
	ReloadNone ReloadMode = iota
	// ReloadHot means the running process re-reads its configuration on demand.
	ReloadHot
	// ReloadSignal means the process reloads when signalled.
	ReloadSignal
	// ReloadRestart means the process must be restarted.
	ReloadRestart
	// ReloadNewInstancesOnly means only newly launched instances pick it up.
	ReloadNewInstancesOnly
)

func (m ReloadMode) String() string {
	switch m {
	case ReloadHot:
		return "hot"
	case ReloadSignal:
		return "signal"
	case ReloadRestart:
		return "restart"
	case ReloadNewInstancesOnly:
		return "new-instances-only"
	default:
		return "none"
	}
}

// File is one generated configuration file: which template produces it and
// where it lands inside a generation.
type File struct {
	// Template is the template name, e.g. "foot/foot.ini.tmpl".
	Template string
	// Path is the destination relative to the generation root,
	// e.g. "foot/foot.ini".
	Path string
	// Mode is the file mode; zero means 0644.
	Mode fs.FileMode
}

// FileMode returns the mode to write this file with.
func (f File) FileMode() fs.FileMode {
	if f.Mode == 0 {
		return 0o644
	}
	return f.Mode
}

// ManagedPath links a generated file to the path the application reads. Target
// is relative to the user's config directory.
type ManagedPath struct {
	// Source is the path inside a generation, e.g. "foot/foot.ini".
	Source string
	// Target is relative to ~/.config, e.g. "foot/foot.ini".
	Target string
}

// Adapter declares one application's generated configuration.
type Adapter interface {
	// Name is the component name used in config.toml and manifests.
	Name() string
	// Files are the configuration files this adapter generates.
	Files() []File
	// ConfigPaths maps generated files to application config locations.
	ConfigPaths() []ManagedPath
	// ReloadMode reports how the application picks up changes.
	ReloadMode() ReloadMode
	// Validate checks generated content before it becomes a generation.
	// dir is the generation root; a nil error means the output is usable.
	Validate(dir string) error
}

// Configurable is implemented by an adapter whose set of files depends on the
// user's configuration rather than being fixed.
//
// Most adapters generate the same files every time: Foot always writes one
// foot.ini. Toolkit integration does not, because whether a Qt 5 platform
// theme or a GTK stylesheet should exist is a decision, not a constant. Rather
// than push a configuration argument through every adapter, the few that need
// one implement this.
type Configurable interface {
	// FilesFor is Files, narrowed by the configuration.
	FilesFor(cfg config.Config) []File
	// ConfigPathsFor is ConfigPaths, narrowed by the configuration.
	ConfigPathsFor(cfg config.Config) []ManagedPath
}

// FilesOf returns the files an adapter generates for a configuration.
func FilesOf(a Adapter, cfg config.Config) []File {
	if c, ok := a.(Configurable); ok {
		return c.FilesFor(cfg)
	}
	return a.Files()
}

// ConfigPathsOf returns the paths an adapter manages for a configuration.
func ConfigPathsOf(a Adapter, cfg config.Config) []ManagedPath {
	if c, ok := a.(Configurable); ok {
		return c.ConfigPathsFor(cfg)
	}
	return a.ConfigPaths()
}

// Registry holds the available adapters by name.
type Registry struct {
	adapters map[string]Adapter
}

// NewRegistry builds a registry from a set of adapters.
func NewRegistry(adapters ...Adapter) *Registry {
	r := &Registry{adapters: make(map[string]Adapter, len(adapters))}
	for _, a := range adapters {
		r.adapters[a.Name()] = a
	}
	return r
}

// Get returns the adapter registered under name.
func (r *Registry) Get(name string) (Adapter, error) {
	a, ok := r.adapters[name]
	if !ok {
		return nil, fmt.Errorf("unknown component %q", name)
	}
	return a, nil
}

// Names returns every registered adapter name, sorted.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Select returns the adapters for the given names, in the order given.
func (r *Registry) Select(names []string) ([]Adapter, error) {
	out := make([]Adapter, 0, len(names))
	for _, name := range names {
		a, err := r.Get(name)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}
