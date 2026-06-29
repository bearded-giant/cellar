package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

func (m Model) openHelp() (tea.Model, tea.Cmd) {
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
		{"Connections", [][2]string{
			{"enter / b", "open (in-app browse)"},
			{"L", "open in legacy lazysql"},
			{"t / a / e", "test / add / edit"},
			{"D / d / r", "duplicate / delete / reload"},
			{"q", "quit"},
		}},
		{"Schema tree", [][2]string{
			{"↑/↓  j/k", "navigate"},
			{"enter  →/l", "open table / expand"},
			{"←/h", "collapse"},
			{"/", "fuzzy search"},
			{"g / G", "top / bottom"},
			{"tab", "focus grid"},
		}},
		{"Results grid", [][2]string{
			{"←/→  h/l", "select column"},
			{"↑/↓  j/k", "move row"},
			{"n / p", "page next / prev"},
			{"c / C", "edit cell / set NULL·EMPTY·DEFAULT"},
			{"o / d", "add row / toggle delete"},
			{"ctrl+s / u", "commit / discard"},
			{"s / / / i", "sort / filter / meta views"},
			{"enter / ⌫", "FK jump / back"},
			{"v / J", "cell view / JSON"},
			{"x / ctrl+y", "export / copy"},
		}},
		{"SQL editor (e)", [][2]string{
			{"ctrl+r", "run query"},
			{"tab", "accept completion"},
			{"ctrl+z", "undo"},
			{"ctrl+s / ctrl+q", "save / back"},
		}},
		{"Tabs", [][2]string{
			{"T", "open selected table in new tab"},
			{"] / [", "next / prev tab"},
			{"ctrl+w", "close tab"},
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
