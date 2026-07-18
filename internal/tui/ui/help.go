package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func (m Model) openHelp() (tea.Model, tea.Cmd) {
	if m.Screen == types.ScreenHelp {
		return m, nil // already open (e.g. F1 pressed again)
	}
	m.HelpReturnScreen = m.Screen
	m.Screen = types.ScreenHelp
	m.HelpScroll = 0
	return m, nil
}

// handleHelpScreen scrolls the cheatsheet; any non-scroll key closes it.
func (m Model) handleHelpScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxScroll := max(0, len(m.helpLines())-m.helpViewportHeight())
	switch msg.String() {
	case "down", "j":
		m.HelpScroll = min(m.HelpScroll+1, maxScroll)
	case "up", "k":
		m.HelpScroll = max(m.HelpScroll-1, 0)
	case "pgdown", "ctrl+d", " ":
		m.HelpScroll = min(m.HelpScroll+m.helpViewportHeight(), maxScroll)
	case "pgup", "ctrl+u":
		m.HelpScroll = max(m.HelpScroll-m.helpViewportHeight(), 0)
	case "g", "home":
		m.HelpScroll = 0
	case "G", "end":
		m.HelpScroll = maxScroll
	default:
		m.Screen = m.HelpReturnScreen
	}
	return m, nil
}

// helpViewportHeight is how many content lines fit between the modal's title and
// scroll footer given the terminal height (border 2 + padding 2 + title + footer).
func (m Model) helpViewportHeight() int {
	return max(3, m.Height-8)
}

// helpLines is the flat, styled keybinding body (no title/footer) so the handler
// and view agree on line count for scroll math.
func (m Model) helpLines() []string {
	groups := []struct {
		title string
		rows  [][2]string
	}{
		{"Navigation", [][2]string{
			{"q / esc / ←", "back one level (grid → tree → confirm disconnect)"},
			{"ctrl+q", "disconnect (confirmed)"},
			{"ctrl+c", "quit cellar"},
			{"? / ctrl+g", "this help"},
		}},
		{"Connections", [][2]string{
			{"enter / b", "open (in-app browse)"},
			{"K / J", "move connection up / down"},
			{"t / a / e", "test / add / edit"},
			{"D / d / r", "duplicate / delete / reload"},
			{"/", "filter list (esc clears)"},
		}},
		{"Schema tree", [][2]string{
			{"↑/↓  j/k", "navigate"},
			{"enter  →/l", "open table / expand"},
			{"←/h", "collapse"},
			{"/", "fuzzy search"},
			{"i", "inspect object (columns / indexes / FKs / DDL)"},
			{"g / G", "top / bottom"},
			{"tab", "focus grid"},
		}},
		{"Table view", [][2]string{
			{"←/→  h/l", "select column (← at edge → tree)"},
			{"↑/↓  j/k", "move row"},
			{"n / p", "page next / prev"},
			{"s / / / i", "sort / filter / inspect"},
			{"enter / ⌫", "FK jump / back"},
			{"v / V", "peek popup / full cell view"},
			{"w", "wide cells (full values inline)"},
			{"y", "copy (cell / row)"},
			{"d / o", "DELETE / INSERT SQL → editor (review + run)"},
			{"ctrl+o", "saved queries / history picker"},
		}},
		{"Query results only", [][2]string{
			{"x / J", "export / JSON view (bounded by your LIMIT)"},
			{"", "disabled on table views — can be millions of rows"},
		}},
		{"Query workspace (e)", [][2]string{
			{"ctrl+enter / ctrl+r", "run statement at cursor"},
			{"ctrl+shift+enter", "run all statements"},
			{"tab", "cycle schema → editor → results"},
			{"ctrl+b", "toggle schema sidebar"},
			{"enter (sidebar)", "insert table/view name at cursor"},
			{"ctrl+t / ctrl+w", "new / close query buffer"},
			{"ctrl+] / ctrl+[", "next / prev query buffer (]/[ from results; ctrl+pgdn/pgup too)"},
			{"ctrl+1..ctrl+9", "jump to query buffer N"},
			{"ctrl+/", "toggle comment"},
			{"ctrl+z / ctrl+y", "undo / yank line"},
			{"ctrl+s / ctrl+shift+s", "save query / save as new name"},
			{"ctrl+o", "saved + history picker"},
			{"ctrl+space", "completion popup (auto-shows at 2+ chars)"},
			{"ctrl+n / ctrl+p", "pick completion — then ↑/↓ move, tab accepts, esc dismisses"},
			{"esc", "editor: back to tree · results: back to editor"},
			{"results: x/J/y/v/w", "export / json / copy / peek (V full) / wide"},
			{"", "buffers autosave on run; restored on reconnect"},
		}},
		{"Tabs", [][2]string{
			{"T", "open selected table in new tab"},
			{"] / [", "next / prev tab"},
			{"X", "close tab"},
		}},
		{"Global", [][2]string{
			{"?", "toggle this help"},
		}},
	}

	var lines []string
	for _, g := range groups {
		lines = append(lines, "", keyStyle.Render(g.title))
		for _, r := range g.rows {
			lines = append(lines, "  "+normalStyle.Render(padRunes(r[0], 18))+dimStyle.Render(r[1]))
		}
	}
	return lines
}

func (m Model) viewHelp() string {
	lines := m.helpLines()
	vh := m.helpViewportHeight()
	maxScroll := max(0, len(lines)-vh)
	scroll := min(max(m.HelpScroll, 0), maxScroll)

	end := min(scroll+vh, len(lines))
	window := lines[scroll:end]

	footer := "↑/↓ scroll · any other key closes"
	if maxScroll > 0 {
		up, down := " ", " "
		if scroll > 0 {
			up = "↑"
		}
		if scroll < maxScroll {
			down = "↓"
		}
		footer = up + down + "  " + footer
	}

	var b strings.Builder
	b.WriteString(accentStyle.Render("Keybindings"))
	b.WriteString("\n")
	b.WriteString(strings.Join(window, "\n"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render(footer))
	return b.String()
}
