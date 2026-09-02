package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dborovcanin/rice/internal/session"
)

// programFields are the settings of the program under the cursor. They come
// from config.toml rather than the theme, because a bar's height and a
// launcher's width are structure, not appearance, and should survive a change
// of palette.
func (m *model) programFields() []session.Field {
	name, ok := m.selected()
	if !ok {
		return nil
	}
	return session.ProgramFields(name)
}

func (m *model) programFieldCursor() int {
	name, _ := m.selected()
	return m.programCursors[name]
}

func (m *model) setProgramFieldCursor(i int) {
	name, ok := m.selected()
	if !ok {
		return
	}
	m.programCursors[name] = clampIndex(i, len(m.programFields()))
}

// programField is the setting under the cursor.
func (m *model) programField() (session.Field, bool) {
	fields := m.programFields()
	if len(fields) == 0 {
		return session.Field{}, false
	}
	return fields[clampIndex(m.programFieldCursor(), len(fields))], true
}

func (m *model) updatePrograms(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab":
		if m.programPane == paneGroups && len(m.programFields()) > 0 {
			m.programPane = paneFields
		} else {
			m.programPane = paneGroups
		}
		return m, nil

	case "up", "k":
		if m.programPane == paneGroups {
			m.programCursor = clampIndex(m.programCursor-1, len(m.programs))
		} else {
			m.setProgramFieldCursor(m.programFieldCursor() - 1)
		}
		return m, nil
	case "down", "j":
		if m.programPane == paneGroups {
			m.programCursor = clampIndex(m.programCursor+1, len(m.programs))
		} else {
			m.setProgramFieldCursor(m.programFieldCursor() + 1)
		}
		return m, nil

	case "left", "h":
		if m.programPane == paneFields {
			return m, m.nudgeProgram(-1)
		}
		return m, nil
	case "right", "l":
		if m.programPane == paneFields {
			return m, m.nudgeProgram(1)
		}
		return m, nil

	case "enter":
		if m.programPane == paneGroups {
			if len(m.programFields()) > 0 {
				m.programPane = paneFields
			}
			return m, nil
		}
		return m.editProgramField()

	case "r":
		return m, m.withProgramField(func(f session.Field) {
			if err := m.sess.Reset(f.Key); err != nil {
				m.setStatus(levelBad, "%v", err)
				return
			}
			value, _ := m.sess.Get(f.Key)
			m.setStatus(levelInfo, "%s reset to %s", f.Key, value)
		})

	case "esc", "q", "g":
		m.screen = screenEditor
		return m, nil

	case "s":
		m.overlay = saveOverlay(m.suggestedName(), false)
		return m, nil

	case "p":
		return m, m.previewSelected()
	case "y":
		return m, m.copySelected()
	case "v":
		return m, m.viewSelected()
	case "x":
		return m, m.stopSelected()
	}
	return m, nil
}

// selected is the component under the cursor.
func (m *model) selected() (string, bool) {
	if len(m.programs) == 0 {
		return "", false
	}
	return m.programs[clampIndex(m.programCursor, len(m.programs))], true
}

func (m *model) withProgramField(fn func(session.Field)) tea.Cmd {
	f, ok := m.programField()
	if !ok || m.programPane != paneFields {
		m.setStatus(levelInfo, "select a setting first")
		return nil
	}
	fn(f)
	return nil
}

func (m *model) nudgeProgram(steps int) tea.Cmd {
	return m.withProgramField(func(f session.Field) {
		if err := m.sess.Nudge(f.Key, steps); err != nil {
			m.setStatus(levelInfo, "%v", err)
			return
		}
		value, _ := m.sess.Get(f.Key)
		m.setStatus(levelInfo, "%s = %s", f.Key, value)
	})
}

func (m *model) editProgramField() (tea.Model, tea.Cmd) {
	f, ok := m.programField()
	if !ok {
		return m, nil
	}

	// A switch and a fixed set of choices are faster to cycle than to type.
	switch f.Kind {
	case session.KindBool, session.KindChoice:
		return m, m.nudgeProgram(1)
	}

	current, _ := m.sess.Get(f.Key)
	m.overlay = textOverlay(f, current)
	return m, nil
}

func (m *model) previewSelected() tea.Cmd {
	name, ok := m.selected()
	if !ok {
		return nil
	}

	l, err := m.sess.LaunchFor(name)
	if err != nil {
		m.setStatus(levelBad, "%v", err)
		return nil
	}
	if l.Confirm != "" {
		m.overlay = confirmOverlay(
			"Preview "+name+"? "+l.Confirm,
			func(mm *model) tea.Cmd { return mm.startPreview(name, true) },
		)
		return nil
	}
	return m.startPreview(name, false)
}

func (m *model) copySelected() tea.Cmd {
	name, ok := m.selected()
	if !ok {
		return nil
	}

	tool, err := m.sess.CopyComponent(context.Background(), name)
	if err != nil {
		m.setStatus(levelBad, "copy %s: %v", name, err)
		return nil
	}
	m.setStatus(levelGood, "%s configuration copied to the clipboard with %s", name, tool)
	return nil
}

// viewSelected shows a component's generated configuration, which is the
// question "what does this actually produce" answered without leaving the
// editor or writing a file.
func (m *model) viewSelected() tea.Cmd {
	name, ok := m.selected()
	if !ok {
		return nil
	}

	text, err := m.sess.ComponentText(name)
	if err != nil {
		m.setStatus(levelBad, "%v", err)
		return nil
	}
	m.overlay = viewOverlayOf(name+" — generated", text)
	m.setStatus(levelInfo, "showing what %s would generate", name)
	return nil
}

func (m *model) stopSelected() tea.Cmd {
	name, ok := m.selected()
	if !ok {
		return nil
	}
	p, running := m.running[name]
	if !running {
		m.setStatus(levelInfo, "%s is not previewing", name)
		return nil
	}
	if err := p.Stop(); err != nil {
		m.setStatus(levelBad, "stop %s: %v", name, err)
		return nil
	}
	m.setStatus(levelInfo, "stopped the %s preview", name)
	return nil
}

func (m *model) viewPrograms() string {
	if len(m.programs) == 0 {
		return m.styles.fail.Render("No components are enabled in config.toml.")
	}

	nameWidth := 0
	for _, name := range m.programs {
		if n := len(name); n > nameWidth {
			nameWidth = n
		}
	}

	var left strings.Builder
	for i, name := range m.programs {
		row := pad(name, nameWidth)
		if m.running[name] != nil {
			row = pad(name+" ●", nameWidth+2)
		}
		switch {
		case i == m.programCursor && m.programPane == paneGroups:
			row = m.styles.rowActive.Render(row)
		case i == m.programCursor:
			row = m.styles.rowCursor.Render(row)
		default:
			row = m.styles.row.Render(row)
		}
		left.WriteString(row + "\n")
	}

	listPane := m.styles.paneOff
	fieldPane := m.styles.paneOff
	if m.programPane == paneGroups {
		listPane = m.styles.paneOn
	} else {
		fieldPane = m.styles.paneOn
	}

	width := m.width - nameWidth - 10
	if width < 24 {
		width = 24
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		listPane.Render(strings.TrimRight(left.String(), "\n")),
		fieldPane.Render(m.viewProgramDetail(width)),
	)
}

func (m *model) viewProgramDetail(width int) string {
	name, ok := m.selected()
	if !ok {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.styles.title.Render(name) + "\n")
	b.WriteString(truncate(m.previewLine(name), width) + "\n\n")

	if note := session.Note(name); note != "" {
		b.WriteString(m.styles.subtle.Render(truncate(note, width)) + "\n\n")
	}

	fields := session.ProgramFields(name)
	if len(fields) == 0 {
		b.WriteString(m.styles.subtle.Render("Nothing to set here; preview and copy still work."))
		return b.String()
	}

	layout := m.measure(fields)
	cursor := m.programFieldCursor()
	visible, offset := m.windowWith(len(fields), cursor, 10)

	for i := offset; i < offset+visible; i++ {
		b.WriteString(m.fieldRow(fields[i], layout, width,
			i == cursor, i == cursor && m.programPane == paneFields) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// previewLine says what previewing this program would do, or why it cannot.
func (m *model) previewLine(name string) string {
	if m.running[name] != nil {
		return m.styles.ok.Render("previewing · x stops it")
	}

	l, err := m.sess.LaunchFor(name)
	switch {
	case err != nil:
		return m.styles.warn.Render(err.Error())
	case l.Confirm != "":
		return m.styles.warn.Render("p asks first: " + l.Confirm)
	case l.Note != "":
		return m.styles.warn.Render("p previews · " + l.Note)
	default:
		return m.styles.subtle.Render("p previews · " + l.Binary + " " + strings.Join(l.Args("<sandbox>"), " "))
	}
}
