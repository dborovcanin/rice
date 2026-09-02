package ownership

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/config"
)

// fakeAdapter declares one configuration path, which is all the ownership
// machinery cares about.
type fakeAdapter struct {
	name   string
	source string
	target string
}

func (f fakeAdapter) Name() string                   { return f.name }
func (f fakeAdapter) Files() []adapter.File          { return nil }
func (f fakeAdapter) ReloadMode() adapter.ReloadMode { return adapter.ReloadHot }
func (f fakeAdapter) Validate(string) error          { return nil }

func (f fakeAdapter) ConfigPaths() []adapter.ManagedPath {
	return []adapter.ManagedPath{{Source: f.source, Target: f.target}}
}

type fixture struct {
	paths     config.Paths
	configDir string
	adapters  []adapter.Adapter
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	root := t.TempDir()
	paths := config.NewPaths(root)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	// A current generation the links can point at.
	gen := paths.Generation(1)
	for _, rel := range []string{"foot/foot.ini", "sway/config"} {
		dest := filepath.Join(gen, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, []byte("generated "+rel+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join("generations", "000001"), paths.Current); err != nil {
		t.Fatal(err)
	}

	return fixture{
		paths:     paths,
		configDir: t.TempDir(),
		adapters: []adapter.Adapter{
			fakeAdapter{name: "foot", source: "foot/foot.ini", target: "foot/foot.ini"},
			fakeAdapter{name: "sway", source: "sway/config", target: "sway/config"},
		},
	}
}

func (f fixture) target(rel string) string {
	return filepath.Join(f.configDir, filepath.FromSlash(rel))
}

func (f fixture) writeTarget(t *testing.T, rel, content string) string {
	t.Helper()
	path := f.target(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func (f fixture) plan(t *testing.T) Plan {
	t.Helper()
	plan, err := BuildPlan(f.adapters, config.DefaultConfig(), f.paths, f.configDir)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	return plan
}

func fixedNow() time.Time { return time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC) }

func TestDetect(t *testing.T) {
	root := t.TempDir()
	dir := t.TempDir()

	regular := filepath.Join(dir, "regular")
	if err := os.WriteFile(regular, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	insideRice := filepath.Join(root, "current", "foot.ini")
	if err := os.MkdirAll(filepath.Dir(insideRice), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(insideRice, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	riceLink := filepath.Join(dir, "rice-link")
	if err := os.Symlink(insideRice, riceLink); err != nil {
		t.Fatal(err)
	}
	danglingRiceLink := filepath.Join(dir, "dangling-rice-link")
	if err := os.Symlink(filepath.Join(root, "current", "gone.ini"), danglingRiceLink); err != nil {
		t.Fatal(err)
	}
	externalLink := filepath.Join(dir, "external-link")
	if err := os.Symlink(regular, externalLink); err != nil {
		t.Fatal(err)
	}
	brokenLink := filepath.Join(dir, "broken-link")
	if err := os.Symlink(filepath.Join(dir, "nowhere"), brokenLink); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want State
	}{
		{"missing", filepath.Join(dir, "nothing"), Missing},
		{"regular file", regular, RegularFile},
		{"directory", subdir, Directory},
		{"symlink into rice", riceLink, RiceManaged},
		{"dangling link into rice is still ours", danglingRiceLink, RiceManaged},
		{"symlink elsewhere", externalLink, ExternalSymlink},
		{"broken symlink", brokenLink, BrokenSymlink},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Detect(tt.path, root)
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if got.State != tt.want {
				t.Errorf("Detect(%s) = %v, want %v", tt.name, got.State, tt.want)
			}
		})
	}
}

func TestDetectResolvesRelativeLinks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "target"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(dir, "link")
	if err := os.Symlink("../target", link); err != nil {
		t.Fatal(err)
	}

	status, err := Detect(link, root)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != RiceManaged {
		t.Errorf("state = %v, want RiceManaged", status.State)
	}
	if status.LinkTarget != filepath.Join(root, "target") {
		t.Errorf("LinkTarget = %q", status.LinkTarget)
	}
}

func TestBuildPlanClassifies(t *testing.T) {
	f := newFixture(t)
	f.writeTarget(t, "foot/foot.ini", "original\n")

	plan := f.plan(t)
	if len(plan.Actions) != 2 {
		t.Fatalf("actions = %d, want 2", len(plan.Actions))
	}

	byTarget := map[string]Action{}
	for _, a := range plan.Actions {
		byTarget[a.Target] = a
	}
	if got := byTarget[f.target("foot/foot.ini")].Kind; got != KindAdopt {
		t.Errorf("existing file = %v, want adopt", got)
	}
	if got := byTarget[f.target("sway/config")].Kind; got != KindLink {
		t.Errorf("missing file = %v, want link", got)
	}
	if len(plan.Adoptions()) != 1 {
		t.Errorf("adoptions = %d, want 1", len(plan.Adoptions()))
	}
	if plan.Empty() {
		t.Error("plan should not be empty")
	}
}

func TestPlanIsInert(t *testing.T) {
	f := newFixture(t)
	path := f.writeTarget(t, "foot/foot.ini", "original\n")

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f.plan(t)

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("building a plan modified a file")
	}
	if _, err := os.Lstat(f.target("sway/config")); !os.IsNotExist(err) {
		t.Error("building a plan created a file")
	}
}

func TestApplyAdoptsAndBacksUp(t *testing.T) {
	f := newFixture(t)
	original := "the user's own configuration\n"
	target := f.writeTarget(t, "foot/foot.ini", original)

	result, err := Apply(f.plan(t), f.paths, Options{Now: fixedNow})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(result.Adopted) != 1 || len(result.Linked) != 1 {
		t.Fatalf("result = %+v", result)
	}

	// The adopted path is now a symlink resolving to generated content.
	status, err := Detect(target, f.paths.Root)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != RiceManaged {
		t.Fatalf("state = %v, want RiceManaged", status.State)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "generated foot/foot.ini") {
		t.Errorf("target resolves to %q", content)
	}

	// The original survives, byte for byte.
	backup := filepath.Join(f.paths.BackupsDir, "2026-09-02T100000Z", "foot", "foot.ini")
	saved, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(saved) != original {
		t.Errorf("backup = %q, want %q", saved, original)
	}

	manifest, err := LoadManifest(filepath.Join(f.paths.StateDir, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := manifest.Find(target)
	if !ok {
		t.Fatal("adopted path is not in the manifest")
	}
	if !entry.HadExisting() || entry.Component != "foot" {
		t.Errorf("entry = %+v", entry)
	}
}

func TestApplyRefusesExternalSymlink(t *testing.T) {
	f := newFixture(t)

	elsewhere := filepath.Join(t.TempDir(), "dotfiles-foot.ini")
	if err := os.WriteFile(elsewhere, []byte("someone else's\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := f.target("foot/foot.ini")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, target); err != nil {
		t.Fatal(err)
	}

	plan := f.plan(t)
	if len(plan.Conflicts()) != 1 {
		t.Fatalf("conflicts = %d, want 1", len(plan.Conflicts()))
	}

	result, err := Apply(plan, f.paths, Options{Now: fixedNow})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(result.Skipped) != 1 {
		t.Errorf("skipped = %d, want 1", len(result.Skipped))
	}

	// The link and the file it points at are both untouched.
	got, err := os.Readlink(target)
	if err != nil || got != elsewhere {
		t.Errorf("link = %q, %v; want it left alone", got, err)
	}
	if content, err := os.ReadFile(elsewhere); err != nil || string(content) != "someone else's\n" {
		t.Errorf("external file changed: %q, %v", content, err)
	}
}

func TestApplyForceReplacesExternalSymlinkButNotItsTarget(t *testing.T) {
	f := newFixture(t)

	elsewhere := filepath.Join(t.TempDir(), "dotfiles-foot.ini")
	if err := os.WriteFile(elsewhere, []byte("someone else's\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := f.target("foot/foot.ini")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, target); err != nil {
		t.Fatal(err)
	}

	if _, err := Apply(f.plan(t), f.paths, Options{Force: true, Now: fixedNow}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	status, err := Detect(target, f.paths.Root)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != RiceManaged {
		t.Errorf("state = %v, want RiceManaged", status.State)
	}
	if content, err := os.ReadFile(elsewhere); err != nil || string(content) != "someone else's\n" {
		t.Errorf("force must not touch what the link pointed at: %q, %v", content, err)
	}
}

func TestApplyNeverForcesADirectory(t *testing.T) {
	f := newFixture(t)
	target := f.target("foot/foot.ini")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "keep-me"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := f.plan(t)
	for _, c := range plan.Conflicts() {
		if c.Target == target && c.Forceable() {
			t.Fatal("a directory conflict must never be forceable")
		}
	}

	result, err := Apply(plan, f.paths, Options{Force: true, Now: fixedNow})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(result.Skipped) != 1 {
		t.Errorf("skipped = %d, want 1", len(result.Skipped))
	}
	if _, err := os.Stat(filepath.Join(target, "keep-me")); err != nil {
		t.Errorf("directory contents were destroyed: %v", err)
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	f := newFixture(t)
	f.writeTarget(t, "foot/foot.ini", "original\n")

	if _, err := Apply(f.plan(t), f.paths, Options{Now: fixedNow}); err != nil {
		t.Fatal(err)
	}

	second := f.plan(t)
	if !second.Empty() {
		t.Errorf("second plan should be empty, got %d changes", len(second.Changes()))
	}
	result, err := Apply(second, f.paths, Options{Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed() {
		t.Errorf("re-applying changed something: %+v", result)
	}

	// Only one backup was ever taken.
	entries, err := os.ReadDir(f.paths.BackupsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("backup directories = %d, want 1", len(entries))
	}
}

func TestRelinkRepairsWithoutAdopting(t *testing.T) {
	f := newFixture(t)
	adopted := f.writeTarget(t, "foot/foot.ini", "original\n")

	// Adopt foot only, so sway stays a path Rice has never been given.
	footOnly, err := BuildPlan(f.adapters[:1], config.DefaultConfig(), f.paths, f.configDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(footOnly, f.paths, Options{Now: fixedNow}); err != nil {
		t.Fatal(err)
	}

	// A new generation appears and current moves; the link goes stale.
	gen2 := f.paths.Generation(2)
	dest := filepath.Join(gen2, "foot", "foot.ini")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("generation two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(adopted); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.paths.Generation(1), "foot", "foot.ini"), adopted); err != nil {
		t.Fatal(err)
	}

	// A path that was never adopted must stay untouched by relink.
	unadopted := f.writeTarget(t, "sway/config", "still the user's\n")

	result, err := Relink(f.plan(t), f.paths)
	if err != nil {
		t.Fatalf("Relink: %v", err)
	}
	if len(result.Relinked) != 1 {
		t.Errorf("relinked = %v, want the adopted path only", result.Relinked)
	}

	status, err := Detect(unadopted, f.paths.Root)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != RegularFile {
		t.Errorf("relink adopted an unadopted path: %v", status.State)
	}
	if content, err := os.ReadFile(unadopted); err != nil || string(content) != "still the user's\n" {
		t.Errorf("unadopted file changed: %q, %v", content, err)
	}
}

func TestRestoreRoundTrip(t *testing.T) {
	f := newFixture(t)
	original := "the user's own configuration\n"
	adopted := f.writeTarget(t, "foot/foot.ini", original)
	linked := f.target("sway/config")

	if _, err := Apply(f.plan(t), f.paths, Options{Now: fixedNow}); err != nil {
		t.Fatal(err)
	}

	manifest, err := LoadManifest(filepath.Join(f.paths.StateDir, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildRestorePlan(manifest, f.paths)
	if err != nil {
		t.Fatalf("BuildRestorePlan: %v", err)
	}

	result, err := Restore(plan, f.paths)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if len(result.Restored) != 1 || len(result.Removed) != 1 {
		t.Fatalf("result = %+v", result)
	}

	// The adopted file is back, byte for byte, as a regular file.
	status, err := Detect(adopted, f.paths.Root)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != RegularFile {
		t.Errorf("state = %v, want RegularFile", status.State)
	}
	content, err := os.ReadFile(adopted)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != original {
		t.Errorf("restored %q, want %q", content, original)
	}

	// The path that had nothing before has nothing again.
	if _, err := os.Lstat(linked); !os.IsNotExist(err) {
		t.Errorf("link should have been removed, got %v", err)
	}

	// The manifest is empty, and backups survive.
	manifest, err = LoadManifest(filepath.Join(f.paths.StateDir, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Managed) != 0 {
		t.Errorf("manifest still holds %d entries", len(manifest.Managed))
	}
	if _, err := os.Stat(filepath.Join(f.paths.BackupsDir, "2026-09-02T100000Z", "foot", "foot.ini")); err != nil {
		t.Errorf("uninstall removed the backup: %v", err)
	}
}

func TestRestoreSkipsPathsSomebodyElseTookOver(t *testing.T) {
	f := newFixture(t)
	adopted := f.writeTarget(t, "foot/foot.ini", "original\n")
	if _, err := Apply(f.plan(t), f.paths, Options{Now: fixedNow}); err != nil {
		t.Fatal(err)
	}

	// The user replaces Rice's symlink with a file of their own.
	if err := os.Remove(adopted); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(adopted, []byte("mine now\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := LoadManifest(filepath.Join(f.paths.StateDir, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildRestorePlan(manifest, f.paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Skipped()) != 1 {
		t.Fatalf("skipped = %d, want 1", len(plan.Skipped()))
	}

	if _, err := Restore(plan, f.paths); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(adopted)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "mine now\n" {
		t.Errorf("restore overwrote a file it did not own: %q", content)
	}
}

func TestManifestUpsertAndRemove(t *testing.T) {
	var m Manifest
	m.Upsert(Entry{Component: "foot", Target: "/a", Source: "foot/foot.ini"})
	m.Upsert(Entry{Component: "foot", Target: "/a", Source: "foot/foot.ini", Backup: "backups/x"})
	m.Upsert(Entry{Component: "sway", Target: "/b", Source: "sway/config"})

	if len(m.Managed) != 2 {
		t.Fatalf("entries = %d, want 2", len(m.Managed))
	}
	if e, _ := m.Find("/a"); e.Backup != "backups/x" {
		t.Errorf("upsert did not replace: %+v", e)
	}
	if !m.Adopted("sway") || m.Adopted("rofi") {
		t.Error("Adopted disagrees with the manifest")
	}
	if got := m.Components(); len(got) != 2 {
		t.Errorf("Components() = %v", got)
	}

	m.Remove("/a")
	if _, ok := m.Find("/a"); ok {
		t.Error("Remove left the entry behind")
	}
}

func TestSaveAndLoadManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", ManifestName)

	empty, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("a missing manifest should not be an error: %v", err)
	}
	if len(empty.Managed) != 0 {
		t.Error("missing manifest should be empty")
	}

	want := Manifest{Managed: []Entry{{
		Component: "foot",
		Target:    "/home/user/.config/foot/foot.ini",
		Source:    "foot/foot.ini",
		Backup:    "backups/2026-09-02T100000Z/foot/foot.ini",
		AdoptedAt: fixedNow(),
	}}}
	if err := SaveManifest(path, want); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}

	got, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(got.Managed) != 1 || got.Managed[0].Target != want.Managed[0].Target {
		t.Errorf("round trip lost data: %+v", got)
	}
	if !got.Managed[0].AdoptedAt.Equal(want.Managed[0].AdoptedAt) {
		t.Errorf("adopted_at = %v", got.Managed[0].AdoptedAt)
	}
}
