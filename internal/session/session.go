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

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/assets"
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
	// WriteConfig persists an edited configuration. It is injected because
	// config.toml belongs to the command layer, which owns its formatting and
	// its header. A nil writer makes per-program settings read-only.
	WriteConfig func(config.Config) error
	// Version is stamped into rendered files, matching a real generation.
	Version string
	// SandboxRoot is where preview renders go. Empty means a directory under
	// the system temporary directory.
	SandboxRoot string
	// CurrentDir is the generation `current` points at. It is what the draft
	// is compared against; empty means there is nothing to compare with.
	CurrentDir string
}

// Session is a draft and everything needed to act on it.
//
// The theme inside Base and Draft is held in source form, exactly as a theme
// file is written: a field left unset is derived from others by normalization.
// Keeping that distinction is what lets an edit to a semantic color still
// reach everything derived from it. Resolved is the normalized form, and is
// what renders.
type Session struct {
	// Base is what the draft started from: the theme as loaded, and the
	// configuration as it is on disk.
	Base Draft
	// Draft is the working copy.
	Draft Draft

	themes      *theme.Store
	registry    *adapter.Registry
	builder     *generation.Builder
	runner      command.Runner
	themesDir   string
	writeConfig func(config.Config) error

	sandboxRoot string
	currentDir  string
	previews    []*Preview

	// started records that SetBase has run at least once, which is what
	// separates "no draft configuration yet" from "a draft configuration that
	// happens to be empty".
	started bool

	// resolved is Draft after normalization. It is recomputed on every change
	// rather than on every read, because an interface reads it constantly and
	// changes it rarely.
	resolved Draft

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
		themes:      opts.Themes,
		registry:    opts.Registry,
		builder:     generation.NewBuilder(opts.Engine, opts.Registry, opts.Version),
		runner:      opts.Runner,
		themesDir:   opts.ThemesDir,
		writeConfig: opts.WriteConfig,
		sandboxRoot: root,
		currentDir:  opts.CurrentDir,
	}
	s.Base.Config = opts.Config
	s.SetBase(base)
	return s, nil
}

// Config is the configuration the draft renders against.
func (s *Session) Config() config.Config { return s.Draft.Config }

// SetBase replaces the base theme and discards the draft. Choosing a different
// theme in the picker starts over rather than carrying edits across, because
// an override that made sense against one palette rarely makes sense against
// another.
// The configuration is carried across: it is not part of the theme, and
// choosing a different palette is no reason to forget a bar height.
func (s *Session) SetBase(base theme.Theme) {
	// On the first call the draft has no configuration yet, so there is
	// nothing to carry. Checking a flag rather than inspecting the
	// configuration keeps a genuinely empty one from being mistaken for it.
	cfg := s.Base.Config
	if s.started {
		cfg = s.Draft.Config
	}
	s.started = true

	s.Base = Draft{Theme: cloneTheme(base), Config: s.Base.Config}
	s.Draft = Draft{Theme: cloneTheme(base), Config: cfg}
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
func (s *Session) Resolved() Draft { return s.resolved }

// Theme is the resolved theme, which is what renders.
func (s *Session) Theme() theme.Theme { return s.resolved.Theme }

// refresh recomputes the resolved draft after a change.
func (s *Session) refresh() {
	s.resolved = Draft{
		Theme:  s.Draft.Theme.Resolved(),
		Config: s.Draft.Config,
	}
}

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
func (s *Session) Components() []string { return s.Draft.Config.Components.Names() }

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

// Effective is what a field's value resolves to, which for an unset override
// is what the theme gives rather than nothing.
func (s *Session) Effective(key string) string {
	f, ok := LookupField(key)
	if !ok {
		return ""
	}
	return f.Effective(s.resolved)
}

// Inherited reports whether a field is showing a value it does not hold: an
// override left unset, following the theme.
func (s *Session) Inherited(key string) bool {
	f, ok := LookupField(key)
	if !ok {
		return false
	}
	return f.Inherited(s.Draft)
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

// Missing reports that a field names something that is supposed to be
// installed on this machine and is not. An interface marks these: a theme
// naming an icon set nobody has renders and deploys perfectly, and the only
// symptom is that the icons never change.
//
// A field that names nothing installable is never missing.
func (s *Session) Missing(key string) bool {
	f, ok := LookupField(key)
	if !ok || !f.PicksAssets {
		return false
	}
	value := f.Display(s.resolved)
	if strings.TrimSpace(value) == "" {
		return false
	}
	return !assets.Installed(f.Assets, value)
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
	s.Draft = cloneDraft(s.Base)
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

// Overrides lists the theme keys that differ from the base theme, in field
// order. Configuration settings are not included: they are tracked separately,
// because they are saved to a different file.
func (s *Session) Overrides() []string {
	var keys []string
	for _, f := range Fields() {
		if f.Store == StoreTheme && !f.Same(s.Draft, s.Base) {
			keys = append(keys, f.Key)
		}
	}
	return keys
}

// ThemeDirty reports whether anything saved to the theme file has changed.
// It walks every field, not just the global ones: the terminal palette is part
// of the theme but is edited under the terminal.
func (s *Session) ThemeDirty() bool {
	for _, f := range EveryField() {
		if f.Store == StoreTheme && !f.Same(s.Draft, s.Base) {
			return true
		}
	}
	return false
}

// Dirty reports whether anything at all is unsaved.
func (s *Session) Dirty() bool { return s.ThemeDirty() || s.ConfigDirty() }

// Validate reports whether the draft is renderable, using the same validation
// a theme file and a configuration file go through.
func (s *Session) Validate() error {
	if err := s.resolved.Theme.Validate(); err != nil {
		return err
	}
	return s.resolved.Config.Validate()
}

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

	out := cloneTheme(s.Draft.Theme)
	out.Name = name
	if err := out.Resolved().Validate(); err != nil {
		return "", err
	}

	// The source form is written, not the resolved one, so a saved theme reads
	// like a hand-written file and its derived values stay derived.
	data, err := theme.Encode(out)
	if err != nil {
		return "", err
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

// SaveConfig writes the edited configuration back to config.toml. It is a
// no-op when nothing changed, so saving a theme after touching no program
// setting does not rewrite the file.
func (s *Session) SaveConfig() error {
	if !s.ConfigDirty() {
		return nil
	}
	if s.writeConfig == nil {
		return errors.New("per-program settings are read-only in this context")
	}

	cfg := s.Draft.Config
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := s.writeConfig(cfg); err != nil {
		return err
	}

	s.Draft.Config = cfg
	s.Base.Config = cfg
	s.refresh()
	return nil
}

// Save writes everything the session has changed: the theme under name, and
// the configuration when a program setting was edited.
func (s *Session) Save(name string) (string, error) {
	path, err := s.SaveTheme(name)
	if err != nil {
		return "", err
	}
	return path, s.SaveConfig()
}

// SetConfigWriter replaces where an edited configuration is written. It exists
// so a caller that builds a session before it knows how to persist — and a
// test — can supply the writer afterwards.
func (s *Session) SetConfigWriter(write func(config.Config) error) { s.writeConfig = write }

// ConfigDirty reports whether anything saved to config.toml has changed.
func (s *Session) ConfigDirty() bool {
	for _, f := range EveryField() {
		if f.Store == StoreConfig && !f.Same(s.Draft, s.Base) {
			return true
		}
	}
	return false
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

// cloneDraft copies a draft. The configuration's slices and maps are shared:
// the editable program settings are all scalars, and the parts that are not —
// outputs, bindings, window rules — are not editable here.
func cloneDraft(d Draft) Draft {
	d.Theme = cloneTheme(d.Theme)
	return d
}
