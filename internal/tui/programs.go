package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// The per-application actions. They live beside the editor rather than on a
// screen of their own: an application is a row in the navigation, so previewing
// one is something you do where you are.

func (m *model) previewSelected() tea.Cmd {
	name, ok := m.app()
	if !ok {
		m.setStatus(levelInfo, "select an application first")
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

func (m *model) viewSelected() tea.Cmd {
	name, ok := m.app()
	if !ok {
		m.setStatus(levelInfo, "select an application first")
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
	name, ok := m.app()
	if !ok {
		m.setStatus(levelInfo, "select an application first")
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
