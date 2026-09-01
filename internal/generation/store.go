package generation

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/theme"
)

// ErrNoGenerations is returned when the store holds nothing yet.
var ErrNoGenerations = errors.New("no generations")

// Store owns the generations directory and the `current` symlink. Committed
// generations are immutable: the store only ever adds, links or removes them.
type Store struct {
	Paths   config.Paths
	Builder *Builder
}

// NewStore returns a store over a Rice root and a builder.
func NewStore(paths config.Paths, builder *Builder) *Store {
	return &Store{Paths: paths, Builder: builder}
}

// Info pairs a generation number with its manifest.
type Info struct {
	Number   int
	Dir      string
	Manifest Manifest
}

// List returns every committed generation, oldest first.
func (s *Store) List() ([]Info, error) {
	entries, err := os.ReadDir(s.Paths.GenerationsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read generations: %w", err)
	}

	var out []Info
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n, err := parseGeneration(e.Name())
		if err != nil {
			continue
		}
		dir := filepath.Join(s.Paths.GenerationsDir, e.Name())
		m, err := ReadManifest(dir)
		if err != nil {
			// A generation without a readable manifest is incomplete; report
			// it rather than hiding it, so `doctor` can flag it.
			m = Manifest{Generation: n}
		}
		out = append(out, Info{Number: n, Dir: dir, Manifest: m})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

// Next returns the number the next generation will get.
func (s *Store) Next() (int, error) {
	list, err := s.List()
	if err != nil {
		return 0, err
	}
	if len(list) == 0 {
		return 1, nil
	}
	return list[len(list)-1].Number + 1, nil
}

// Current returns the generation `current` points at.
func (s *Store) Current() (int, error) {
	target, err := os.Readlink(s.Paths.Current)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, ErrNoGenerations
	}
	if err != nil {
		return 0, fmt.Errorf("read current link: %w", err)
	}
	return parseGeneration(filepath.Base(target))
}

// Previous returns the generation `current` pointed at before the last switch.
func (s *Store) Previous() (int, error) {
	data, err := os.ReadFile(s.previousFile())
	if errors.Is(err, fs.ErrNotExist) {
		return 0, ErrNoGenerations
	}
	if err != nil {
		return 0, fmt.Errorf("read previous generation: %w", err)
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// Create builds a new generation and commits it. The generation directory is
// assembled under a temporary name and renamed into place, so a failed build
// never leaves a half-written generation behind.
func (s *Store) Create(cfg config.Config, th theme.Theme, opts BuildOptions) (Info, error) {
	if err := s.Paths.EnsureDirs(); err != nil {
		return Info{}, err
	}

	number, err := s.Next()
	if err != nil {
		return Info{}, err
	}
	if opts.Parent == 0 {
		if current, err := s.Current(); err == nil {
			opts.Parent = current
		}
	}

	staging, err := os.MkdirTemp(s.Paths.GenerationsDir, ".build-")
	if err != nil {
		return Info{}, fmt.Errorf("create staging directory: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			os.RemoveAll(staging)
		}
	}()

	manifest, err := s.Builder.Build(staging, cfg, th, number, opts)
	if err != nil {
		return Info{}, err
	}
	if err := os.Chmod(staging, 0o755); err != nil {
		return Info{}, fmt.Errorf("set generation permissions: %w", err)
	}

	final := s.Paths.Generation(number)
	if _, err := os.Lstat(final); err == nil {
		return Info{}, fmt.Errorf("generation %d already exists", number)
	}
	if err := os.Rename(staging, final); err != nil {
		return Info{}, fmt.Errorf("finalize generation %d: %w", number, err)
	}
	committed = true

	return Info{Number: number, Dir: final, Manifest: manifest}, nil
}

// Switch points `current` at a generation, recording the outgoing generation
// so rollback has somewhere to go back to.
func (s *Store) Switch(number int) error {
	dir := s.Paths.Generation(number)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("generation %d: %w", number, err)
	}

	previous, hadPrevious := 0, false
	if current, err := s.Current(); err == nil {
		if current == number {
			return nil
		}
		previous, hadPrevious = current, true
	}

	target := filepath.Join("generations", config.FormatGeneration(number))
	if err := replaceSymlink(s.Paths.Current, target); err != nil {
		return err
	}

	if hadPrevious {
		if err := os.MkdirAll(s.Paths.StateDir, 0o755); err != nil {
			return fmt.Errorf("create state directory: %w", err)
		}
		if err := os.WriteFile(s.previousFile(), []byte(strconv.Itoa(previous)+"\n"), 0o644); err != nil {
			return fmt.Errorf("record previous generation: %w", err)
		}
	}
	return nil
}

// Prune deletes old generations, keeping the newest keep of them. The current
// and previous generations are never removed regardless of the limit.
func (s *Store) Prune(keep int) ([]int, error) {
	if keep <= 0 {
		return nil, nil
	}
	list, err := s.List()
	if err != nil {
		return nil, err
	}
	if len(list) <= keep {
		return nil, nil
	}

	protected := map[int]bool{}
	if current, err := s.Current(); err == nil {
		protected[current] = true
	}
	if previous, err := s.Previous(); err == nil {
		protected[previous] = true
	}

	var removed []int
	for _, info := range list[:len(list)-keep] {
		if protected[info.Number] {
			continue
		}
		if err := os.RemoveAll(info.Dir); err != nil {
			return removed, fmt.Errorf("remove generation %d: %w", info.Number, err)
		}
		removed = append(removed, info.Number)
	}
	return removed, nil
}

func (s *Store) previousFile() string {
	return filepath.Join(s.Paths.StateDir, "previous")
}

// replaceSymlink atomically points link at target by creating a sibling
// symlink and renaming it over the old one.
func replaceSymlink(link, target string) error {
	dir := filepath.Dir(link)
	tmp, err := os.MkdirTemp(dir, ".link-")
	if err != nil {
		return fmt.Errorf("create link staging directory: %w", err)
	}
	defer os.RemoveAll(tmp)

	staged := filepath.Join(tmp, "current")
	if err := os.Symlink(target, staged); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}
	if err := os.Rename(staged, link); err != nil {
		return fmt.Errorf("switch symlink %s: %w", link, err)
	}
	return nil
}

func parseGeneration(name string) (int, error) {
	n, err := strconv.Atoi(name)
	if err != nil {
		return 0, fmt.Errorf("not a generation directory: %q", name)
	}
	return n, nil
}
