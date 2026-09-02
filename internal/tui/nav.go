package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dborovcanin/rice/internal/session"
)

// The editor is one list, not two screens. The flow is theme, then what the
// whole desktop shares, then app by app — so that flow is the navigation,
// visible the whole time, rather than a mode you reach by knowing a key.
//
// Apps sit flat under Global for now. When there are enough of them to want
// grouping by category, this is where the extra headings go.

// navKind is what a row in the navigation is.
type navKind int

const (
	// navHeading is a label. It is not selectable.
	navHeading navKind = iota
	// navGroup is a section of the global settings.
	navGroup
	// navApp is one application.
	navApp
)

// navItem is one row of the left-hand list.
type navItem struct {
	kind  navKind
	label string
	group session.Group
	app   string
}

// key identifies an item, for remembering where the cursor was inside it.
func (n navItem) key() string {
	switch n.kind {
	case navGroup:
		return "group:" + n.group.String()
	case navApp:
		return "app:" + n.app
	}
	return "heading:" + n.label
}

// selectable reports whether the cursor may rest on this row.
func (n navItem) selectable() bool { return n.kind != navHeading }

// buildNav lays out the whole editor: the global sections, then the enabled
// applications.
func buildNav(apps []string) []navItem {
	items := []navItem{{kind: navHeading, label: "GLOBAL"}}
	for _, g := range session.Groups() {
		items = append(items, navItem{kind: navGroup, label: g.String(), group: g})
	}

	if len(apps) == 0 {
		return items
	}
	items = append(items, navItem{kind: navHeading, label: "APPS"})
	for _, app := range apps {
		items = append(items, navItem{kind: navApp, label: app, app: app})
	}
	return items
}

// nav returns the item under the cursor.
func (m *model) nav() navItem {
	if len(m.items) == 0 {
		return navItem{}
	}
	return m.items[clampIndex(m.navCursor, len(m.items))]
}

// moveNav steps the cursor by delta, skipping headings, and stops at the ends
// rather than wrapping past them.
//
// Moving clears the search: a filter carried into another section would show
// it as empty, which reads as a section with nothing in it.
func (m *model) moveNav(delta int) {
	if len(m.items) == 0 {
		return
	}
	m.clearSearch()

	at := m.navCursor
	for {
		next := at + delta
		if next < 0 || next >= len(m.items) {
			return
		}
		at = next
		if m.items[at].selectable() {
			m.navCursor = at
			return
		}
	}
}

// firstSelectable is where the cursor starts.
func firstSelectable(items []navItem) int {
	for i, item := range items {
		if item.selectable() {
			return i
		}
	}
	return 0
}

// allFields are every editable field of whatever the navigation points at,
// before any filter.
func (m *model) allFields() []session.Field {
	switch item := m.nav(); item.kind {
	case navGroup:
		return session.FieldsIn(item.group)
	case navApp:
		return session.ProgramFields(item.app)
	}
	return nil
}

// fields are the fields on screen: what the navigation points at, narrowed by
// the search. SwayFX alone is nearly forty settings, which is more than
// anyone wants to walk through to reach one.
func (m *model) fields() []session.Field {
	all := m.allFields()

	query := strings.ToLower(strings.TrimSpace(m.search.Value()))
	if query == "" {
		return all
	}

	// Both the label and the key are matched: "gaps" finds Gaps inner, and
	// "sway.idle" finds every idle setting at once.
	var out []session.Field
	for _, f := range all {
		if strings.Contains(strings.ToLower(f.Label), query) ||
			strings.Contains(strings.ToLower(f.Key), query) {
			out = append(out, f)
		}
	}
	return out
}

// fieldCursor is remembered per navigation item, because moving away and back
// should return to where you were.
func (m *model) fieldCursor() int { return m.fieldCursors[m.nav().key()] }

func (m *model) setFieldCursor(i int) {
	m.fieldCursors[m.nav().key()] = clampIndex(i, len(m.fields()))
}

// field is the field under the cursor, and false when there is none.
func (m *model) field() (session.Field, bool) {
	fields := m.fields()
	if len(fields) == 0 {
		return session.Field{}, false
	}
	return fields[clampIndex(m.fieldCursor(), len(fields))], true
}

// app is the selected application, and false when a global section is
// selected instead.
func (m *model) app() (string, bool) {
	item := m.nav()
	return item.app, item.kind == navApp
}

// viewNav renders the left-hand list.
func (m *model) viewNav(width int) string {
	var b strings.Builder

	for i, item := range m.items {
		if item.kind == navHeading {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(m.styles.subtle.Render(pad(item.label, width)) + "\n")
			continue
		}

		label := "  " + item.label
		if item.kind == navApp && m.running[item.app] != nil {
			label += " ●"
		}
		row := pad(label, width)

		switch {
		case i == m.navCursor && m.pane == paneNav:
			row = m.styles.rowActive.Render(row)
		case i == m.navCursor:
			row = m.styles.rowCursor.Render(row)
		default:
			row = m.styles.row.Render(row)
		}
		b.WriteString(row + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// navWidth is how wide the left column needs to be.
func (m *model) navWidth() int {
	width := 0
	for _, item := range m.items {
		// Two for the indent, two more for the running marker.
		if n := len(item.label) + 4; n > width {
			width = n
		}
	}
	return width
}

// newSearchInput builds the field search box.
func newSearchInput() textinput.Model {
	in := textinput.New()
	in.Placeholder = "narrow these settings"
	in.CharLimit = 64
	in.Width = 30
	return in
}

// clearSearch drops the filter and takes focus off it.
func (m *model) clearSearch() {
	m.search.SetValue("")
	m.search.Blur()
	m.searching = false
}

// updateSearch handles keys while the search box has focus.
func (m *model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearSearch()
		return m, nil
	case "enter":
		// Keep the filter, but hand the keys back to the list.
		m.search.Blur()
		m.searching = false
		m.pane = paneFields
		return m, nil
	}

	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	m.setFieldCursor(m.fieldCursor())
	return m, cmd
}
