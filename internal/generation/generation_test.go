package generation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/render"
	"github.com/dborovcanin/rice/internal/theme"
)

// fakeAdapter renders one file and fails validation on demand, which is what
// the store's failure paths need to be exercised.
type fakeAdapter struct {
	name    string
	invalid bool
}

func (f fakeAdapter) Name() string { return f.name }

func (f fakeAdapter) Files() []adapter.File {
	return []adapter.File{{Template: f.name + ".tmpl", Path: f.name + "/config"}}
}

func (f fakeAdapter) ConfigPaths() []adapter.ManagedPath {
	return []adapter.ManagedPath{{Source: f.name + "/config", Target: f.name + "/config"}}
}

func (f fakeAdapter) ReloadMode() adapter.ReloadMode { return adapter.ReloadHot }

func (f fakeAdapter) Validate(dir string) error {
	if f.invalid {
		return &validationFailure{component: f.name}
	}
	return nil
}

type validationFailure struct{ component string }

func (e *validationFailure) Error() string { return "validate " + e.component + ": broken" }

func newTestStore(t *testing.T, invalid bool) (*Store, config.Config, theme.Theme) {
	t.Helper()

	paths := config.NewPaths(t.TempDir())
	if err := paths.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	templates := fstest.MapFS{
		"templates/sway.tmpl": &fstest.MapFile{
			Data: []byte("theme={{ .Theme.Name }}\ngeneration={{ .Generation }}\n"),
		},
		"templates/foot.tmpl": &fstest.MapFile{
			Data: []byte("background={{ bare .Colors.Background }}\n"),
		},
	}
	engine := render.NewEngine("", templates, "templates")
	registry := adapter.NewRegistry(
		fakeAdapter{name: "sway", invalid: invalid},
		fakeAdapter{name: "foot"},
	)
	builder := NewBuilder(engine, registry, "test")
	builder.Now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }

	cfg := config.DefaultConfig()
	cfg.Components = config.Components{Sway: true, Foot: true}
	cfg.Normalize()

	th, err := theme.Parse([]byte("name=\"t\"\n[colors]\nbackground=\"#282828\"\nforeground=\"#ebdbb2\"\nprimary=\"#d79921\"\n"), "t.toml")
	if err != nil {
		t.Fatal(err)
	}

	return NewStore(paths, builder), cfg, th
}

func TestCreateAndSwitch(t *testing.T) {
	store, cfg, th := newTestStore(t, false)

	first, err := store.Create(cfg, th, BuildOptions{Description: "first"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if first.Number != 1 {
		t.Errorf("first generation = %d, want 1", first.Number)
	}

	data, err := os.ReadFile(filepath.Join(first.Dir, "sway", "config"))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if !strings.Contains(string(data), "generation=1") {
		t.Errorf("generated file = %q", data)
	}

	if _, err := store.Current(); err == nil {
		t.Error("current should not exist before the first switch")
	}
	if err := store.Switch(first.Number); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	current, err := store.Current()
	if err != nil || current != 1 {
		t.Fatalf("Current() = %d, %v", current, err)
	}

	second, err := store.Create(cfg, th, BuildOptions{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if second.Number != 2 {
		t.Errorf("second generation = %d, want 2", second.Number)
	}
	if second.Manifest.Parent != 1 {
		t.Errorf("parent = %d, want 1", second.Manifest.Parent)
	}
	if err := store.Switch(second.Number); err != nil {
		t.Fatalf("Switch: %v", err)
	}

	previous, err := store.Previous()
	if err != nil || previous != 1 {
		t.Fatalf("Previous() = %d, %v", previous, err)
	}

	// Rollback is just another switch, and it must be exact.
	if err := store.Switch(previous); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	current, err = store.Current()
	if err != nil || current != 1 {
		t.Fatalf("after rollback Current() = %d, %v", current, err)
	}
	if prev, err := store.Previous(); err != nil || prev != 2 {
		t.Fatalf("after rollback Previous() = %d, %v", prev, err)
	}
}

func TestCurrentSymlinkIsRelative(t *testing.T) {
	store, cfg, th := newTestStore(t, false)
	info, err := store.Create(cfg, th, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Switch(info.Number); err != nil {
		t.Fatal(err)
	}

	target, err := os.Readlink(store.Paths.Current)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.IsAbs(target) {
		t.Errorf("current -> %q, want a relative target so the tree stays movable", target)
	}
	if target != filepath.Join("generations", "000001") {
		t.Errorf("current -> %q", target)
	}
}

func TestSwitchToSameGenerationIsNoop(t *testing.T) {
	store, cfg, th := newTestStore(t, false)
	info, err := store.Create(cfg, th, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Switch(info.Number); err != nil {
		t.Fatal(err)
	}
	if err := store.Switch(info.Number); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Previous(); err == nil {
		t.Error("switching to the current generation should not record a previous")
	}
}

func TestSwitchToMissingGeneration(t *testing.T) {
	store, _, _ := newTestStore(t, false)
	if err := store.Switch(42); err == nil {
		t.Fatal("want error")
	}
}

func TestFailedBuildLeavesNothingBehind(t *testing.T) {
	store, cfg, th := newTestStore(t, true)

	if _, err := store.Create(cfg, th, BuildOptions{}); err == nil {
		t.Fatal("want validation error")
	}

	entries, err := os.ReadDir(store.Paths.GenerationsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("failed build left %d entries behind", len(entries))
	}
	if next, err := store.Next(); err != nil || next != 1 {
		t.Errorf("Next() = %d, %v; a failed build must not consume a number", next, err)
	}
}

func TestManifestRecordsFilesAndHashes(t *testing.T) {
	store, cfg, th := newTestStore(t, false)
	info, err := store.Create(cfg, th, BuildOptions{Description: "notes"})
	if err != nil {
		t.Fatal(err)
	}

	m, err := ReadManifest(info.Dir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if m.Generation != 1 || m.Theme != "t" || m.Description != "notes" || m.RiceVersion != "test" {
		t.Errorf("manifest = %+v", m)
	}
	if len(m.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(m.Files))
	}
	if m.Files[0].Path != "foot/config" || m.Files[1].Path != "sway/config" {
		t.Errorf("files should be sorted by path, got %+v", m.Files)
	}
	for _, f := range m.Files {
		if len(f.SHA256) != 64 {
			t.Errorf("file %s has no content hash", f.Path)
		}
		if f.Reload != "hot" {
			t.Errorf("file %s reload = %q", f.Path, f.Reload)
		}
	}
	if !m.CreatedAt.Equal(time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("created = %v", m.CreatedAt)
	}
}

func TestPruneKeepsCurrentAndPrevious(t *testing.T) {
	store, cfg, th := newTestStore(t, false)

	for i := 0; i < 5; i++ {
		info, err := store.Create(cfg, th, BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Switch(info.Number); err != nil {
			t.Fatal(err)
		}
	}

	// Roll back so the current generation is an old one that must survive.
	if err := store.Switch(1); err != nil {
		t.Fatal(err)
	}

	removed, err := store.Prune(2)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(removed) != 2 {
		t.Errorf("removed %v, want 2 generations", removed)
	}

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	kept := map[int]bool{}
	for _, info := range list {
		kept[info.Number] = true
	}
	if !kept[1] {
		t.Error("current generation was pruned")
	}
	if !kept[5] {
		t.Error("previous generation was pruned")
	}
	if kept[2] || kept[3] {
		t.Errorf("old generations survived: %v", kept)
	}
}

func TestPruneNoopBelowLimit(t *testing.T) {
	store, cfg, th := newTestStore(t, false)
	if _, err := store.Create(cfg, th, BuildOptions{}); err != nil {
		t.Fatal(err)
	}
	removed, err := store.Prune(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Errorf("removed %v, want none", removed)
	}
}

func TestListIgnoresNonGenerationDirs(t *testing.T) {
	store, cfg, th := newTestStore(t, false)
	if _, err := store.Create(cfg, th, BuildOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(store.Paths.GenerationsDir, "scratch"), 0o755); err != nil {
		t.Fatal(err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("List() = %d entries, want 1", len(list))
	}
}
