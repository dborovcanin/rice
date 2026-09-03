package session_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

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
	"github.com/dborovcanin/rice/internal/assets"
	"github.com/dborovcanin/rice/internal/command"
	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/generation"
	"github.com/dborovcanin/rice/internal/render"
	"github.com/dborovcanin/rice/internal/session"
	"github.com/dborovcanin/rice/internal/theme"
)

// newSession builds a session over the bundled themes and templates, with a
// fake runner so nothing is executed and a temporary directory for saves.
func newSession(t *testing.T) (*session.Session, *command.Fake, string) {
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
	return s, runner, themesDir
}

func TestFieldsCoverEveryGroup(t *testing.T) {
	for _, g := range session.Groups() {
		if len(session.FieldsIn(g)) == 0 {
			t.Errorf("group %s has no fields", g)
		}
	}

	seen := map[string]bool{}
	for _, f := range session.Fields() {
		if seen[f.Key] {
			t.Errorf("duplicate field key %q", f.Key)
		}
		seen[f.Key] = true
		if f.Label == "" {
			t.Errorf("field %q has no label", f.Key)
		}
	}
}

func TestSetAndReset(t *testing.T) {
	s, _, _ := newSession(t)

	if s.Dirty() {
		t.Fatal("a fresh session should not be dirty")
	}

	if err := s.Set("colors.background", "#101010"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := s.Get("colors.background")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "#101010" {
		t.Errorf("background = %q, want #101010", got)
	}
	if !s.Overridden("colors.background") {
		t.Error("background should be marked overridden")
	}
	if overrides := s.Overrides(); len(overrides) != 1 || overrides[0] != "colors.background" {
		t.Errorf("overrides = %v, want [colors.background]", overrides)
	}

	if err := s.Reset("colors.background"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if s.Dirty() {
		t.Error("session should be clean after reset")
	}
}

func TestSetRejectsBadValues(t *testing.T) {
	s, _, _ := newSession(t)

	cases := []struct{ key, value string }{
		{"colors.background", "not-a-color"},
		{"ui.opacity", "2"},
		{"ui.radius", "-1"},
		{"fonts.ui_size", "words"},
		{"icons.size", "4"},
	}
	for _, c := range cases {
		before, _ := s.Get(c.key)
		if err := s.Set(c.key, c.value); err == nil {
			t.Errorf("set %s=%q should have failed", c.key, c.value)
		}
		after, _ := s.Get(c.key)
		if before != after {
			t.Errorf("set %s=%q changed the draft to %q despite failing", c.key, c.value, after)
		}
	}

	if err := s.Set("nope.missing", "1"); err == nil {
		t.Error("setting an unknown field should fail")
	}
}

func TestNudge(t *testing.T) {
	s, _, _ := newSession(t)

	if err := s.Set("ui.radius", "8"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := s.Nudge("ui.radius", 2); err != nil {
		t.Fatalf("nudge: %v", err)
	}
	if got, _ := s.Get("ui.radius"); got != "10" {
		t.Errorf("radius = %q, want 10", got)
	}

	// Bounds hold: radius has a floor of zero.
	if err := s.Nudge("ui.radius", -100); err != nil {
		t.Fatalf("nudge: %v", err)
	}
	if got, _ := s.Get("ui.radius"); got != "0" {
		t.Errorf("radius = %q, want 0", got)
	}

	// A color nudge changes lightness rather than failing.
	before, _ := s.Get("colors.primary")
	if err := s.Nudge("colors.primary", 1); err != nil {
		t.Fatalf("nudge color: %v", err)
	}
	if after, _ := s.Get("colors.primary"); after == before {
		t.Error("nudging a color should change it")
	}

	if err := s.Nudge("icons.theme", 1); err == nil {
		t.Error("nudging a text field should fail")
	}
}

func TestSaveThemeRoundTrips(t *testing.T) {
	s, _, themesDir := newSession(t)

	if err := s.Set("colors.background", "#0a0a0a"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := s.Set("fonts.mono_family", "Iosevka"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := s.Set("icons.size", "32"); err != nil {
		t.Fatalf("set: %v", err)
	}

	path, err := s.SaveTheme("my-dark")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if want := filepath.Join(themesDir, "my-dark.toml"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}

	// Saving rebases the draft, so nothing is left overridden.
	if s.Dirty() {
		t.Error("draft should be clean immediately after saving")
	}

	// A saved theme keeps its derived values derived, so the file stays close
	// to what a person would have written.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved theme: %v", err)
	}
	if strings.Contains(string(raw), "#00000000") {
		t.Errorf("saved theme materialized unset values:\n%s", raw)
	}

	// The written file must parse through the ordinary theme loader, which
	// validates as well as decodes.
	reloaded, err := theme.ParseFile(path)
	if err != nil {
		t.Fatalf("reload saved theme: %v", err)
	}
	if reloaded.Name != "my-dark" {
		t.Errorf("name = %q, want my-dark", reloaded.Name)
	}
	if got := reloaded.Colors.Background.String(); got != "#0a0a0a" {
		t.Errorf("background = %q, want #0a0a0a", got)
	}
	if reloaded.Fonts.MonoFamily != "Iosevka" {
		t.Errorf("mono family = %q, want Iosevka", reloaded.Fonts.MonoFamily)
	}
	if reloaded.Icons.Size != 32 {
		t.Errorf("icon size = %d, want 32", reloaded.Icons.Size)
	}

	// It must also be findable by name, since a user theme shadows a bundled
	// one and that is how a customized theme is applied.
	found, err := s.Themes()
	if err != nil {
		t.Fatalf("list themes: %v", err)
	}
	var names []string
	for _, e := range found {
		names = append(names, e.Name)
	}
	if !slices.Contains(names, "my-dark") {
		t.Errorf("saved theme missing from %v", names)
	}
}

func TestValidThemeName(t *testing.T) {
	bad := []string{"", "../escape", "with/slash", `with\slash`, ".hidden", "name.toml"}
	for _, name := range bad {
		if err := session.ValidThemeName(name); err == nil {
			t.Errorf("name %q should be rejected", name)
		}
	}
	for _, name := range []string{"my-dark", "solarized_light", "Theme 2"} {
		if err := session.ValidThemeName(name); err != nil {
			t.Errorf("name %q should be accepted: %v", name, err)
		}
	}
}

func TestSandboxRendersAndCleansUp(t *testing.T) {
	s, _, _ := newSession(t)

	dir, cleanup, err := s.Sandbox()
	if err != nil {
		t.Fatalf("sandbox: %v", err)
	}
	for _, rel := range []string{"foot/foot.ini", "waybar/config.jsonc", "rofi/config.rasi", "sway/config"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}

	if err := cleanup(); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("sandbox %s should be gone, got %v", dir, err)
	}
}

func TestRenderReflectsTheDraft(t *testing.T) {
	s, _, _ := newSession(t)

	// catppuccin-mocha spells its terminal palette out, so the terminal
	// background is the field foot.ini reads.
	if err := s.Set("terminal.background", "#0b0b0b"); err != nil {
		t.Fatalf("set: %v", err)
	}
	text, err := s.ComponentText("foot")
	if err != nil {
		t.Fatalf("component text: %v", err)
	}
	// foot.ini writes bare hex, without the leading '#'.
	if !strings.Contains(text, "0b0b0b") {
		t.Error("foot.ini does not carry the edited terminal background")
	}
}

// A theme that leaves the terminal palette unset derives it from the semantic
// colors, and editing the semantic color must still move everything derived
// from it. This is why the draft is held in source form.
func TestDerivedValuesFollowTheirSource(t *testing.T) {
	s, _, _ := newSession(t)

	minimal, err := theme.ParseSource([]byte(`
name = "minimal"

[colors]
background = "#101010"
foreground = "#e0e0e0"
primary = "#5599ff"
`), "minimal.toml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	s.SetBase(minimal)

	if s.Explicit("terminal.background") {
		t.Fatal("terminal.background should be derived, not explicit")
	}
	if got, _ := s.Get("terminal.background"); got != "#101010" {
		t.Fatalf("terminal.background = %q, want the semantic background", got)
	}

	if err := s.Set("colors.background", "#0b0b0b"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got, _ := s.Get("terminal.background"); got != "#0b0b0b" {
		t.Errorf("terminal.background = %q, want it to follow colors.background", got)
	}

	text, err := s.ComponentText("foot")
	if err != nil {
		t.Fatalf("component text: %v", err)
	}
	if !strings.Contains(text, "0b0b0b") {
		t.Error("foot.ini did not follow the edited semantic background")
	}

	// Making it explicit stops it following.
	if err := s.Set("terminal.background", "#222222"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !s.Explicit("terminal.background") {
		t.Error("terminal.background should now be explicit")
	}
	if err := s.Set("colors.background", "#333333"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got, _ := s.Get("terminal.background"); got != "#222222" {
		t.Errorf("terminal.background = %q, want it to stay at the explicit value", got)
	}

	// Clearing hands it back to derivation.
	if err := s.Clear("terminal.background"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got, _ := s.Get("terminal.background"); got != "#333333" {
		t.Errorf("terminal.background = %q, want it derived again", got)
	}
}

func TestPreviewRules(t *testing.T) {
	s, runner, _ := newSession(t)

	// dunst is blocked outright while the running daemon owns the D-Bus name.
	if _, err := s.LaunchFor("dunst"); err == nil {
		t.Error("dunst preview should be refused")
	}

	// swaylock needs explicit confirmation, because it locks the screen.
	if _, err := s.Preview("swaylock", false); err == nil {
		t.Error("swaylock preview should require confirmation")
	}

	// A missing binary is reported rather than launched.
	runner.Missing = map[string]bool{"foot": true}
	if _, err := s.LaunchFor("foot"); err == nil {
		t.Error("preview should fail when the binary is absent")
	}
	runner.Missing = nil

	p, err := s.Preview("foot", false)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if p.Component != "foot" {
		t.Errorf("component = %q, want foot", p.Component)
	}
	if !strings.Contains(p.Command(), "foot -c ") {
		t.Errorf("command = %q, want a foot invocation", p.Command())
	}
	if _, err := os.Stat(p.Dir); err != nil {
		t.Fatalf("sandbox missing: %v", err)
	}

	if err := p.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, err := os.Stat(p.Dir); !os.IsNotExist(err) {
		t.Errorf("sandbox %s should be removed after stop", p.Dir)
	}
}

func TestCopyComponent(t *testing.T) {
	s, runner, _ := newSession(t)

	tool, err := s.CopyComponent(context.Background(), "rofi")
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if tool != "wl-copy" {
		t.Errorf("tool = %q, want wl-copy", tool)
	}

	if len(runner.Calls) != 1 {
		t.Fatalf("calls = %v, want one", runner.Commands())
	}
	if !strings.Contains(string(runner.Calls[0].Stdin), "configuration {") {
		t.Error("clipboard did not receive the rofi configuration")
	}

	// With no clipboard tool at all, the failure says so rather than
	// pretending it worked.
	runner.Missing = map[string]bool{"wl-copy": true, "xclip": true, "xsel": true}
	if _, err := s.CopyComponent(context.Background(), "rofi"); err == nil {
		t.Error("copy should fail when no clipboard tool exists")
	}
	if s.CanCopy() {
		t.Error("CanCopy should be false when no clipboard tool exists")
	}
}

func TestSetBaseDiscardsDraft(t *testing.T) {
	s, _, _ := newSession(t)

	if err := s.Set("colors.background", "#123456"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := s.LoadBase("tokyo-night"); err != nil {
		t.Fatalf("load base: %v", err)
	}
	if s.Dirty() {
		t.Error("choosing a new base theme should discard the draft")
	}
	if s.Base.Theme.Name != "tokyo-night" {
		t.Errorf("base = %q, want tokyo-night", s.Base.Theme.Name)
	}
}

func TestProgramFieldsAreConfigNotTheme(t *testing.T) {
	s, _, _ := newSession(t)

	if len(session.ProgramFields("waybar")) == 0 {
		t.Fatal("waybar has no editable settings")
	}
	if len(session.ProgramFields("nonesuch")) != 0 {
		t.Error("an unknown component should have no settings")
	}

	if err := s.Set("waybar.height", "42"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got, _ := s.Get("waybar.height"); got != "42" {
		t.Errorf("height = %q, want 42", got)
	}
	if s.Config().Waybar.Height != 42 {
		t.Errorf("config height = %d, want 42", s.Config().Waybar.Height)
	}

	// A program setting is not a theme override, because it is saved to a
	// different file.
	if s.ThemeDirty() {
		t.Error("editing a program setting should not dirty the theme")
	}
	if !s.ConfigDirty() || !s.Dirty() {
		t.Error("editing a program setting should dirty the configuration")
	}

	// It reaches the rendered output.
	text, err := s.ComponentText("waybar")
	if err != nil {
		t.Fatalf("component text: %v", err)
	}
	if !strings.Contains(text, `"height": 42`) {
		t.Error("waybar config does not carry the edited height")
	}
}

func TestProgramFieldsValidateAndCycle(t *testing.T) {
	s, _, _ := newSession(t)

	if err := s.Set("waybar.position", "sideways"); err == nil {
		t.Error("a value outside the choices should be rejected")
	}
	if err := s.Set("waybar.position", "bottom"); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Nudging a choice moves through the accepted values.
	if err := s.Nudge("waybar.position", 1); err != nil {
		t.Fatalf("nudge: %v", err)
	}
	if got, _ := s.Get("waybar.position"); got != "left" {
		t.Errorf("position = %q, want left", got)
	}

	// Nudging a switch flips it.
	before, _ := s.Get("rofi.show_icons")
	if err := s.Nudge("rofi.show_icons", 1); err != nil {
		t.Fatalf("nudge: %v", err)
	}
	if after, _ := s.Get("rofi.show_icons"); after == before {
		t.Error("nudging a switch should flip it")
	}

	// A program setting is never "derived": config.toml spells everything out.
	if !s.Explicit("waybar.position") {
		t.Error("program settings should always read as explicit")
	}
}

func TestSaveWritesConfigOnlyWhenChanged(t *testing.T) {
	s, _, _ := newSession(t)

	var written []config.Config
	s.SetConfigWriter(func(cfg config.Config) error {
		written = append(written, cfg)
		return nil
	})

	if _, err := s.Save("no-config-change"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if len(written) != 0 {
		t.Error("saving with no program change should not rewrite config.toml")
	}

	if err := s.Set("foot.pad_x", "12"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := s.Save("with-config-change"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if len(written) != 1 {
		t.Fatalf("config written %d times, want once", len(written))
	}
	if written[0].Foot.PadX != 12 {
		t.Errorf("written pad_x = %d, want 12", written[0].Foot.PadX)
	}
	if s.ConfigDirty() {
		t.Error("the configuration should be clean after saving")
	}
}

func TestChangingThemeKeepsProgramSettings(t *testing.T) {
	s, _, _ := newSession(t)

	if err := s.Set("rofi.lines", "15"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := s.LoadBase("tokyo-night"); err != nil {
		t.Fatalf("load base: %v", err)
	}

	if got, _ := s.Get("rofi.lines"); got != "15" {
		t.Errorf("rofi.lines = %q, want it carried across the theme change", got)
	}
	if !s.ConfigDirty() {
		t.Error("the program change should still be pending")
	}
}

// A configuration with nothing enabled is still a configuration: switching
// theme must not mistake it for "no draft yet" and throw away program edits.
func TestEmptyConfigurationSurvivesAThemeChange(t *testing.T) {
	themesDir := t.TempDir()
	store := theme.NewStore(themesDir, rice.Themes, "themes")
	base, err := store.LoadSource("catppuccin-mocha")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}

	// No components at all, so Components is its zero value.
	cfg := config.DefaultConfig()
	cfg.Components = config.Components{}
	cfg.Rofi.Lines = 9

	s, err := session.New(base, session.Options{
		Themes:    store,
		Registry:  adapter.NewRegistry(),
		Engine:    render.NewEngine("", rice.Templates, "templates"),
		Runner:    command.NewFake(),
		Config:    cfg,
		ThemesDir: themesDir,
		Version:   rice.Version,
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if err := s.Set("rofi.lines", "21"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := s.LoadBase("tokyo-night"); err != nil {
		t.Fatalf("load base: %v", err)
	}

	if got, _ := s.Get("rofi.lines"); got != "21" {
		t.Errorf("rofi.lines = %q, want the edit carried across", got)
	}
}

func TestDiffAgainstDeployed(t *testing.T) {
	themesDir := t.TempDir()
	store := theme.NewStore(themesDir, rice.Themes, "themes")
	base, err := store.LoadSource("catppuccin-mocha")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Normalize()

	registry := adapter.NewRegistry(
		sway.New(), waybar.New(), rofi.New(),
		foot.New(), dunst.New(), swaylock.New(), gtk.New(), qt.New(),
	)
	engine := render.NewEngine("", rice.Templates, "templates")

	// Stand in for a deployed generation by building one.
	deployed := t.TempDir()
	builder := generation.NewBuilder(engine, registry, rice.Version)
	resolved, err := store.Load("catppuccin-mocha")
	if err != nil {
		t.Fatalf("load resolved: %v", err)
	}
	if _, err := builder.Build(deployed, cfg, resolved, 7, generation.BuildOptions{}); err != nil {
		t.Fatalf("build: %v", err)
	}

	s, err := session.New(base, session.Options{
		Themes:     store,
		Registry:   registry,
		Engine:     engine,
		Runner:     command.NewFake(),
		Config:     cfg,
		ThemesDir:  themesDir,
		Version:    rice.Version,
		CurrentDir: deployed,
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// An untouched draft matches what is deployed, including the generation
	// number in every header.
	out, err := s.Diff(3)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("an untouched draft should show no difference:\n%s", out)
	}

	// One edit shows up, and only where it belongs.
	if err := s.Set("terminal.background", "#010203"); err != nil {
		t.Fatalf("set: %v", err)
	}
	out, err = s.Diff(3)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(out, "+background=010203") {
		t.Errorf("the edit is missing from the diff:\n%s", out)
	}
	if strings.Contains(out, "waybar/config.jsonc") {
		t.Errorf("an unrelated file appeared in the diff:\n%s", out)
	}
}

func TestDiffNeedsSomethingToCompareWith(t *testing.T) {
	s, _, _ := newSession(t)

	if _, err := s.Diff(3); !errors.Is(err, session.ErrNothingToCompare) {
		t.Errorf("err = %v, want ErrNothingToCompare", err)
	}
}

// The global section is the desktop as a whole: colours, fonts, the
// compositor, icons. Anything belonging to one application lives with that
// application.
func TestGlobalGroups(t *testing.T) {
	var names []string
	for _, g := range session.Groups() {
		names = append(names, g.String())
	}
	want := []string{"Colors", "Fonts", "SwayFX", "Icons & Cursor"}
	if !slices.Equal(names, want) {
		t.Errorf("groups = %v, want %v", names, want)
	}
}

// The ANSI palette is what a terminal is for, so it is edited under the
// terminal — but it is still part of the theme, so it saves to the theme file.
func TestTerminalPaletteLivesWithTheTerminal(t *testing.T) {
	foot := session.ProgramFields("foot")
	if len(foot) == 0 {
		t.Fatal("foot has no fields")
	}
	if foot[0].Key != "terminal.background" {
		t.Errorf("foot leads with %q, want the palette first", foot[0].Key)
	}

	var found session.Field
	for _, f := range foot {
		if f.Key == "terminal.regular.1" {
			found = f
		}
	}
	if found.Key == "" {
		t.Fatal("the ANSI palette is not under foot")
	}
	if found.Store != session.StoreTheme {
		t.Error("the palette is appearance and must save to the theme file")
	}

	// And it is not in any global group.
	for _, f := range session.Fields() {
		if strings.HasPrefix(f.Key, "terminal.") {
			t.Errorf("%q is still a global field", f.Key)
		}
	}
}

// The compositor is the desktop, not an application on it.
func TestCompositorSettingsAreGlobal(t *testing.T) {
	if len(session.ProgramFields("sway")) != 0 {
		t.Error("sway should have no per-program settings")
	}
	if session.Note("sway") == "" {
		t.Error("sway needs a note saying where its settings went")
	}

	inSwayFX := map[string]session.Field{}
	for _, f := range session.FieldsIn(session.GroupSwayFX) {
		inSwayFX[f.Key] = f
	}

	for _, key := range []string{
		"ui.radius", "ui.gaps_inner", "ui.opacity",
		"sway.mod", "sway.smart_gaps", "sway.focus_follows_mouse",
		"sway.keyboard.layout", "sway.touchpad.tap", "sway.idle.lock_after",
	} {
		if _, ok := inSwayFX[key]; !ok {
			t.Errorf("%q is missing from the SwayFX section", key)
		}
	}

	// Geometry is appearance; behaviour is structure. They share a section
	// but not a file.
	if inSwayFX["ui.radius"].Store != session.StoreTheme {
		t.Error("ui.radius should save to the theme")
	}
	if inSwayFX["sway.mod"].Store != session.StoreConfig {
		t.Error("sway.mod should save to config.toml")
	}
}

// Dirty tracking follows where a field is saved, not which table it was
// declared in — the terminal palette is edited under an application but
// belongs to the theme.
func TestDirtyTrackingFollowsTheStore(t *testing.T) {
	s, _, _ := newSession(t)

	if err := s.Set("terminal.regular.1", "#ff0000"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !s.ThemeDirty() {
		t.Error("editing the palette should dirty the theme")
	}
	if s.ConfigDirty() {
		t.Error("editing the palette should not dirty the configuration")
	}

	s.ResetAll()
	if err := s.Set("sway.mod", "Mod1"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if s.ThemeDirty() {
		t.Error("editing the compositor should not dirty the theme")
	}
	if !s.ConfigDirty() {
		t.Error("editing the compositor should dirty the configuration")
	}
}

// A palette edit made under the terminal has to survive a save, which it only
// does if the theme knows it changed.
func TestPaletteEditedUnderTheTerminalIsSaved(t *testing.T) {
	s, _, themesDir := newSession(t)

	if err := s.Set("terminal.regular.2", "#00ff00"); err != nil {
		t.Fatalf("set: %v", err)
	}
	path, err := s.SaveTheme("palette-edit")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if path != filepath.Join(themesDir, "palette-edit.toml") {
		t.Errorf("path = %q", path)
	}

	reloaded, err := theme.ParseFile(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := reloaded.Terminal.Regular[2].String(); got != "#00ff00" {
		t.Errorf("terminal.regular.2 = %q, want #00ff00", got)
	}
}

// Every field belongs to exactly one place, or the editor shows it twice and
// two rows disagree about the same value.
func TestNoFieldHasTwoHomes(t *testing.T) {
	seen := map[string]string{}

	for _, f := range session.Fields() {
		if where, dup := seen[f.Key]; dup {
			t.Errorf("%q is in both %s and a global group", f.Key, where)
		}
		seen[f.Key] = "global"
	}
	for _, component := range []string{"sway", "waybar", "rofi", "foot", "dunst", "swaylock", "gtk", "qt"} {
		for _, f := range session.ProgramFields(component) {
			if where, dup := seen[f.Key]; dup {
				t.Errorf("%q is in both %s and %s", f.Key, where, component)
			}
			seen[f.Key] = component
		}
	}

	if len(session.EveryField()) != len(seen) {
		t.Errorf("EveryField has %d, the tables have %d", len(session.EveryField()), len(seen))
	}
}

// The launcher's font and icon settings override the theme for rofi alone.
// Unset means "follow the theme", which has to be visible, undoable, and
// pickable from the same lists as the global equivalents.
func TestRofiOverridesFollowTheThemeUntilSet(t *testing.T) {
	s, _, _ := newSession(t)

	cases := []struct{ key, want string }{
		{"rofi.icon_theme", s.Theme().Icons.Theme},
		{"rofi.font_family", s.Theme().Fonts.UIFamily},
		{"rofi.font_size", strconv.Itoa(s.Theme().Fonts.UISize)},
		{"rofi.icon_size", strconv.Itoa(s.Theme().Icons.Size)},
	}

	for _, c := range cases {
		if !s.Inherited(c.key) {
			t.Errorf("%s should start following the theme", c.key)
		}
		if got := s.Effective(c.key); got != c.want {
			t.Errorf("%s shows %q, want the theme's %q", c.key, got, c.want)
		}
		// The stored value is still empty: showing the theme's value must not
		// quietly make the override explicit.
		if got, _ := s.Get(c.key); got != "" && got != "0" {
			t.Errorf("%s stores %q, want it unset", c.key, got)
		}
	}

	// Setting one takes it off the theme.
	if err := s.Set("rofi.font_size", "22"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if s.Inherited("rofi.font_size") {
		t.Error("a set override should no longer follow the theme")
	}
	if got := s.Effective("rofi.font_size"); got != "22" {
		t.Errorf("effective = %q, want 22", got)
	}

	// And clearing puts it back, which is the only way to undo an override.
	if err := s.Clear("rofi.font_size"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if !s.Inherited("rofi.font_size") {
		t.Error("clearing should return the override to the theme")
	}
	if got := s.Effective("rofi.font_size"); got != strconv.Itoa(s.Theme().Fonts.UISize) {
		t.Errorf("effective = %q, want the theme's size again", got)
	}
}

// These hold the same kind of value as the global font and icon-theme fields,
// so they are picked from the same lists rather than typed.
func TestRofiOverridesArePickable(t *testing.T) {
	byKey := map[string]session.Field{}
	for _, f := range session.ProgramFields("rofi") {
		byKey[f.Key] = f
	}

	if got := byKey["rofi.font_family"]; got.Kind != session.KindFont {
		t.Errorf("rofi.font_family kind = %v, want the font picker", got.Kind)
	}
	if got := byKey["rofi.icon_theme"]; !got.PicksAssets || got.Assets != assets.IconThemes {
		t.Error("rofi.icon_theme should pick from the installed icon themes")
	}

	// Overrides live in config.toml, not the theme.
	for _, key := range []string{"rofi.font_family", "rofi.icon_theme", "rofi.font_size", "rofi.icon_size"} {
		if byKey[key].Store != session.StoreConfig {
			t.Errorf("%s should save to config.toml", key)
		}
	}
}

// A border override exists only where it can do anything. The bar, the
// launcher and notifications draw their own frame; a terminal's is drawn by
// SwayFX, from the desktop's border, so a setting under the terminal would
// change nothing.
func TestBorderOverridesOnlyExistWhereTheAppDrawsItsOwn(t *testing.T) {
	has := func(component string) bool {
		for _, f := range session.ProgramFields(component) {
			if strings.HasSuffix(f.Key, ".border.width") {
				return true
			}
		}
		return false
	}

	for _, component := range []string{"waybar", "rofi", "dunst"} {
		if !has(component) {
			t.Errorf("%s draws its own frame and should be able to override the border", component)
		}
		if !config.SelfDecorated(component) {
			t.Errorf("%s should be self-decorated", component)
		}
	}
	for _, component := range []string{"sway", "foot", "swaylock", "gtk", "qt"} {
		if has(component) {
			t.Errorf("%s is decorated by SwayFX; its own border setting would be a no-op", component)
		}
		if config.SelfDecorated(component) {
			t.Errorf("%s should not be self-decorated", component)
		}
	}
}

// An unset border override follows the desktop's border and says so, and the
// launcher follows the focused colour because that is what it renders with.
func TestBorderOverridesFollowTheDesktopUntilSet(t *testing.T) {
	s, _, _ := newSession(t)
	wm := s.Theme().Border()

	cases := []struct{ key, want string }{
		{"rofi.border.width", strconv.Itoa(wm.Width)},
		{"rofi.border.color", wm.Focus.String()},
		{"rofi.border.radius", strconv.Itoa(wm.Radius)},
		{"dunst.border.color", wm.Color.String()},
	}
	for _, c := range cases {
		if !s.Inherited(c.key) {
			t.Errorf("%s should start following the desktop's border", c.key)
		}
		if got := s.Effective(c.key); got != c.want {
			t.Errorf("%s shows %q, want %q", c.key, got, c.want)
		}
	}

	// The swatch agrees with the value beside it rather than showing black.
	if c, ok := s.Color("rofi.border.color"); !ok || c != wm.Focus {
		t.Errorf("rofi.border.color swatch = %v, want the border it follows %v", c, wm.Focus)
	}

	if err := s.Set("rofi.border.width", "6"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if s.Inherited("rofi.border.width") || s.Config().Rofi.Border.Width != 6 {
		t.Error("a set border width should be the launcher's own")
	}
	if err := s.Clear("rofi.border.width"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if !s.Inherited("rofi.border.width") {
		t.Error("clearing should return the launcher to the desktop's border")
	}
}

// Arrows move the number that is on the screen. An override holds zero until
// it is set, so nudging what is stored would step away from the row's value:
// down to the minimum, and up from one rather than from what is shown.
func TestNudgingAnUnsetOverrideMovesFromTheValueOnScreen(t *testing.T) {
	s, _, _ := newSession(t)

	for _, key := range []string{"rofi.border.radius", "rofi.font_size", "rofi.icon_size", "dunst.font_size"} {
		f, ok := session.LookupField(key)
		if !ok {
			t.Fatalf("no field %q", key)
		}
		step := int(f.Step)

		shown, err := strconv.Atoi(s.Effective(key))
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}

		if err := s.Nudge(key, 1); err != nil {
			t.Fatalf("nudge: %v", err)
		}
		if got := s.Effective(key); got != strconv.Itoa(shown+step) {
			t.Errorf("%s up = %q, want one step above the %d on screen", key, got, shown)
		}

		if err := s.Clear(key); err != nil {
			t.Fatalf("clear: %v", err)
		}
		if err := s.Nudge(key, -1); err != nil {
			t.Fatalf("nudge: %v", err)
		}
		if got := s.Effective(key); got != strconv.Itoa(shown-step) {
			t.Errorf("%s down = %q, want one step below the %d on screen", key, got, shown)
		}
	}
}

// Zero means "follow the desktop", so turning a border off needs a value of
// its own.
func TestABorderCanBeTurnedOff(t *testing.T) {
	s, _, _ := newSession(t)

	if err := s.Set("rofi.border.width", "-1"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if s.Inherited("rofi.border.width") {
		t.Error("no border is a decision, not following the desktop")
	}
	if got := s.Config().Rofi.Border.Resolve(s.Theme().Border()).Width; got != 0 {
		t.Errorf("resolved width = %d, want none drawn", got)
	}
	if err := s.Set("rofi.border.width", "-2"); err == nil {
		t.Error("-2 is a typo, not a border")
	}

	// The whole desktop can lose its border the same way.
	if err := s.Set("ui.border_width", "-1"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := s.Theme().Border().Width; got != 0 {
		t.Errorf("desktop border width = %d, want none drawn", got)
	}
}

// Border colour sits with border width, because that is where you are thinking
// about borders — but it is still a theme value and still saves to the theme.
func TestBorderColoursAreInTheSwayFXSection(t *testing.T) {
	inSwayFX := map[string]session.Field{}
	for _, f := range session.FieldsIn(session.GroupSwayFX) {
		inSwayFX[f.Key] = f
	}

	for _, key := range []string{"colors.border", "colors.border_focus"} {
		f, ok := inSwayFX[key]
		if !ok {
			t.Errorf("%s is not in the SwayFX section", key)
			continue
		}
		if f.Kind != session.KindColor {
			t.Errorf("%s is not a colour", key)
		}
		if f.Store != session.StoreTheme {
			t.Errorf("%s should save to the theme", key)
		}
	}

	// And not left behind in Colors as well.
	for _, f := range session.FieldsIn(session.GroupColors) {
		if strings.HasPrefix(f.Key, "colors.border") {
			t.Errorf("%s is in both Colors and SwayFX", f.Key)
		}
	}
}
