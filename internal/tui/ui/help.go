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
	return m, nil
}

// handleHelpScreen closes the cheatsheet on any key.
func (m Model) handleHelpScreen(_ tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.Screen = m.HelpReturnScreen
	return m, nil
}

func (m Model) viewHelp() string {
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
			{"tab", "cycle editor ⟷ results"},
			{"ctrl+t / ctrl+w", "new / close query buffer"},
			{"ctrl+pgdn/pgup", "next / prev query buffer (]/[ from results)"},
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

	var b strings.Builder
	b.WriteString(accentStyle.Render("Keybindings"))
	b.WriteString("\n")
	for _, g := range groups {
		b.WriteString("\n")
		b.WriteString(keyStyle.Render(g.title))
		b.WriteString("\n")
		for _, r := range g.rows {
			b.WriteString("  ")
			b.WriteString(normalStyle.Render(padRunes(r[0], 18)))
			b.WriteString(dimStyle.Render(r[1]))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("press any key to close"))
	return b.String()
}
