package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

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
	// overlaySave asks for a theme name, and optionally applies afterwards.
	overlaySave
	// overlayConfirm asks a yes-or-no question before something irreversible
	// or surprising.
	overlayConfirm
)

// overlay is the modal state. Only one is ever open, so this is a single
// struct rather than a stack: nothing in the editor needs a modal over a modal.
type overlay struct {
	kind  overlayKind
	title string
	input textinput.Model

	// field is what overlayText and overlayFonts are editing.
	field session.Field

	// font picker state.
	families []fonts.Family
	cursor   int
	mono     bool
	loading  bool
	loadErr  error

	// apply marks a save overlay that should build a generation afterwards.
	apply bool

	// onConfirm runs when a confirm overlay is accepted.
	onConfirm func(*model) tea.Cmd
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
		loading: !done,
		loadErr: loadErr,
	}
	if done && loadErr == nil {
		o.families = catalog.Filter("", f.Mono)
		for i, fam := range o.families {
			if fam.Name == current {
				o.cursor = i
			}
		}
	}
	return o
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

func (o overlay) help() string {
	switch o.kind {
	case overlayText:
		return "enter accept · esc cancel"
	case overlayFonts:
		return "type to filter · ↑↓ move · enter choose · esc cancel"
	case overlaySave:
		return "enter save · esc cancel"
	case overlayConfirm:
		return "y confirm · n or esc cancel"
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

	case overlayFonts:
		switch msg.String() {
		case "up":
			m.overlay.cursor = clampIndex(m.overlay.cursor-1, len(m.overlay.families))
			return m, nil
		case "down":
			m.overlay.cursor = clampIndex(m.overlay.cursor+1, len(m.overlay.families))
			return m, nil
		case "enter":
			return m, m.commitFont()
		}
	}

	// Everything else is typing.
	var cmd tea.Cmd
	m.overlay.input, cmd = m.overlay.input.Update(msg)

	if m.overlay.kind == overlayFonts && m.catalogDone && m.catalogErr == nil {
		m.overlay.families = m.catalog.Filter(m.overlay.input.Value(), m.overlay.mono)
		m.overlay.cursor = clampIndex(m.overlay.cursor, len(m.overlay.families))
	}
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

func (m *model) commitFont() tea.Cmd {
	f := m.overlay.field

	// With no catalog, the filter box doubles as a plain text entry, so a
	// missing fontconfig does not make font fields uneditable.
	name := m.overlay.input.Value()
	if len(m.overlay.families) > 0 {
		name = m.overlay.families[clampIndex(m.overlay.cursor, len(m.overlay.families))].Name
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

	path, err := m.sess.SaveTheme(name)
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

	case overlayFonts:
		b.WriteString(o.input.View() + "\n\n")
		switch {
		case o.loadErr != nil:
			b.WriteString(m.styles.warn.Render(o.loadErr.Error()) + "\n")
			b.WriteString(m.styles.subtle.Render("Type a family name and press enter."))
		case !m.catalogDone:
			b.WriteString(m.styles.subtle.Render("reading installed fonts…"))
		case len(o.families) == 0:
			b.WriteString(m.styles.subtle.Render("no family matches; enter accepts what you typed"))
		default:
			visible, offset := m.window(len(o.families), o.cursor)
			for i := offset; i < offset+visible; i++ {
				fam := o.families[i]
				row := fam.Name
				if fam.Mono {
					row += "  " + m.styles.subtle.Render("mono")
				}
				if i == o.cursor {
					b.WriteString(m.styles.rowCursor.Render("▸ ") + m.styles.rowActive.Render(row) + "\n")
				} else {
					b.WriteString("  " + row + "\n")
				}
			}
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
