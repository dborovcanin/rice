package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dborovcanin/rice/internal/theme"
)

// styles are the lipgloss styles the interface draws with. They are rebuilt
// from the draft whenever it changes, so the editor itself wears the palette
// being edited: the closest thing to a live preview a terminal can offer for
// free.
type styles struct {
	header    lipgloss.Style
	subtle    lipgloss.Style
	paneOn    lipgloss.Style
	paneOff   lipgloss.Style
	title     lipgloss.Style
	row       lipgloss.Style
	rowActive lipgloss.Style
	rowCursor lipgloss.Style
	label     lipgloss.Style
	value     lipgloss.Style
	derived   lipgloss.Style
	changed   lipgloss.Style
	ok        lipgloss.Style
	warn      lipgloss.Style
	fail      lipgloss.Style
	keys      lipgloss.Style
}

func newStyles(t theme.Theme) styles {
	c := func(col theme.Color) lipgloss.Color { return lipgloss.Color(col.Hex()) }

	fg := c(t.Colors.Foreground)
	muted := c(t.Colors.Muted)
	primary := c(t.Colors.Primary)
	border := c(t.Colors.Border)
	surface := c(t.Colors.Surface)

	return styles{
		header:    lipgloss.NewStyle().Bold(true).Foreground(primary),
		subtle:    lipgloss.NewStyle().Foreground(muted),
		paneOn:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(primary).Padding(0, 1),
		paneOff:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1),
		title:     lipgloss.NewStyle().Bold(true).Foreground(fg),
		row:       lipgloss.NewStyle().Foreground(fg),
		rowActive: lipgloss.NewStyle().Foreground(fg).Background(surface),
		rowCursor: lipgloss.NewStyle().Foreground(primary).Bold(true),
		label:     lipgloss.NewStyle().Foreground(fg),
		value:     lipgloss.NewStyle().Foreground(muted),
		derived:   lipgloss.NewStyle().Foreground(muted).Italic(true),
		changed:   lipgloss.NewStyle().Foreground(c(t.Colors.Accent)),
		ok:        lipgloss.NewStyle().Foreground(c(t.Colors.Success)),
		warn:      lipgloss.NewStyle().Foreground(c(t.Colors.Warning)),
		fail:      lipgloss.NewStyle().Foreground(c(t.Colors.Error)),
		keys:      lipgloss.NewStyle().Foreground(muted),
	}
}

// swatch draws a block of solid color. Two cells read as a color chip at any
// font width, where one can be mistaken for a cursor artifact.
func swatch(col theme.Color) string {
	return lipgloss.NewStyle().Background(lipgloss.Color(col.Hex())).Render("  ")
}

// sample draws text in a color over a background, so a foreground and
// background pair can be judged for contrast rather than guessed at.
func sample(fg, bg theme.Color, text string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(fg.Hex())).
		Background(lipgloss.Color(bg.Hex())).
		Render(text)
}

// truecolor reports whether the terminal advertises 24-bit color. Rice checks
// the environment rather than probing, and says so when the answer is no: a
// palette editor that silently quantizes every swatch to sixteen colors is
// worse than one that admits it cannot show them.
func truecolor() bool {
	switch strings.ToLower(os.Getenv("COLORTERM")) {
	case "truecolor", "24bit":
		return true
	}
	return false
}

// pad right-pads a string to width, measured in terminal cells.
func pad(s string, width int) string {
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// truncate shortens a string to width cells, marking the cut.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// parseColorPreview parses a hex color for the live swatch beside a color
// input, where a half-typed value is expected and not an error worth showing.
func parseColorPreview(raw string) (theme.Color, error) {
	return theme.ParseColor(strings.TrimSpace(raw))
}
