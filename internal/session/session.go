// Package session holds the state of an interactive editing session: a draft
// theme derived from a base theme, the operations that change it, and the ways
// a draft leaves the session — rendered to a sandbox, copied to the clipboard,
// or saved as a user theme.
//
// The package is deliberately free of any interface concern. A terminal
// interface, and one day possibly a graphical one, are views over this; the
// editing logic itself lives here so it can be tested without a terminal.
package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/command"
	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/generation"
	"github.com/dborovcanin/rice/internal/render"
	"github.com/dborovcanin/rice/internal/theme"
)

// Options are the collaborators a session needs. Everything is an existing
// Rice package: the session adds no parallel implementation of theming,
// rendering or validation.
type Options struct {
	Themes   *theme.Store
	Registry *adapter.Registry
	Engine   *render.Engine
	Runner   command.Runner
	Config   config.Config

	// ThemesDir is where a saved draft is written, normally
	// ~/.config/rice/themes.
	ThemesDir string
	// Version is stamped into rendered files, matching a real generation.
	Version string
	// SandboxRoot is where preview renders go. Empty means a directory under
	// the system temporary directory.
	SandboxRoot string
}

// Session is a draft theme and everything needed to act on it.
//
// Base and Draft are held in source form, exactly as a theme file is written:
// a field left unset is derived from others by normalization. Keeping that
// distinction is what lets an edit to a semantic color still reach everything
// derived from it. Resolved is the normalized form, and is what renders.
type Session struct {
	// Base is the theme the draft started from, in source form.
	Base theme.Theme
	// Draft is the working copy, in source form.
	Draft theme.Theme
	// Config is the structural configuration the draft renders against. The
	// session does not edit it; structure stays in config.toml.
	Config config.Config

	themes    *theme.Store
	registry  *adapter.Registry
	builder   *generation.Builder
	runner    command.Runner
	themesDir string

	sandboxRoot string
	previews    []*Preview

	// resolved is Draft after normalization. It is recomputed on every change
	// rather than on every read, because an interface reads it constantly and
	// changes it rarely.
	resolved theme.Theme

	// previews caches other themes resolved for display, so a picker that
	// draws a palette for every row does not re-read and re-parse the theme
	// files on every keystroke.
	previewCache map[string]theme.Theme
}

// New builds a session over a theme in source form, as returned by
// theme.Store.LoadSource.
func New(base theme.Theme, opts Options) (*Session, error) {
	switch {
	case opts.Themes == nil:
		return nil, errors.New("session: no theme store")
	case opts.Registry == nil:
		return nil, errors.New("session: no adapter registry")
	case opts.Engine == nil:
		return nil, errors.New("session: no render engine")
	case opts.Runner == nil:
		return nil, errors.New("session: no command runner")
	}

	root := opts.SandboxRoot
	if root == "" {
		root = filepath.Join(os.TempDir(), "rice-preview")
	}

	s := &Session{
		Config:      opts.Config,
		themes:      opts.Themes,
		registry:    opts.Registry,
		builder:     generation.NewBuilder(opts.Engine, opts.Registry, opts.Version),
		runner:      opts.Runner,
		themesDir:   opts.ThemesDir,
		sandboxRoot: root,
	}
	s.SetBase(base)
	return s, nil
}

// SetBase replaces the base theme and discards the draft. Choosing a different
// theme in the picker starts over rather than carrying edits across, because
// an override that made sense against one palette rarely makes sense against
// another.
func (s *Session) SetBase(base theme.Theme) {
	s.Base = cloneTheme(base)
	s.Draft = cloneTheme(base)
	s.refresh()

	// A saved theme changes what its name resolves to, so the cache cannot
	// outlive a rebase.
	delete(s.previewCache, base.Name)
}

// LoadBase resolves a theme by name and makes it the base.
func (s *Session) LoadBase(name string) error {
	th, err := s.themes.LoadSource(name)
	if err != nil {
		return err
	}
	s.SetBase(th)
	return nil
}

// Resolved is the draft after normalization: every derived value filled in,
// and what every render, preview and export uses.
func (s *Session) Resolved() theme.Theme { return s.resolved }

// refresh recomputes the resolved theme after the draft changes.
func (s *Session) refresh() { s.resolved = s.Draft.Resolved() }

// Themes lists the themes available to start from.
func (s *Session) Themes() ([]theme.Entry, error) { return s.themes.List() }

// ThemePreview resolves another theme by name, for showing its palette without
// making it the base. Results are cached for the life of the session.
func (s *Session) ThemePreview(name string) (theme.Theme, error) {
	if th, ok := s.previewCache[name]; ok {
		return th, nil
	}
	th, err := s.themes.Load(name)
	if err != nil {
		return theme.Theme{}, err
	}
	if s.previewCache == nil {
		s.previewCache = map[string]theme.Theme{}
	}
	s.previewCache[name] = th
	return th, nil
}

// Components are the enabled component names, in registry order.
func (s *Session) Components() []string { return s.Config.Components.Names() }

// Set writes a raw value into the draft. The draft is unchanged when the value
// does not parse.
func (s *Session) Set(key, raw string) error {
	f, ok := LookupField(key)
	if !ok {
		return fmt.Errorf("unknown field %q", key)
	}
	if err := f.Set(&s.Draft, raw); err != nil {
		return err
	}
	s.refresh()
	return nil
}

// Get renders a field's effective value: what the theme actually renders with,
// derived values included.
func (s *Session) Get(key string) (string, error) {
	f, ok := LookupField(key)
	if !ok {
		return "", fmt.Errorf("unknown field %q", key)
	}
	return f.Display(s.resolved), nil
}

// Color returns a field's effective color, and false when it is not a color
// field. An interface uses this to draw swatches.
func (s *Session) Color(key string) (theme.Color, bool) {
	f, ok := LookupField(key)
	if !ok {
		return theme.Color{}, false
	}
	return f.Color(s.resolved)
}

// Nudge moves a numeric field by steps, or a color's lightness. Nudging a
// derived value makes it explicit at the value that was on screen.
func (s *Session) Nudge(key string, steps int) error {
	f, ok := LookupField(key)
	if !ok {
		return fmt.Errorf("unknown field %q", key)
	}
	if err := f.Nudge(&s.Draft, s.resolved, steps); err != nil {
		return err
	}
	s.refresh()
	return nil
}

// Reset returns one field to its value in the base theme, which may mean
// returning it to being derived.
func (s *Session) Reset(key string) error {
	f, ok := LookupField(key)
	if !ok {
		return fmt.Errorf("unknown field %q", key)
	}
	f.CopyFrom(&s.Draft, s.Base)
	s.refresh()
	return nil
}

// Clear drops a field's explicit value so it is derived again, following
// whatever it is derived from. Unlike Reset, this can remove a value the base
// theme spelled out.
func (s *Session) Clear(key string) error {
	f, ok := LookupField(key)
	if !ok {
		return fmt.Errorf("unknown field %q", key)
	}
	f.Clear(&s.Draft)
	s.refresh()
	return nil
}

// Explicit reports whether the draft spells a field out rather than deriving
// it. An interface shows this, because it decides whether editing a related
// field will move this one too.
func (s *Session) Explicit(key string) bool {
	f, ok := LookupField(key)
	if !ok {
		return false
	}
	return f.Explicit(s.Draft)
}

// ResetAll discards every override.
func (s *Session) ResetAll() {
	s.Draft = cloneTheme(s.Base)
	s.refresh()
}

// Overridden reports whether a field differs from the base theme.
func (s *Session) Overridden(key string) bool {
	f, ok := LookupField(key)
	if !ok {
		return false
	}
	return !f.Same(s.Draft, s.Base)
}

// Overrides lists the keys that differ from the base theme, in field order.
func (s *Session) Overrides() []string {
	var keys []string
	for _, f := range Fields() {
		if !f.Same(s.Draft, s.Base) {
			keys = append(keys, f.Key)
		}
	}
	return keys
}

// Dirty reports whether the draft differs from the base theme at all.
func (s *Session) Dirty() bool { return len(s.Overrides()) > 0 }

// Validate reports whether the draft is renderable, using the same validation
// a theme file goes through.
func (s *Session) Validate() error { return s.resolved.Validate() }

// SaveTheme writes the draft to the user theme directory and returns the path.
// A user theme shadows a bundled theme of the same name, which is the existing
// rule and is how a bundled theme is customized in place.
func (s *Session) SaveTheme(name string) (string, error) {
	name = strings.TrimSpace(name)
	if err := ValidThemeName(name); err != nil {
		return "", err
	}
	if s.themesDir == "" {
		return "", errors.New("no theme directory configured")
	}

	out := cloneTheme(s.Draft)
	out.Name = name
	if err := out.Resolved().Validate(); err != nil {
		return "", err
	}

	// The source form is written, not the resolved one, so a saved theme reads
	// like a hand-written file and its derived values stay derived.
	data, err := toml.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("encode theme: %w", err)
	}

	if err := os.MkdirAll(s.themesDir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", s.themesDir, err)
	}
	path := filepath.Join(s.themesDir, name+".toml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}

	// The draft is now a theme in its own right, so further edits are measured
	// against what was saved.
	s.SetBase(out)
	return path, nil
}

// ValidThemeName rejects names that would escape the theme directory or
// produce a file the theme store cannot find again.
func ValidThemeName(name string) error {
	switch {
	case name == "":
		return errors.New("theme name is empty")
	case strings.ContainsAny(name, `/\`):
		return errors.New("theme name may not contain a path separator")
	case strings.HasPrefix(name, "."):
		return errors.New("theme name may not start with a dot")
	case name != filepath.Clean(name):
		return fmt.Errorf("theme name %q is not a plain file name", name)
	case strings.HasSuffix(name, ".toml"):
		return errors.New("theme name should not include the .toml extension")
	}
	return nil
}

// Close releases everything the session started: any preview still running is
// killed and every sandbox directory is removed.
func (s *Session) Close() error {
	var errs []error
	for _, p := range s.previews {
		errs = append(errs, p.Stop())
	}
	s.previews = nil

	// Only remove the root when the session owns it exclusively; a shared
	// temporary root may hold another session's previews.
	if entries, err := os.ReadDir(s.sandboxRoot); err == nil && len(entries) == 0 {
		errs = append(errs, os.Remove(s.sandboxRoot))
	}
	return errors.Join(errs...)
}

// cloneTheme copies a theme deeply enough that editing one does not change the
// other. Only Icons.Paths is a reference type.
func cloneTheme(t theme.Theme) theme.Theme {
	t.Icons.Paths = slices.Clone(t.Icons.Paths)
	return t
}
