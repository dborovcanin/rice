package generation

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/theme"
)

// A preview is a mutable render that `current` points at directly, instead of
// at a committed generation. It exists so a theme can be lived with before it
// becomes history: trying six themes should not leave six generations behind.
//
// Nothing about a preview is committed. Its directory is rewritten in place,
// it never takes a generation number, and it is not what rollback goes back
// to. Committing one builds an ordinary generation from the same theme, and
// cancelling one puts `current` back exactly where it was.

var (
	// ErrPreviewActive is returned where a committed generation was expected
	// but a preview holds `current`.
	ErrPreviewActive = errors.New("a preview is active")
	// ErrNoPreview is returned when a preview operation finds no preview.
	ErrNoPreview = errors.New("no preview is active")
)

// previewLink is what `current` points at while a preview is active.
const previewLink = "preview"

// PreviewState is what a running preview remembers.
type PreviewState struct {
	// Theme is the theme being previewed.
	Theme string
	// Parent is the generation `current` pointed at before the preview began,
	// and where cancelling returns to. Zero means there was none.
	Parent int
}

// PreviewActive reports whether `current` points at the preview directory.
func (s *Store) PreviewActive() bool {
	target, err := os.Readlink(s.Paths.Current)
	if err != nil {
		return false
	}
	return filepath.Base(target) == previewLink
}

// Preview renders a theme into the preview directory and points `current` at
// it. It deliberately does not record a previous generation: a preview is not
// something to roll back to.
func (s *Store) Preview(cfg config.Config, th theme.Theme) (PreviewState, error) {
	if err := s.Paths.EnsureDirs(); err != nil {
		return PreviewState{}, err
	}

	state := PreviewState{Theme: th.Name}
	if active := s.PreviewActive(); !active {
		// Remember where to go back to. Starting a preview while one is
		// already running keeps the original parent, so cancel still returns
		// to the last committed generation rather than to another preview.
		if current, err := s.Current(); err == nil {
			state.Parent = current
		}
	} else {
		existing, err := s.PreviewState()
		if err != nil {
			return PreviewState{}, err
		}
		state.Parent = existing.Parent
	}

	// The render goes to a staging directory and is swapped in, so a template
	// error leaves the running preview alone rather than half-rewriting it.
	staging, err := os.MkdirTemp(s.Paths.Root, ".preview-")
	if err != nil {
		return PreviewState{}, fmt.Errorf("create preview staging directory: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			os.RemoveAll(staging)
		}
	}()

	// A preview is numbered as the generation it would become, so the headers
	// it writes match what committing it would produce.
	number, err := s.Next()
	if err != nil {
		return PreviewState{}, err
	}
	if _, err := s.Builder.Build(staging, cfg, th, number, BuildOptions{
		ThemeSource: th.Name,
		Parent:      state.Parent,
		Description: "preview",
	}); err != nil {
		return PreviewState{}, err
	}
	if err := os.Chmod(staging, 0o755); err != nil {
		return PreviewState{}, fmt.Errorf("set preview permissions: %w", err)
	}

	if err := os.RemoveAll(s.Paths.PreviewDir); err != nil {
		return PreviewState{}, fmt.Errorf("clear preview directory: %w", err)
	}
	if err := os.Rename(staging, s.Paths.PreviewDir); err != nil {
		return PreviewState{}, fmt.Errorf("install preview: %w", err)
	}
	committed = true

	if err := s.writePreviewState(state); err != nil {
		return PreviewState{}, err
	}
	if err := replaceSymlink(s.Paths.Current, previewLink); err != nil {
		return PreviewState{}, err
	}
	return state, nil
}

// CommitPreview turns the running preview into an ordinary generation. The
// generation is built fresh from the same theme rather than by promoting the
// preview directory: rendering is deterministic, and a real build gets a real
// generation number in the files it writes.
func (s *Store) CommitPreview(cfg config.Config, th theme.Theme, opts BuildOptions) (Info, error) {
	if !s.PreviewActive() {
		return Info{}, ErrNoPreview
	}

	state, err := s.PreviewState()
	if err != nil {
		return Info{}, err
	}
	if opts.Parent == 0 {
		opts.Parent = state.Parent
	}
	if opts.ThemeSource == "" {
		opts.ThemeSource = th.Name
	}

	info, err := s.Create(cfg, th, opts)
	if err != nil {
		return Info{}, err
	}

	// Point at the new generation directly. Switch would read `current`,
	// which is still the preview, and record it as the previous generation.
	target := filepath.Join("generations", config.FormatGeneration(info.Number))
	if err := replaceSymlink(s.Paths.Current, target); err != nil {
		return Info{}, err
	}
	if state.Parent != 0 {
		if err := s.writePrevious(state.Parent); err != nil {
			return Info{}, err
		}
	}

	if err := s.clearPreview(); err != nil {
		return Info{}, err
	}
	return info, nil
}

// CancelPreview puts `current` back where it was and removes the preview.
func (s *Store) CancelPreview() (PreviewState, error) {
	if !s.PreviewActive() {
		return PreviewState{}, ErrNoPreview
	}

	state, err := s.PreviewState()
	if err != nil {
		return PreviewState{}, err
	}

	if state.Parent == 0 {
		// A preview started before anything was committed has nowhere to go
		// back to. Removing the link is the honest undo.
		if err := os.Remove(s.Paths.Current); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return PreviewState{}, fmt.Errorf("remove current link: %w", err)
		}
	} else {
		dir := s.Paths.Generation(state.Parent)
		if _, err := os.Stat(dir); err != nil {
			return PreviewState{}, fmt.Errorf("generation %d is gone, cannot cancel: %w", state.Parent, err)
		}
		target := filepath.Join("generations", config.FormatGeneration(state.Parent))
		if err := replaceSymlink(s.Paths.Current, target); err != nil {
			return PreviewState{}, err
		}
	}

	if err := s.clearPreview(); err != nil {
		return PreviewState{}, err
	}
	return state, nil
}

// PreviewState reads what the running preview remembers. A preview whose state
// file is missing — a crash between the render and the write — still cancels,
// with no parent to return to.
func (s *Store) PreviewState() (PreviewState, error) {
	data, err := os.ReadFile(s.previewStateFile())
	if errors.Is(err, fs.ErrNotExist) {
		return PreviewState{}, nil
	}
	if err != nil {
		return PreviewState{}, fmt.Errorf("read preview state: %w", err)
	}

	var state PreviewState
	for line := range strings.SplitSeq(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "theme":
			state.Theme = value
		case "parent":
			state.Parent, _ = strconv.Atoi(value)
		}
	}
	return state, nil
}

func (s *Store) writePreviewState(state PreviewState) error {
	if err := os.MkdirAll(s.Paths.StateDir, 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	body := fmt.Sprintf("theme=%s\nparent=%d\n", state.Theme, state.Parent)
	if err := os.WriteFile(s.previewStateFile(), []byte(body), 0o644); err != nil {
		return fmt.Errorf("record preview state: %w", err)
	}
	return nil
}

// clearPreview removes the preview directory and its state.
func (s *Store) clearPreview() error {
	if err := os.RemoveAll(s.Paths.PreviewDir); err != nil {
		return fmt.Errorf("remove preview: %w", err)
	}
	if err := os.Remove(s.previewStateFile()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove preview state: %w", err)
	}
	return nil
}

func (s *Store) previewStateFile() string {
	return filepath.Join(s.Paths.StateDir, "preview")
}

func (s *Store) writePrevious(number int) error {
	if err := os.MkdirAll(s.Paths.StateDir, 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	if err := os.WriteFile(s.previousFile(), []byte(strconv.Itoa(number)+"\n"), 0o644); err != nil {
		return fmt.Errorf("record previous generation: %w", err)
	}
	return nil
}
