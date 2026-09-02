package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dborovcanin/rice/internal/session"
	"github.com/dborovcanin/rice/internal/theme"
)

// swatchKeys are the colors shown as a strip beside each theme in the picker.
// They are enough to recognize a palette without opening it.
var swatchKeys = []string{
	"colors.background",
	"colors.surface",
	"colors.foreground",
	"colors.primary",
	"colors.secondary",
	"colors.accent",
	"colors.error",
}

func (m *model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.pickerCursor = clampIndex(m.pickerCursor-1, len(m.themes))
	case "down", "j":
		m.pickerCursor = clampIndex(m.pickerCursor+1, len(m.themes))
	case "home", "g":
		m.pickerCursor = 0
	case "end", "G":
		m.pickerCursor = len(m.themes) - 1

	case "enter":
		if len(m.themes) == 0 {
			m.setStatus(levelBad, "no themes found")
			return m, nil
		}
		name := m.themes[m.pickerCursor].Name
		if m.sess.Dirty() && name != m.sess.Base.Theme.Name {
			// Choosing a base discards the draft, so say so before doing it.
			m.overlay = confirmOverlay(
				"Discard unsaved changes and start from "+name+"?",
				func(mm *model) tea.Cmd { return mm.chooseTheme(name) },
			)
			return m, nil
		}
		return m, m.chooseTheme(name)
	}
	return m, nil
}

// chooseTheme makes a theme the base and moves to the editor.
func (m *model) chooseTheme(name string) tea.Cmd {
	if err := m.sess.LoadBase(name); err != nil {
		m.setStatus(levelBad, "%v", err)
		return nil
	}
	m.restyle()
	m.screen = screenEditor
	m.pane = paneNav
	m.setStatus(levelGood, "editing %s", name)
	return nil
}

func (m *model) viewPicker() string {
	if len(m.themes) == 0 {
		return m.styles.fail.Render("No themes found.")
	}

	// The name column is sized to the longest name so the swatches line up,
	// which is the whole point of showing them.
	nameWidth := 0
	for _, e := range m.themes {
		if n := len(e.Name); n > nameWidth {
			nameWidth = n
		}
	}

	var b strings.Builder
	b.WriteString(m.styles.title.Render("Themes") + "\n\n")

	visible, offset := m.window(len(m.themes), m.pickerCursor)
	for i := offset; i < offset+visible; i++ {
		e := m.themes[i]

		cursor := "  "
		if i == m.pickerCursor {
			cursor = m.styles.rowCursor.Render("▸ ")
		}

		row := pad(e.Name, nameWidth+2) + pad(string(e.Source), 9)

		// Loading every theme to draw its palette is cheap next to redrawing
		// the screen, and a picker without colors would be a list of names.
		if th, err := m.sess.ThemePreview(e.Name); err == nil {
			row += themeSwatches(th) + "  " + m.styles.subtle.Render(th.Description)
		} else {
			row += m.styles.fail.Render("unreadable: " + err.Error())
		}

		line := cursor + row
		if i == m.pickerCursor {
			line = cursor + m.styles.rowActive.Render(pad(row, m.width-4))
		}
		b.WriteString(truncate(line, m.width) + "\n")
	}
	return b.String()
}

// themeSwatches renders a theme's palette strip.
func themeSwatches(th theme.Theme) string {
	var b strings.Builder
	for _, key := range swatchKeys {
		f, ok := lookup(key)
		if !ok {
			continue
		}
		if c, isColor := f.Color(session.Draft{Theme: th}); isColor {
			b.WriteString(swatch(c))
		}
	}
	return b.String()
}

// window returns how many rows fit and where the list should start so the
// cursor stays visible. The reserve is what the surrounding chrome takes:
// header, pane borders, status and help.
func (m *model) window(total, cursor int) (visible, offset int) {
	return m.windowWith(total, cursor, 8)
}

func (m *model) windowWith(total, cursor, reserve int) (visible, offset int) {
	visible = m.height - reserve
	if visible < 1 {
		visible = 1
	}
	if visible > total {
		visible = total
	}

	offset = cursor - visible/2
	if offset < 0 {
		offset = 0
	}
	if offset+visible > total {
		offset = total - visible
	}
	return visible, offset
}

func clampIndex(i, length int) int {
	if length == 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= length {
		return length - 1
	}
	return i
}
