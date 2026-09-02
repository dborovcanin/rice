package theme

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNotFound is returned when no theme with the requested name exists.
var ErrNotFound = errors.New("theme not found")

// Source says where a theme came from, which matters because a user theme
// shadows a bundled one with the same name.
type Source string

const (
	SourceUser    Source = "user"
	SourceBuiltin Source = "builtin"
)

// Entry is a theme available to load, without the cost of parsing it.
type Entry struct {
	Name   string
	Path   string
	Source Source
}

// Store resolves theme names against the user's theme directory first and the
// bundled themes second.
type Store struct {
	// UserDir is searched first; it may not exist.
	UserDir string
	// Builtin holds the bundled themes, rooted at BuiltinRoot.
	Builtin fs.FS
	// BuiltinRoot is the directory inside Builtin holding *.toml themes.
	BuiltinRoot string
}

// NewStore builds a store over a user theme directory and an embedded theme FS.
func NewStore(userDir string, builtin fs.FS, builtinRoot string) *Store {
	return &Store{UserDir: userDir, Builtin: builtin, BuiltinRoot: builtinRoot}
}

// List returns every available theme, sorted by name, with user themes
// shadowing bundled themes of the same name.
func (s *Store) List() ([]Entry, error) {
	seen := map[string]Entry{}

	if s.Builtin != nil {
		names, err := fs.Glob(s.Builtin, filepath.Join(s.BuiltinRoot, "*.toml"))
		if err != nil {
			return nil, fmt.Errorf("list builtin themes: %w", err)
		}
		for _, path := range names {
			name := NameFromPath(path)
			seen[name] = Entry{Name: name, Path: path, Source: SourceBuiltin}
		}
	}

	if s.UserDir != "" {
		names, err := filepath.Glob(filepath.Join(s.UserDir, "*.toml"))
		if err != nil {
			return nil, fmt.Errorf("list user themes: %w", err)
		}
		for _, path := range names {
			name := NameFromPath(path)
			seen[name] = Entry{Name: name, Path: path, Source: SourceUser}
		}
	}

	out := make([]Entry, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Load resolves a theme by name, or by path when the name looks like one, and
// returns it normalized and validated.
func (s *Store) Load(name string) (Theme, error) {
	data, path, err := s.read(name)
	if err != nil {
		return Theme{}, err
	}
	return Parse(data, path)
}

// LoadSource is Load without normalization, returning the theme exactly as it
// is written. Editors want this form so that derived values stay derived.
func (s *Store) LoadSource(name string) (Theme, error) {
	data, path, err := s.read(name)
	if err != nil {
		return Theme{}, err
	}
	return ParseSource(data, path)
}

// read resolves a theme name to its bytes: a path if it looks like one, then
// the user directory, then the bundled themes.
func (s *Store) read(name string) ([]byte, string, error) {
	if looksLikePath(name) {
		data, err := os.ReadFile(name)
		if err != nil {
			return nil, "", fmt.Errorf("read theme: %w", err)
		}
		return data, name, nil
	}

	if s.UserDir != "" {
		path := filepath.Join(s.UserDir, name+".toml")
		data, err := os.ReadFile(path)
		if err == nil {
			return data, path, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("read theme %s: %w", path, err)
		}
	}

	if s.Builtin != nil {
		path := filepath.Join(s.BuiltinRoot, name+".toml")
		data, err := fs.ReadFile(s.Builtin, path)
		if err == nil {
			return data, path, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("read builtin theme %s: %w", path, err)
		}
	}

	return nil, "", fmt.Errorf("%w: %s", ErrNotFound, name)
}

func looksLikePath(name string) bool {
	return strings.ContainsRune(name, filepath.Separator) || strings.HasSuffix(name, ".toml")
}
