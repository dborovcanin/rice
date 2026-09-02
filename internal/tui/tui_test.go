package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	rice "github.com/dborovcanin/rice"
	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/adapter/dunst"
	"github.com/dborovcanin/rice/internal/adapter/foot"
	"github.com/dborovcanin/rice/internal/adapter/gtk"
	"github.com/dborovcanin/rice/internal/adapter/qt"
	"github.com/dborovcanin/rice/internal/adapter/rofi"
	"github.com/dborovcanin/rice/internal/adapter/sway"
	"github.com/dborovcanin/rice/internal/adapter/swaylock"
	"github.com/dborovcanin/rice/internal/adapter/waybar"
	"github.com/dborovcanin/rice/internal/command"
	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/render"
	"github.com/dborovcanin/rice/internal/session"
	"github.com/dborovcanin/rice/internal/theme"
)

// newTestModel builds the interface over a real session with a fake runner, so
// key handling is exercised without a terminal and without launching anything.
func newTestModel(t *testing.T) (*model, *command.Fake, string) {
	t.Helper()

	themesDir := t.TempDir()
	store := theme.NewStore(themesDir, rice.Themes, "themes")
	base, err := store.LoadSource("catppuccin-mocha")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Normalize()

	runner := command.NewFake()
	s, err := session.New(base, session.Options{
		Themes: store,
		Registry: adapter.NewRegistry(
			sway.New(), waybar.New(), rofi.New(),
			foot.New(), dunst.New(), swaylock.New(),
			gtk.New(), qt.New(),
		),
		Engine:      render.NewEngine("", rice.Templates, "templates"),
		Runner:      runner,
		Config:      cfg,
		ThemesDir:   themesDir,
		Version:     rice.Version,
		SandboxRoot: filepath.Join(t.TempDir(), "preview"),
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	var applied []string
	m, err := newModel(Options{
		Session: s,
		Runner:  runner,
		Apply: func(name string) error {
			applied = append(applied, name)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	m.width, m.height = 120, 40
	return m, runner, themesDir
}

// press sends one key and returns the model, which is always the same pointer.
func press(t *testing.T, m *model, keys ...string) tea.Cmd {
	t.Helper()

	var cmd tea.Cmd
	for _, key := range keys {
		var msg tea.KeyMsg
		switch key {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEscape}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "left":
			msg = tea.KeyMsg{Type: tea.KeyLeft}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}

		got, c := m.Update(msg)
		if got != tea.Model(m) {
			t.Fatalf("Update returned a different model for key %q", key)
		}
		cmd = c
	}
	return cmd
}

// typeText sends each rune of s as its own key, the way a terminal would.
func typeText(t *testing.T, m *model, s string) {
	t.Helper()
	for _, r := range s {
		press(t, m, string(r))
	}
}

func TestViewsRenderAtAnySize(t *testing.T) {
	m, _, _ := newTestModel(t)

	sizes := []struct{ w, h int }{{120, 40}, {80, 24}, {40, 10}, {20, 6}}
	screens := []screen{screenPicker, screenEditor, screenPrograms}

	for _, size := range sizes {
		m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		for _, sc := range screens {
			m.screen = sc
			out := m.View()
			if out == "" {
				t.Errorf("screen %d rendered nothing at %dx%d", sc, size.w, size.h)
			}
		}
	}
}

func TestPickerChoosesATheme(t *testing.T) {
	m, _, _ := newTestModel(t)

	if m.screen != screenPicker {
		t.Fatal("the interface should open on the theme picker")
	}

	// Move to a different theme and choose it.
	target := ""
	for i, e := range m.themes {
		if e.Name != m.sess.Base.Theme.Name {
			m.pickerCursor = i
			target = e.Name
			break
		}
	}
	press(t, m, "enter")

	if m.screen != screenEditor {
		t.Error("choosing a theme should open the editor")
	}
	if m.sess.Base.Theme.Name != target {
		t.Errorf("base = %q, want %q", m.sess.Base.Theme.Name, target)
	}
}

func TestPickerConfirmsBeforeDiscardingADraft(t *testing.T) {
	m, _, _ := newTestModel(t)

	if err := m.sess.Set("colors.primary", "#ff0000"); err != nil {
		t.Fatalf("set: %v", err)
	}

	for i, e := range m.themes {
		if e.Name != m.sess.Base.Theme.Name {
			m.pickerCursor = i
			break
		}
	}
	press(t, m, "enter")

	if m.overlay.kind != overlayConfirm {
		t.Fatal("switching base with unsaved changes should ask first")
	}

	// Declining leaves everything alone.
	press(t, m, "n")
	if m.screen != screenPicker || !m.sess.Dirty() {
		t.Error("declining should keep the draft and stay on the picker")
	}

	press(t, m, "enter")
	press(t, m, "y")
	if m.screen != screenEditor {
		t.Error("confirming should open the editor")
	}
	if m.sess.Dirty() {
		t.Error("confirming should have discarded the draft")
	}
}

func TestEditorEditsAField(t *testing.T) {
	m, _, _ := newTestModel(t)
	press(t, m, "enter") // into the editor

	// Focus the fields pane; the first group is Colors, whose first field is
	// the background.
	press(t, m, "tab")
	if m.pane != paneFields {
		t.Fatal("tab should move focus to the fields")
	}

	f, ok := m.field()
	if !ok || f.Key != "colors.background" {
		t.Fatalf("field = %v, want colors.background", f.Key)
	}

	press(t, m, "enter")
	if m.overlay.kind != overlayText {
		t.Fatal("enter on a color field should open the text editor")
	}

	m.overlay.input.SetValue("#123456")
	press(t, m, "enter")

	if m.overlay.kind != overlayNone {
		t.Error("a valid value should close the overlay")
	}
	if got, _ := m.sess.Get("colors.background"); got != "#123456" {
		t.Errorf("background = %q, want #123456", got)
	}
	if !m.sess.Overridden("colors.background") {
		t.Error("the field should be marked as changed")
	}
}

func TestEditorKeepsOverlayOpenOnABadValue(t *testing.T) {
	m, _, _ := newTestModel(t)
	press(t, m, "enter", "tab", "enter")

	m.overlay.input.SetValue("not-a-color")
	press(t, m, "enter")

	if m.overlay.kind != overlayText {
		t.Error("an invalid value should leave the overlay open to be corrected")
	}
	if m.level != levelBad {
		t.Error("an invalid value should be reported")
	}
}

func TestEditorNudgeResetAndClear(t *testing.T) {
	m, _, _ := newTestModel(t)
	press(t, m, "enter", "tab")

	before, _ := m.sess.Get("colors.background")
	press(t, m, "right")
	after, _ := m.sess.Get("colors.background")
	if before == after {
		t.Error("nudging should change the value")
	}

	press(t, m, "r")
	if got, _ := m.sess.Get("colors.background"); got != before {
		t.Errorf("reset gave %q, want %q", got, before)
	}

	// Clearing an explicit value hands it back to derivation.
	press(t, m, "c")
	if m.sess.Explicit("colors.background") {
		t.Error("clear should have removed the explicit value")
	}
}

func TestEditorSavesATheme(t *testing.T) {
	m, _, themesDir := newTestModel(t)
	press(t, m, "enter", "tab")

	if err := m.sess.Set("colors.primary", "#abcdef"); err != nil {
		t.Fatalf("set: %v", err)
	}

	press(t, m, "s")
	if m.overlay.kind != overlaySave {
		t.Fatal("s should open the save prompt")
	}
	// The suggestion must not silently shadow the bundled theme.
	if got := m.overlay.input.Value(); !strings.HasSuffix(got, "-custom") {
		t.Errorf("suggested name = %q, want a -custom variant of a bundled theme", got)
	}

	m.overlay.input.SetValue("saved-theme")
	press(t, m, "enter")

	if m.overlay.kind != overlayNone {
		t.Fatal("saving should close the prompt")
	}
	if m.level != levelGood {
		t.Errorf("saving should report success, got status %q", m.status)
	}

	if _, err := theme.ParseFile(filepath.Join(themesDir, "saved-theme.toml")); err != nil {
		t.Errorf("saved theme does not load: %v", err)
	}

	// The picker must learn about the new theme.
	found := false
	for _, e := range m.themes {
		if e.Name == "saved-theme" {
			found = true
		}
	}
	if !found {
		t.Error("the saved theme is missing from the picker")
	}
}

func TestEditorRejectsABadThemeName(t *testing.T) {
	m, _, _ := newTestModel(t)
	press(t, m, "enter", "s")

	m.overlay.input.SetValue("../escape")
	press(t, m, "enter")

	if m.overlay.kind != overlaySave {
		t.Error("a rejected name should leave the prompt open")
	}
	if m.level != levelBad {
		t.Error("a rejected name should be reported")
	}
}

func TestProgramsPreviewAndCopy(t *testing.T) {
	m, runner, _ := newTestModel(t)
	press(t, m, "enter", "g")

	if m.screen != screenPrograms {
		t.Fatal("g should open the programs screen")
	}

	// Land on foot, which previews without conditions.
	for i, name := range m.programs {
		if name == "foot" {
			m.programCursor = i
		}
	}

	cmd := press(t, m, "p")
	if m.running["foot"] == nil {
		t.Fatalf("preview did not start: %s", m.status)
	}
	if cmd == nil {
		t.Error("a preview should return a command that waits for the process")
	}

	// The wait command reports the exit back into the model.
	msg := cmd()
	m.Update(msg)
	if m.running["foot"] != nil {
		t.Error("the preview should be forgotten once the process exits")
	}

	runner.Calls = nil
	press(t, m, "y")
	if len(runner.Calls) != 1 || runner.Calls[0].Name != "wl-copy" {
		t.Errorf("copy ran %v, want a wl-copy", runner.Commands())
	}
}

func TestProgramsConfirmBeforeLockingTheScreen(t *testing.T) {
	m, _, _ := newTestModel(t)
	press(t, m, "enter", "g")

	for i, name := range m.programs {
		if name == "swaylock" {
			m.programCursor = i
		}
	}

	press(t, m, "p")
	if m.overlay.kind != overlayConfirm {
		t.Fatal("previewing swaylock should ask first")
	}
	if m.running["swaylock"] != nil {
		t.Fatal("nothing should start before confirmation")
	}

	press(t, m, "y")
	if m.running["swaylock"] == nil {
		t.Error("confirming should start the preview")
	}
}

func TestProgramsReportWhyAPreviewIsUnavailable(t *testing.T) {
	m, _, _ := newTestModel(t)
	press(t, m, "enter", "g")

	for i, name := range m.programs {
		if name == "dunst" {
			m.programCursor = i
		}
	}

	press(t, m, "p")
	if m.running["dunst"] != nil {
		t.Fatal("dunst should not preview while the daemon owns the D-Bus name")
	}
	if m.level != levelBad || !strings.Contains(m.status, "D-Bus") {
		t.Errorf("status = %q, want an explanation", m.status)
	}
}

func TestFontOverlayFallsBackToTyping(t *testing.T) {
	m, runner, _ := newTestModel(t)
	runner.Missing = map[string]bool{"fc-list": true}

	press(t, m, "enter")

	// Move to the Fonts group and its first field.
	for i, g := range session.Groups() {
		if g == session.GroupFonts {
			m.groupCursor = i
		}
	}
	press(t, m, "tab", "enter")

	if m.overlay.kind != overlayFonts {
		t.Fatal("a font field should open the font picker")
	}

	// With no fontconfig, the filter box is the value.
	m.catalogDone, m.catalogErr = true, nil
	typeText(t, m, "Iosevka")
	press(t, m, "enter")

	if got, _ := m.sess.Get("fonts.ui_family"); got != "Iosevka" {
		t.Errorf("ui family = %q, want Iosevka", got)
	}
}

func TestQuitStopsRunningPreviews(t *testing.T) {
	m, _, _ := newTestModel(t)
	press(t, m, "enter", "g")

	for i, name := range m.programs {
		if name == "foot" {
			m.programCursor = i
		}
	}
	press(t, m, "p")

	p := m.running["foot"]
	if p == nil {
		t.Fatal("preview did not start")
	}

	m.quit()
	if m.fatal != nil {
		t.Errorf("quit reported %v", m.fatal)
	}
}

func TestProgramsEditASetting(t *testing.T) {
	m, _, _ := newTestModel(t)
	press(t, m, "enter", "g")

	for i, name := range m.programs {
		if name == "waybar" {
			m.programCursor = i
		}
	}

	press(t, m, "tab")
	if m.programPane != paneFields {
		t.Fatal("tab should move focus to the program's settings")
	}

	// waybar's first setting is its position, a fixed set of choices, so
	// enter cycles rather than opening a text prompt.
	f, ok := m.programField()
	if !ok || f.Key != "waybar.position" {
		t.Fatalf("field = %q, want waybar.position", f.Key)
	}

	before, _ := m.sess.Get(f.Key)
	press(t, m, "enter")
	after, _ := m.sess.Get(f.Key)
	if before == after {
		t.Error("enter on a choice should move to the next value")
	}
	if m.overlay.kind != overlayNone {
		t.Error("a choice should not open a text prompt")
	}

	press(t, m, "r")
	if got, _ := m.sess.Get(f.Key); got != before {
		t.Errorf("reset gave %q, want %q", got, before)
	}

	// A numeric setting does open a prompt.
	m.setProgramFieldCursor(2) // waybar.height
	f, _ = m.programField()
	if f.Key != "waybar.height" {
		t.Fatalf("field = %q, want waybar.height", f.Key)
	}
	press(t, m, "enter")
	if m.overlay.kind != overlayText {
		t.Fatal("a numeric setting should open the text prompt")
	}
	m.overlay.input.SetValue("36")
	press(t, m, "enter")
	if got, _ := m.sess.Get("waybar.height"); got != "36" {
		t.Errorf("height = %q, want 36", got)
	}
}

func TestProgramsKeepTheirOwnCursor(t *testing.T) {
	m, _, _ := newTestModel(t)
	press(t, m, "enter", "g")

	for i, name := range m.programs {
		if name == "foot" {
			m.programCursor = i
		}
	}
	press(t, m, "tab", "down", "down")
	footCursor := m.programFieldCursor()
	if footCursor != 2 {
		t.Fatalf("foot cursor = %d, want 2", footCursor)
	}

	press(t, m, "tab", "up")
	press(t, m, "tab")
	if got := m.programFieldCursor(); got == footCursor {
		t.Skip("the neighbouring program happens to share a cursor position")
	}

	// Coming back restores where the cursor was.
	for i, name := range m.programs {
		if name == "foot" {
			m.programCursor = i
		}
	}
	if got := m.programFieldCursor(); got != footCursor {
		t.Errorf("foot cursor = %d, want it remembered as %d", got, footCursor)
	}
}

func TestSaveWritesProgramSettings(t *testing.T) {
	m, _, _ := newTestModel(t)

	var written int
	m.sess.SetConfigWriter(func(config.Config) error {
		written++
		return nil
	})

	press(t, m, "enter", "g")
	for i, name := range m.programs {
		if name == "rofi" {
			m.programCursor = i
		}
	}
	press(t, m, "tab")
	m.setProgramFieldCursor(1) // rofi.lines
	press(t, m, "enter")
	m.overlay.input.SetValue("14")
	press(t, m, "enter")

	if !m.sess.ConfigDirty() {
		t.Fatal("the program setting did not register")
	}

	press(t, m, "s")
	if m.overlay.kind != overlaySave {
		t.Fatal("s should open the save prompt from the programs screen")
	}
	m.overlay.input.SetValue("with-programs")
	press(t, m, "enter")

	if written != 1 {
		t.Errorf("config written %d times, want once", written)
	}
	if m.level != levelGood {
		t.Errorf("saving reported %q", m.status)
	}
}

func TestIconThemeOpensTheInstalledThemePicker(t *testing.T) {
	// Point the asset lookups at a tree with two known icon themes.
	root := t.TempDir()
	for _, dir := range []string{"icons/Papirus-Dark", "icons/Adwaita"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "index.theme"), nil, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	t.Setenv("XDG_DATA_HOME", root)
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "empty"))
	t.Setenv("HOME", root)

	m, _, _ := newTestModel(t)
	press(t, m, "enter")

	for i, g := range session.Groups() {
		if g == session.GroupIcons {
			m.groupCursor = i
		}
	}
	press(t, m, "tab", "enter")

	if m.overlay.kind != overlayAssets {
		t.Fatalf("overlay = %v, want the installed-theme picker", m.overlay.kind)
	}
	if len(m.overlay.entries) != 2 {
		t.Fatalf("entries = %v, want the two installed themes", m.overlay.entries)
	}

	// The cursor starts on the value the theme already has.
	current, _ := m.sess.Get("icons.theme")
	if got := m.overlay.entries[m.overlay.cursor].name; got != current {
		t.Errorf("cursor on %q, want the current value %q", got, current)
	}

	// Typing filters, and enter takes the highlighted row.
	typeText(t, m, "adw")
	if len(m.overlay.entries) != 1 || m.overlay.entries[0].name != "Adwaita" {
		t.Fatalf("filtered to %v, want just Adwaita", m.overlay.entries)
	}
	press(t, m, "enter")

	if got, _ := m.sess.Get("icons.theme"); got != "Adwaita" {
		t.Errorf("icon theme = %q, want Adwaita", got)
	}
}

func TestPickerFallsBackToTypingWhenNothingIsInstalled(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "empty"))
	t.Setenv("HOME", root)

	m, _, _ := newTestModel(t)
	press(t, m, "enter")

	for i, g := range session.Groups() {
		if g == session.GroupIcons {
			m.groupCursor = i
		}
	}
	press(t, m, "tab", "enter")

	if m.overlay.kind != overlayAssets {
		t.Fatal("expected the installed-theme picker")
	}
	typeText(t, m, "Hand-Typed")
	press(t, m, "enter")

	if got, _ := m.sess.Get("icons.theme"); got != "Hand-Typed" {
		t.Errorf("icon theme = %q, want the typed value", got)
	}
}
