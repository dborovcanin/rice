// Package tui is the terminal interface over a session: a theme picker, an
// editor for the global theme, and a per-program screen that previews and
// copies generated configuration.
//
// It holds no configuration state of its own. Every value the user changes
// goes through internal/session, which is where the editing logic is tested.
// What lives here is cursor position, focus, filter text and layout.
package tui

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dborovcanin/rice/internal/command"
	"github.com/dborovcanin/rice/internal/fonts"
	"github.com/dborovcanin/rice/internal/session"
	"github.com/dborovcanin/rice/internal/theme"
)

// Options are what the interface needs from the command layer.
type Options struct {
	// Session is the draft being edited.
	Session *session.Session
	// Runner executes fc-list when the font picker is first opened.
	Runner command.Runner
	// Apply saves the draft under a name and builds a generation from it.
	// It is injected because applying belongs to the command tree, which owns
	// generations, deployment and reload; the editor only asks for it.
	Apply func(themeName string) error
}

// Run starts the interface and blocks until the user leaves it.
func Run(opts Options) error {
	if opts.Session == nil {
		return errors.New("tui: no session")
	}

	m, err := newModel(opts)
	if err != nil {
		return err
	}

	program := tea.NewProgram(m, tea.WithAltScreen())
	final, err := program.Run()
	if err != nil {
		return err
	}
	if fm, ok := final.(*model); ok && fm.fatal != nil {
		return fm.fatal
	}
	return nil
}

// screen is which view is on top. There are only two: choosing what to start
// from, and editing it. Applications are rows in the editor's navigation
// rather than a screen of their own, because "global, then app by app" is one
// flow and splitting it hid half of it behind a keystroke.
type screen int

const (
	screenPicker screen = iota
	screenEditor
)

// pane is which half of the editor has focus.
type pane int

const (
	paneNav pane = iota
	paneFields
)

type model struct {
	opts Options
	sess *session.Session

	screen screen
	pane   pane
	width  int
	height int

	// picker state.
	themes       []theme.Entry
	pickerCursor int

	// editor state. items is the whole navigation — the global sections and
	// then the applications — and fieldCursors remembers where the cursor was
	// inside each of them, because moving away and back should return to it.
	items        []navItem
	navCursor    int
	fieldCursors map[string]int

	// search narrows the field list of the selected section. searching is
	// whether keystrokes are going into it.
	search    textinput.Model
	searching bool

	// programs are the enabled applications, in deployment order.
	programs []string

	// overlay state.
	overlay overlay

	// running previews, keyed by component, so the same program is not
	// launched twice and everything can be stopped on the way out.
	running map[string]*session.Preview

	// font catalog, loaded on first use.
	catalog     fonts.Catalog
	catalogErr  error
	catalogDone bool

	styles styles
	status string
	level  level
	fatal  error
	busy   string
}

// level is how a status line should read.
type level int

const (
	levelInfo level = iota
	levelGood
	levelWarn
	levelBad
)

func newModel(opts Options) (*model, error) {
	m := &model{
		opts:         opts,
		sess:         opts.Session,
		fieldCursors: map[string]int{},
		running:      map[string]*session.Preview{},
		search:       newSearchInput(),
		// A terminal that never reports its size still gets a usable layout
		// rather than an empty screen.
		width:  80,
		height: 24,
	}

	list, err := m.sess.Themes()
	if err != nil {
		return nil, err
	}
	m.themes = list
	for i, e := range list {
		if e.Name == m.sess.Base.Theme.Name {
			m.pickerCursor = i
		}
	}

	m.programs = m.sess.Components()
	m.items = buildNav(m.programs)
	m.navCursor = firstSelectable(m.items)
	m.restyle()

	if !truecolor() {
		m.setStatus(levelWarn, "terminal does not advertise 24-bit color: swatches are approximate")
	}
	return m, nil
}

// restyle rebuilds the interface palette from the draft.
func (m *model) restyle() { m.styles = newStyles(m.sess.Theme()) }

func (m *model) setStatus(l level, format string, args ...any) {
	m.level = l
	m.status = fmt.Sprintf(format, args...)
}

func (m *model) Init() tea.Cmd { return nil }

// previewExitedMsg arrives when a previewed application closes.
type previewExitedMsg struct {
	component string
	err       error
}

// fontsLoadedMsg carries the result of enumerating installed families.
type fontsLoadedMsg struct {
	catalog fonts.Catalog
	err     error
}

// appliedMsg carries the result of saving and applying a theme.
type appliedMsg struct {
	name string
	err  error
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.width = msg.Width
		}
		if msg.Height > 0 {
			m.height = msg.Height
		}
		return m, nil

	case previewExitedMsg:
		delete(m.running, msg.component)
		if msg.err != nil {
			// A non-zero exit is ordinary here: Rofi returns 1 when it is
			// dismissed. The message is worth showing, an alarm is not.
			m.setStatus(levelWarn, "%s preview exited: %v", msg.component, msg.err)
		} else {
			m.setStatus(levelInfo, "%s preview closed", msg.component)
		}
		return m, nil

	case fontsLoadedMsg:
		m.catalog, m.catalogErr, m.catalogDone = msg.catalog, msg.err, true
		if msg.err != nil {
			m.setStatus(levelWarn, "%v", msg.err)
			return m, nil
		}
		if m.overlay.kind == overlayFonts {
			m.overlay.all = fontEntries(m.catalog.Filter(m.overlay.input.Value(), m.overlay.mono))
			m.overlay.entries = m.overlay.all
			m.setStatus(levelInfo, "%d font families installed", m.catalog.Len())
		}
		return m, nil

	case appliedMsg:
		m.busy = ""
		if msg.err != nil {
			m.setStatus(levelBad, "apply: %v", msg.err)
			return m, nil
		}
		if list, err := m.sess.Themes(); err == nil {
			m.themes = list
		}
		m.setStatus(levelGood, "applied %s and switched to the new generation", msg.name)
		return m, nil

	case tea.KeyMsg:
		if m.busy != "" {
			// A build is in flight; swallow input rather than queue edits
			// against a draft that is being written out.
			return m, nil
		}
		if m.overlay.kind != overlayNone {
			return m.updateOverlay(msg)
		}
		return m.updateScreen(msg)
	}
	return m, nil
}

// updateScreen handles keys that are not captured by an overlay.
func (m *model) updateScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, m.quit()
	case "q":
		if m.screen == screenPicker {
			return m, m.confirmQuit()
		}
	}

	switch m.screen {
	case screenPicker:
		return m.updatePicker(msg)
	case screenEditor:
		return m.updateEditor(msg)
	}
	return m, nil
}

// confirmQuit asks before throwing away unsaved edits, and leaves immediately
// when there is nothing to lose.
func (m *model) confirmQuit() tea.Cmd {
	if !m.sess.Dirty() {
		return m.quit()
	}
	m.overlay = confirmOverlay(
		"Leave without saving? The draft is not written anywhere.",
		func(mm *model) tea.Cmd { return mm.quit() },
	)
	return nil
}

// quit stops every running preview and leaves.
func (m *model) quit() tea.Cmd {
	if err := m.sess.Close(); err != nil {
		m.fatal = err
	}
	return tea.Quit
}

// loadFonts enumerates installed families in the background, because fc-list
// over a few thousand fonts is fast but not instant.
func (m *model) loadFonts() tea.Cmd {
	if m.catalogDone || m.opts.Runner == nil {
		return nil
	}
	runner := m.opts.Runner
	return func() tea.Msg {
		catalog, err := fonts.Load(context.Background(), runner)
		return fontsLoadedMsg{catalog: catalog, err: err}
	}
}

// startPreview launches one component and watches for it to exit.
func (m *model) startPreview(component string, confirmed bool) tea.Cmd {
	if p, ok := m.running[component]; ok {
		m.setStatus(levelInfo, "%s preview is already running (%s)", component, p.Command())
		return nil
	}

	p, err := m.sess.Preview(component, confirmed)
	if err != nil {
		m.setStatus(levelBad, "%v", err)
		return nil
	}
	m.running[component] = p

	if l, err := m.sess.LaunchFor(component); err == nil && l.Note != "" {
		m.setStatus(levelWarn, "%s: %s", component, l.Note)
	} else {
		m.setStatus(levelGood, "previewing %s from %s", component, p.Dir)
	}

	return func() tea.Msg {
		err := p.Wait()
		return previewExitedMsg{component: component, err: err}
	}
}

// applyDraft saves the draft under a name and builds a generation from it.
func (m *model) applyDraft(name string) tea.Cmd {
	if m.opts.Apply == nil {
		m.setStatus(levelBad, "applying is not available in this context")
		return nil
	}
	if _, err := m.sess.Save(name); err != nil {
		m.setStatus(levelBad, "%v", err)
		return nil
	}

	m.busy = "applying"
	m.setStatus(levelInfo, "applying %s…", name)

	apply := m.opts.Apply
	return func() tea.Msg {
		return appliedMsg{name: name, err: apply(name)}
	}
}

func (m *model) View() string {
	var body string
	switch m.screen {
	case screenPicker:
		body = m.viewPicker()
	case screenEditor:
		body = m.viewEditor()
	}

	if m.overlay.kind != overlayNone {
		body = m.viewOverlay()
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewHeader(),
		body,
		m.viewStatus(),
	)
}

func (m *model) viewHeader() string {
	name := m.sess.Base.Theme.Name
	if name == "" {
		name = "untitled"
	}
	title := "rice · " + name
	if m.sess.Dirty() {
		title += " *"
	}

	right := ""
	if n := len(m.running); n > 0 {
		right = m.styles.subtle.Render(fmt.Sprintf("%d preview(s) running", n))
	}

	left := m.styles.header.Render(title)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + pad("", gap) + right
}

func (m *model) viewStatus() string {
	if m.busy != "" {
		return m.styles.warn.Render(m.busy + "…")
	}

	style := m.styles.subtle
	switch m.level {
	case levelGood:
		style = m.styles.ok
	case levelWarn:
		style = m.styles.warn
	case levelBad:
		style = m.styles.fail
	}

	status := style.Render(truncate(m.status, m.width))
	return status + "\n" + m.styles.keys.Render(truncate(m.helpLine(), m.width))
}

func (m *model) helpLine() string {
	if m.overlay.kind != overlayNone {
		return m.overlay.help()
	}
	switch m.screen {
	case screenPicker:
		return "↑↓ move · enter choose · q quit"
	case screenEditor:
		if m.searching {
			return "type to narrow · enter keep · esc clear"
		}
		// The keys that act on an application are only worth naming when one
		// is selected, which keeps this line short enough to survive.
		if _, isApp := m.app(); isApp {
			return "tab pane · ↑↓ move · / search · enter edit · ←→ change · r reset · " +
				"p preview · v view · y copy · s save · a apply · t themes"
		}
		return "tab pane · ↑↓ move · / search · enter edit · ←→ nudge · r reset · " +
			"c clear · d diff · y copy theme · s save · a apply · t themes"
	}
	return ""
}
