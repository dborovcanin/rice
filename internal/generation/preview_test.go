package generation

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewSwitchesWithoutCommitting(t *testing.T) {
	store, cfg, th := newTestStore(t, false)

	first, err := store.Create(cfg, th, BuildOptions{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Switch(first.Number); err != nil {
		t.Fatalf("Switch: %v", err)
	}

	state, err := store.Preview(cfg, th)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if state.Parent != first.Number {
		t.Errorf("parent = %d, want %d", state.Parent, first.Number)
	}

	if !store.PreviewActive() {
		t.Error("the preview should be active")
	}
	if target, _ := os.Readlink(store.Paths.Current); target != "preview" {
		t.Errorf("current -> %q, want preview", target)
	}
	if _, err := os.Stat(filepath.Join(store.Paths.PreviewDir, "sway", "config")); err != nil {
		t.Errorf("preview was not rendered: %v", err)
	}

	// A preview takes no generation number and is not a generation.
	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("generations = %d, want the preview not to have committed one", len(list))
	}
	if next, _ := store.Next(); next != 2 {
		t.Errorf("next = %d, want 2", next)
	}

	// Current has no answer while a preview holds the link, and says so.
	if _, err := store.Current(); !errors.Is(err, ErrPreviewActive) {
		t.Errorf("Current err = %v, want ErrPreviewActive", err)
	}
}

func TestPreviewCancelRestoresExactly(t *testing.T) {
	store, cfg, th := newTestStore(t, false)

	first, _ := store.Create(cfg, th, BuildOptions{})
	second, _ := store.Create(cfg, th, BuildOptions{})
	if err := store.Switch(first.Number); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if err := store.Switch(second.Number); err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if _, err := store.Preview(cfg, th); err != nil {
		t.Fatalf("Preview: %v", err)
	}
	state, err := store.CancelPreview()
	if err != nil {
		t.Fatalf("CancelPreview: %v", err)
	}
	if state.Parent != second.Number {
		t.Errorf("parent = %d, want %d", state.Parent, second.Number)
	}

	if current, err := store.Current(); err != nil || current != second.Number {
		t.Errorf("current = %d (%v), want %d", current, err, second.Number)
	}
	// Previewing must not disturb where rollback goes.
	if previous, err := store.Previous(); err != nil || previous != first.Number {
		t.Errorf("previous = %d (%v), want %d", previous, err, first.Number)
	}
	if _, err := os.Stat(store.Paths.PreviewDir); !os.IsNotExist(err) {
		t.Errorf("preview directory survived cancel: %v", err)
	}
	if store.PreviewActive() {
		t.Error("the preview should be gone")
	}
}

func TestPreviewCommitBuildsARealGeneration(t *testing.T) {
	store, cfg, th := newTestStore(t, false)

	first, _ := store.Create(cfg, th, BuildOptions{})
	if err := store.Switch(first.Number); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if _, err := store.Preview(cfg, th); err != nil {
		t.Fatalf("Preview: %v", err)
	}

	info, err := store.CommitPreview(cfg, th, BuildOptions{Description: "kept"})
	if err != nil {
		t.Fatalf("CommitPreview: %v", err)
	}
	if info.Number != 2 {
		t.Errorf("committed generation = %d, want 2", info.Number)
	}
	if info.Manifest.Parent != first.Number {
		t.Errorf("parent = %d, want %d", info.Manifest.Parent, first.Number)
	}

	if current, err := store.Current(); err != nil || current != info.Number {
		t.Errorf("current = %d (%v), want %d", current, err, info.Number)
	}
	// The generation the preview was started from becomes the rollback target.
	if previous, err := store.Previous(); err != nil || previous != first.Number {
		t.Errorf("previous = %d (%v), want %d", previous, err, first.Number)
	}
	if store.PreviewActive() {
		t.Error("committing should end the preview")
	}
	if _, err := os.Stat(store.Paths.PreviewDir); !os.IsNotExist(err) {
		t.Errorf("preview directory survived commit: %v", err)
	}

	// The committed files carry the real generation number, not the preview's
	// guess at one.
	data, err := os.ReadFile(filepath.Join(info.Dir, "sway", "config"))
	if err != nil {
		t.Fatalf("read committed file: %v", err)
	}
	if want := "generation=2"; !strings.Contains(string(data), want) {
		t.Errorf("committed file does not say %q:\n%s", want, data)
	}
}

// Previewing twice keeps the original parent: cancelling must return to the
// last committed generation, not to another preview.
func TestPreviewTwiceKeepsTheOriginalParent(t *testing.T) {
	store, cfg, th := newTestStore(t, false)

	first, _ := store.Create(cfg, th, BuildOptions{})
	if err := store.Switch(first.Number); err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if _, err := store.Preview(cfg, th); err != nil {
		t.Fatalf("first Preview: %v", err)
	}
	state, err := store.Preview(cfg, th)
	if err != nil {
		t.Fatalf("second Preview: %v", err)
	}
	if state.Parent != first.Number {
		t.Errorf("parent = %d, want %d", state.Parent, first.Number)
	}
}

// A preview started before anything was committed has nowhere to return to,
// and cancelling has to say so rather than fail.
func TestPreviewWithNoParent(t *testing.T) {
	store, cfg, th := newTestStore(t, false)

	state, err := store.Preview(cfg, th)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if state.Parent != 0 {
		t.Errorf("parent = %d, want none", state.Parent)
	}

	if _, err := store.CancelPreview(); err != nil {
		t.Fatalf("CancelPreview: %v", err)
	}
	if _, err := os.Lstat(store.Paths.Current); !os.IsNotExist(err) {
		t.Errorf("current should be gone: %v", err)
	}
}

func TestPreviewOperationsNeedAPreview(t *testing.T) {
	store, cfg, th := newTestStore(t, false)

	if _, err := store.CancelPreview(); !errors.Is(err, ErrNoPreview) {
		t.Errorf("CancelPreview err = %v, want ErrNoPreview", err)
	}
	if _, err := store.CommitPreview(cfg, th, BuildOptions{}); !errors.Is(err, ErrNoPreview) {
		t.Errorf("CommitPreview err = %v, want ErrNoPreview", err)
	}
}

// A failed render must leave the running preview alone rather than replacing
// it with a half-written one.
func TestFailedPreviewLeavesTheRunningOneAlone(t *testing.T) {
	store, cfg, th := newTestStore(t, false)

	if _, err := store.Preview(cfg, th); err != nil {
		t.Fatalf("Preview: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(store.Paths.PreviewDir, "sway", "config"))
	if err != nil {
		t.Fatalf("read preview: %v", err)
	}

	broken := cfg
	broken.Components.Waybar = true // no adapter is registered for it
	if _, err := store.Preview(broken, th); err == nil {
		t.Fatal("a preview of an unknown component should fail")
	}

	after, err := os.ReadFile(filepath.Join(store.Paths.PreviewDir, "sway", "config"))
	if err != nil {
		t.Fatalf("the running preview was destroyed: %v", err)
	}
	if string(before) != string(after) {
		t.Error("the running preview was rewritten by a failed render")
	}
	if !store.PreviewActive() {
		t.Error("the running preview should still be active")
	}
}

// Pruning must not remove the generation a running preview would cancel to.
func TestPruneProtectsThePreviewParent(t *testing.T) {
	store, cfg, th := newTestStore(t, false)

	first, _ := store.Create(cfg, th, BuildOptions{})
	if err := store.Switch(first.Number); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	for range 4 {
		if _, err := store.Create(cfg, th, BuildOptions{}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	if _, err := store.Preview(cfg, th); err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if _, err := store.Prune(1); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if _, err := os.Stat(store.Paths.Generation(first.Number)); err != nil {
		t.Errorf("the preview parent was pruned: %v", err)
	}

	if _, err := store.CancelPreview(); err != nil {
		t.Errorf("cancel after pruning: %v", err)
	}
}
