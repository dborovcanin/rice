package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dborovcanin/rice/internal/assets"
	"github.com/dborovcanin/rice/internal/fonts"
	"github.com/dborovcanin/rice/internal/session"
)

// overlayKind is which modal is open over the current screen.
type overlayKind int

const (
	overlayNone overlayKind = iota
	// overlayText edits one field's value as text.
	overlayText
	// overlayFonts picks a font family from the installed families.
	overlayFonts
	// overlayAssets picks an installed icon, cursor, GTK or Kvantum theme.
	overlayAssets
	// overlaySave asks for a theme name, and optionally applies afterwards.
	overlaySave
	// overlayConfirm asks a yes-or-no question before something irreversible
	// or surprising.
	overlayConfirm
	// overlayView shows generated output, scrolled but not editable.
	overlayView
)

// overlay is the modal state. Only one is ever open, so this is a single
// struct rather than a stack: nothing in the editor needs a modal over a modal.
type overlay struct {
	kind  overlayKind
	title string
	input textinput.Model

	// field is what overlayText and overlayFonts are editing.
	field session.Field

	// picker state, shared by the font and installed-theme pickers.
	entries []entry
	all     []entry
	cursor  int
	mono    bool
	loadErr error

	// apply marks a save overlay that should build a generation afterwards.
	apply bool

	// onConfirm runs when a confirm overlay is accepted.
	onConfirm func(*model) tea.Cmd

	// lines is the content of a view overlay, and offset is how far down it
	// has been scrolled.
	lines  []string
	offset int
}

// entry is one row in a picker: what would be chosen, and a word about it.
type entry struct {
	name string
	note string
}

// filterEntries keeps the rows matching a query, with prefix matches first, so
// typing narrows a list the same way everywhere.
func filterEntries(all []entry, query string) []entry {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return all
	}

	var prefix, contains []entry
	for _, e := range all {
		lower := strings.ToLower(e.name)
		switch {
		case strings.HasPrefix(lower, q):
			prefix = append(prefix, e)
		case strings.Contains(lower, q):
			contains = append(contains, e)
		}
	}
	return append(prefix, contains...)
}

func newInput(value, placeholder string, width int) textinput.Model {
	in := textinput.New()
	in.SetValue(value)
	in.Placeholder = placeholder
	in.CharLimit = 256
	in.Width = width
	in.Focus()
	in.CursorEnd()
	return in
}

func textOverlay(f session.Field, current string) overlay {
	return overlay{
		kind:  overlayText,
		title: f.Key,
		field: f,
		input: newInput(current, "value", 40),
	}
}

func fontOverlay(f session.Field, current string, catalog fonts.Catalog, done bool, loadErr error) overlay {
	o := overlay{
		kind:    overlayFonts,
		title:   f.Key,
		field:   f,
		mono:    f.Mono,
		input:   newInput("", "filter families", 40),
		loadErr: loadErr,
	}
	if done && loadErr == nil {
		o.all = fontEntries(catalog.Filter("", f.Mono))
		o.entries = o.all
		o.selectCurrent(current)
	}
	return o
}

// assetOverlay picks an installed icon, cursor, GTK or Kvantum theme. Unlike
// fonts, these are a directory scan, so the list is available immediately.
func assetOverlay(f session.Field, current string) overlay {
	names := assets.List(f.Assets)

	o := overlay{
		kind:  overlayAssets,
		title: f.Key,
		field: f,
		input: newInput("", "filter "+f.Assets.String()+"s", 40),
	}
	for _, name := range names {
		e := entry{name: name}
		if assets.Builtin(f.Assets, name) {
			e.note = "built in"
		}
		o.all = append(o.all, e)
	}
	o.entries = o.all
	o.selectCurrent(current)
	return o
}

// fontEntries turns font families into picker rows.
func fontEntries(families []fonts.Family) []entry {
	out := make([]entry, 0, len(families))
	for _, f := range families {
		e := entry{name: f.Name}
		if f.Mono {
			e.note = "mono"
		}
		out = append(out, e)
	}
	return out
}

// selectCurrent puts the cursor on the value the field already holds, so
// opening a picker shows where you are rather than the top of the list.
func (o *overlay) selectCurrent(current string) {
	for i, e := range o.entries {
		if e.name == current {
			o.cursor = i
			return
		}
	}
}

func saveOverlay(suggested string, apply bool) overlay {
	title := "Save theme as"
	if apply {
		title = "Save and apply as"
	}
	return overlay{
		kind:  overlaySave,
		title: title,
		apply: apply,
		input: newInput(suggested, "theme name", 40),
	}
}

func confirmOverlay(question string, onConfirm func(*model) tea.Cmd) overlay {
	return overlay{kind: overlayConfirm, title: question, onConfirm: onConfirm}
}

// viewOverlayOf shows generated text. It is read-only on purpose: the way to
// change generated output is to change what it was generated from.
func viewOverlayOf(title, content string) overlay {
	return overlay{
		kind:  overlayView,
		title: title,
		lines: strings.Split(strings.TrimRight(content, "\n"), "\n"),
	}
}

func (o overlay) help() string {
	switch o.kind {
	case overlayText:
		return "enter accept · esc cancel"
	case overlayFonts, overlayAssets:
		return "type to filter · ↑↓ move · enter choose · esc cancel"
	case overlaySave:
		return "enter save · esc cancel"
	case overlayConfirm:
		return "y confirm · n or esc cancel"
	case overlayView:
		return "↑↓ scroll · pgup/pgdn page · y copy · esc close"
	}
	return ""
}

func (m *model) closeOverlay() { m.overlay = overlay{} }

func (m *model) updateOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.closeOverlay()
		return m, nil
	}

	switch m.overlay.kind {
	case overlayConfirm:
		switch msg.String() {
		case "y", "Y", "enter":
			fn := m.overlay.onConfirm
			m.closeOverlay()
			if fn != nil {
				return m, fn(m)
			}
		case "n", "N":
			m.closeOverlay()
		}
		return m, nil

	case overlayText:
		if msg.String() == "enter" {
			return m, m.commitText()
		}

	case overlaySave:
		if msg.String() == "enter" {
			return m, m.commitSave()
		}

	case overlayView:
		return m.updateView(msg)

	case overlayFonts, overlayAssets:
		switch msg.String() {
		case "up":
			m.overlay.cursor = clampIndex(m.overlay.cursor-1, len(m.overlay.entries))
			return m, nil
		case "down":
			m.overlay.cursor = clampIndex(m.overlay.cursor+1, len(m.overlay.entries))
			return m, nil
		case "enter":
			return m, m.commitPick()
		}
	}

	// Everything else is typing.
	var cmd tea.Cmd
	m.overlay.input, cmd = m.overlay.input.Update(msg)

	switch m.overlay.kind {
	case overlayFonts:
		if m.catalogDone && m.catalogErr == nil {
			m.overlay.all = fontEntries(m.catalog.Filter(m.overlay.input.Value(), m.overlay.mono))
			m.overlay.entries = m.overlay.all
		}
	case overlayAssets:
		m.overlay.entries = filterEntries(m.overlay.all, m.overlay.input.Value())
	}
	m.overlay.cursor = clampIndex(m.overlay.cursor, len(m.overlay.entries))
	return m, cmd
}

func (m *model) commitText() tea.Cmd {
	f := m.overlay.field
	if err := m.sess.Set(f.Key, m.overlay.input.Value()); err != nil {
		// The overlay stays open so the value can be corrected rather than
		// retyped from scratch.
		m.setStatus(levelBad, "%v", err)
		return nil
	}
	m.closeOverlay()
	m.restyle()
	value, _ := m.sess.Get(f.Key)
	m.setStatus(levelGood, "%s = %s", f.Key, value)
	return nil
}

// updateView scrolls a read-only view. Typing does nothing here, so every key
// that is not a movement is ignored rather than fed to an input.
func (m *model) updateView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	page := max(m.height-8, 1)
	last := max(len(m.overlay.lines)-1, 0)

	switch msg.String() {
	case "up", "k":
		m.overlay.offset = clampIndex(m.overlay.offset-1, len(m.overlay.lines))
	case "down", "j":
		m.overlay.offset = clampIndex(m.overlay.offset+1, len(m.overlay.lines))
	case "pgup", "b":
		m.overlay.offset = clampIndex(m.overlay.offset-page, len(m.overlay.lines))
	case "pgdown", "f", " ":
		m.overlay.offset = clampIndex(m.overlay.offset+page, len(m.overlay.lines))
	case "home", "g":
		m.overlay.offset = 0
	case "end", "G":
		m.overlay.offset = last
	case "y":
		return m, m.copySelected()
	}
	return m, nil
}

func (m *model) commitPick() tea.Cmd {
	f := m.overlay.field

	// With nothing to pick from, the filter box doubles as a plain text entry:
	// a missing fontconfig, or a theme Rice cannot see, must not make the
	// field uneditable.
	name := m.overlay.input.Value()
	if len(m.overlay.entries) > 0 {
		name = m.overlay.entries[clampIndex(m.overlay.cursor, len(m.overlay.entries))].name
	}

	if err := m.sess.Set(f.Key, name); err != nil {
		m.setStatus(levelBad, "%v", err)
		return nil
	}
	m.closeOverlay()
	m.restyle()
	m.setStatus(levelGood, "%s = %s", f.Key, name)
	return nil
}

func (m *model) commitSave() tea.Cmd {
	name := strings.TrimSpace(m.overlay.input.Value())
	apply := m.overlay.apply

	if err := session.ValidThemeName(name); err != nil {
		m.setStatus(levelBad, "%v", err)
		return nil
	}
	m.closeOverlay()

	if apply {
		return m.applyDraft(name)
	}

	path, err := m.sess.Save(name)
	if err != nil {
		m.setStatus(levelBad, "%v", err)
		return nil
	}

	// The saved theme may be new, so the picker has to be told about it.
	if list, err := m.sess.Themes(); err == nil {
		m.themes = list
	}
	m.setStatus(levelGood, "saved %s", path)
	return nil
}

func (m *model) viewOverlay() string {
	o := m.overlay

	var b strings.Builder
	b.WriteString(m.styles.title.Render(o.title) + "\n\n")

	switch o.kind {
	case overlayConfirm:
		b.WriteString(m.styles.warn.Render("y / n"))

	case overlayFonts, overlayAssets:
		b.WriteString(o.input.View() + "\n\n")
		switch {
		case o.loadErr != nil:
			b.WriteString(m.styles.warn.Render(o.loadErr.Error()) + "\n")
			b.WriteString(m.styles.subtle.Render("Type a name and press enter."))
		case o.kind == overlayFonts && !m.catalogDone:
			b.WriteString(m.styles.subtle.Render("reading installed fonts…"))
		case len(o.entries) == 0:
			b.WriteString(m.styles.subtle.Render("nothing matches; enter accepts what you typed"))
		default:
			visible, offset := m.window(len(o.entries), o.cursor)
			for i := offset; i < offset+visible; i++ {
				e := o.entries[i]
				row := e.name
				if e.note != "" {
					row += "  " + m.styles.subtle.Render(e.note)
				}
				if i == o.cursor {
					b.WriteString(m.styles.rowCursor.Render("▸ ") + m.styles.rowActive.Render(row) + "\n")
				} else {
					b.WriteString("  " + row + "\n")
				}
			}
		}

	case overlayView:
		visible := max(m.height-8, 1)
		end := min(o.offset+visible, len(o.lines))
		for _, line := range o.lines[min(o.offset, len(o.lines)):end] {
			b.WriteString(truncate(line, m.width-4) + "\n")
		}
		if len(o.lines) > visible {
			b.WriteString(m.styles.subtle.Render(fmt.Sprintf(
				"\nline %d-%d of %d", o.offset+1, end, len(o.lines))))
		}

	default:
		b.WriteString(o.input.View())
		if o.kind == overlayText && o.field.Kind == session.KindColor {
			if c, err := parseColorPreview(o.input.Value()); err == nil {
				b.WriteString("  " + swatch(c))
			}
		}
	}

	return m.styles.paneOn.Render(b.String())
}
