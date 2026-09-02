package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) updatePrograms(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.programCursor = clampIndex(m.programCursor-1, len(m.programs))
	case "down", "j":
		m.programCursor = clampIndex(m.programCursor+1, len(m.programs))

	case "esc", "q", "g":
		m.screen = screenEditor
		return m, nil

	case "p", "enter":
		return m, m.previewSelected()

	case "y":
		return m, m.copySelected()

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

	var b strings.Builder
	b.WriteString(m.styles.title.Render("Programs") + "\n")
	b.WriteString(m.styles.subtle.Render(
		"Preview renders the draft into a private directory and runs the real program against it. "+
			"Nothing under ~/.config is touched.") + "\n\n")

	for i, name := range m.programs {
		prefix := "  "
		if i == m.programCursor {
			prefix = m.styles.rowCursor.Render("▸ ")
		}

		row := pad(name, nameWidth+2)

		var note string
		var style = m.styles.subtle
		switch {
		case m.running[name] != nil:
			note, style = "running · x to stop", m.styles.ok
		default:
			l, err := m.sess.LaunchFor(name)
			switch {
			case err != nil:
				note, style = err.Error(), m.styles.warn
			case l.Confirm != "":
				note, style = "asks first: "+l.Confirm, m.styles.warn
			case l.Note != "":
				note = l.Note
			default:
				note = l.Binary + " " + strings.Join(l.Args("<sandbox>"), " ")
			}
		}

		line := prefix + row + style.Render(note)
		if i == m.programCursor {
			line = prefix + m.styles.rowActive.Render(pad(row, nameWidth+2)) + " " + style.Render(note)
		}
		b.WriteString(truncate(line, m.width) + "\n")
	}
	return b.String()
}
