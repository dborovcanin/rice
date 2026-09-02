package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dborovcanin/rice/internal/session"
)

// lookup resolves a field by key, and is the only place the interface reaches
// into the field table.
func lookup(key string) (session.Field, bool) { return session.LookupField(key) }

// group returns the group under the group cursor.
func (m *model) group() session.Group {
	groups := session.Groups()
	return groups[clampIndex(m.groupCursor, len(groups))]
}

// fields returns the fields of the current group.
func (m *model) fields() []session.Field { return session.FieldsIn(m.group()) }

// field returns the field under the field cursor, and false when the group is
// somehow empty.
func (m *model) field() (session.Field, bool) {
	fields := m.fields()
	if len(fields) == 0 {
		return session.Field{}, false
	}
	return fields[clampIndex(m.fieldCursor(), len(fields))], true
}

func (m *model) fieldCursor() int { return m.fieldCursors[m.group()] }

func (m *model) setFieldCursor(i int) {
	m.fieldCursors[m.group()] = clampIndex(i, len(m.fields()))
}

func (m *model) updateEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		if m.pane == paneGroups {
			m.pane = paneFields
		} else {
			m.pane = paneGroups
		}
		return m, nil
	case "shift+tab":
		if m.pane == paneFields {
			m.pane = paneGroups
		} else {
			m.pane = paneFields
		}
		return m, nil

	case "left", "h":
		if m.pane == paneGroups {
			return m, nil
		}
		return m, m.nudge(-1)
	case "right", "l":
		if m.pane == paneGroups {
			return m, nil
		}
		return m, m.nudge(1)

	case "up", "k":
		if m.pane == paneGroups {
			m.groupCursor = clampIndex(m.groupCursor-1, len(session.Groups()))
		} else {
			m.setFieldCursor(m.fieldCursor() - 1)
		}
		return m, nil
	case "down", "j":
		if m.pane == paneGroups {
			m.groupCursor = clampIndex(m.groupCursor+1, len(session.Groups()))
		} else {
			m.setFieldCursor(m.fieldCursor() + 1)
		}
		return m, nil

	case "enter":
		if m.pane == paneGroups {
			m.pane = paneFields
			return m, nil
		}
		return m.editField()

	case "r":
		return m, m.withField(func(f session.Field) {
			if err := m.sess.Reset(f.Key); err != nil {
				m.setStatus(levelBad, "%v", err)
				return
			}
			m.restyle()
			m.setStatus(levelInfo, "%s reset to %s", f.Key, m.sess.Base.Theme.Name)
		})

	case "c":
		return m, m.withField(func(f session.Field) {
			if err := m.sess.Clear(f.Key); err != nil {
				m.setStatus(levelBad, "%v", err)
				return
			}
			m.restyle()
			value, _ := m.sess.Get(f.Key)
			m.setStatus(levelInfo, "%s is derived again: %s", f.Key, value)
		})

	case "R":
		m.overlay = confirmOverlay("Discard every change to this theme?", func(mm *model) tea.Cmd {
			mm.sess.ResetAll()
			mm.restyle()
			mm.setStatus(levelInfo, "all changes discarded")
			return nil
		})
		return m, nil

	case "g":
		m.screen = screenPrograms
		return m, nil
	case "t", "q", "esc":
		m.screen = screenPicker
		return m, nil

	case "y":
		return m, m.copyTheme()

	case "d":
		return m, m.showDiff()

	case "s":
		m.overlay = saveOverlay(m.suggestedName(), false)
		return m, nil
	case "a":
		m.overlay = saveOverlay(m.suggestedName(), true)
		return m, nil
	}
	return m, nil
}

// withField runs an action against the focused field, or reports why it could
// not. It keeps every field action from repeating the same guard.
func (m *model) withField(fn func(session.Field)) tea.Cmd {
	f, ok := m.field()
	if !ok {
		m.setStatus(levelBad, "no field selected")
		return nil
	}
	if m.pane != paneFields {
		m.setStatus(levelInfo, "select a field first")
		return nil
	}
	fn(f)
	return nil
}

func (m *model) nudge(steps int) tea.Cmd {
	return m.withField(func(f session.Field) {
		if err := m.sess.Nudge(f.Key, steps); err != nil {
			m.setStatus(levelInfo, "%v", err)
			return
		}
		m.restyle()
		value, _ := m.sess.Get(f.Key)
		m.setStatus(levelInfo, "%s = %s", f.Key, value)
	})
}

// editField opens the right editor for the focused field: a font list for a
// font family, a text input for everything else.
func (m *model) editField() (tea.Model, tea.Cmd) {
	f, ok := m.field()
	if !ok {
		return m, nil
	}
	current, _ := m.sess.Get(f.Key)

	switch {
	case f.Kind == session.KindFont:
		m.overlay = fontOverlay(f, current, m.catalog, m.catalogDone, m.catalogErr)
		return m, m.loadFonts()
	case f.PicksAssets:
		m.overlay = assetOverlay(f, current)
		return m, nil
	}

	m.overlay = textOverlay(f, current)
	return m, nil
}

// showDiff answers the question the editor cannot otherwise answer: not what
// the draft says, but what applying it would actually change.
func (m *model) showDiff() tea.Cmd {
	text, err := m.sess.Diff(3)
	if err != nil {
		m.setStatus(levelBad, "%v", err)
		return nil
	}
	if strings.TrimSpace(text) == "" {
		m.setStatus(levelInfo, "the draft matches what is deployed")
		return nil
	}

	m.overlay = viewOverlayOf("what applying would change", text)
	m.setStatus(levelInfo, "showing the difference against the deployed generation")
	return nil
}

func (m *model) copyTheme() tea.Cmd {
	tool, err := m.sess.CopyTheme(context.Background())
	if err != nil {
		m.setStatus(levelBad, "copy: %v", err)
		return nil
	}
	m.setStatus(levelGood, "theme copied to the clipboard with %s", tool)
	return nil
}

// suggestedName is the name offered when saving: the base name for a theme
// the user already owns, and a "-custom" variant for a bundled one, so saving
// a bundled theme does not silently shadow it unless that is asked for.
func (m *model) suggestedName() string {
	name := m.sess.Base.Theme.Name
	if name == "" {
		return "my-theme"
	}
	for _, e := range m.themes {
		if e.Name == name && e.Source == "builtin" {
			return name + "-custom"
		}
	}
	return name
}

func (m *model) viewEditor() string {
	groups := session.Groups()

	groupWidth := 0
	for _, g := range groups {
		if n := len(g.String()); n > groupWidth {
			groupWidth = n
		}
	}

	var left strings.Builder
	for i, g := range groups {
		row := pad(g.String(), groupWidth)
		switch {
		case i == m.groupCursor && m.pane == paneGroups:
			row = m.styles.rowActive.Render(row)
		case i == m.groupCursor:
			row = m.styles.rowCursor.Render(row)
		default:
			row = m.styles.row.Render(row)
		}
		left.WriteString(row + "\n")
	}

	groupPane := m.styles.paneOff
	fieldPane := m.styles.paneOff
	if m.pane == paneGroups {
		groupPane = m.styles.paneOn
	} else {
		fieldPane = m.styles.paneOn
	}

	fieldsWidth := m.width - groupWidth - 8
	if fieldsWidth < 20 {
		fieldsWidth = 20
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		groupPane.Render(strings.TrimRight(left.String(), "\n")),
		fieldPane.Render(m.viewFields(fieldsWidth)),
	)
}

func (m *model) viewFields(width int) string {
	fields := m.fields()
	if len(fields) == 0 {
		return "no fields"
	}

	layout := m.measure(fields)
	cursor := m.fieldCursor()
	visible, offset := m.window(len(fields), cursor)

	var b strings.Builder
	for i := offset; i < offset+visible; i++ {
		b.WriteString(m.fieldRow(fields[i], layout, width,
			i == cursor, i == cursor && m.pane == paneFields) + "\n")
	}

	if detail := m.viewFieldDetail(width); detail != "" {
		b.WriteString("\n" + detail)
	}
	return strings.TrimRight(b.String(), "\n")
}

// columns is how wide each part of a field row should be, measured across the
// whole list so nothing runs into what follows it.
type columns struct{ label, value int }

// measure sizes the columns to their longest entry.
func (m *model) measure(fields []session.Field) columns {
	c := columns{value: 8}
	for _, f := range fields {
		if n := len(f.Label); n > c.label {
			c.label = n
		}
		if n := len(f.Display(m.sess.Resolved())); n > c.value {
			c.value = n
		}
	}
	return c
}

// fieldRow renders one editable field. Both panes use it, so a colour has a
// swatch and a missing theme is marked wherever it is edited.
func (m *model) fieldRow(f session.Field, layout columns, width int, cursor, focused bool) string {
	value, _ := m.sess.Get(f.Key)
	if value == "" {
		value = "—"
	}

	row := pad(f.Label, layout.label+2)
	if c, ok := m.sess.Color(f.Key); ok {
		row += swatch(c) + " "
	}
	row += pad(value, layout.value+2)

	switch {
	case m.sess.Missing(f.Key):
		// A theme naming an icon set nobody has installed renders and deploys
		// perfectly; the only symptom is that nothing changes.
		row += m.styles.fail.Render("not installed ")
	case m.sess.Overridden(f.Key):
		row += m.styles.changed.Render("changed ")
	case !f.Explicit(m.sess.Draft):
		row += m.styles.derived.Render("derived ")
	default:
		row += pad("", 14)
	}
	if f.Help != "" {
		row += m.styles.subtle.Render(f.Help)
	}

	prefix := "  "
	if cursor {
		prefix = m.styles.rowCursor.Render("▸ ")
	}

	line := pad(row, width-2)
	if focused {
		line = m.styles.rowActive.Render(line)
	}
	return truncate(prefix+line, width)
}

// viewFieldDetail shows what the focused field means in practice: for a
// foreground or background color, how readable the pair actually is.
func (m *model) viewFieldDetail(width int) string {
	f, ok := m.field()
	if !ok {
		return ""
	}

	c, isColor := m.sess.Color(f.Key)
	if !isColor {
		return ""
	}

	resolved := m.sess.Theme()
	bg := resolved.Colors.Background
	fg := resolved.Colors.Foreground

	line := sample(c, bg, " text on background ") + " " + sample(fg, c, " text on this color ")
	return truncate(line, width)
}
