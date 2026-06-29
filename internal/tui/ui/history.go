package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func (m Model) openHistory() (tea.Model, tea.Cmd) {
	return m, m.Cmds.LoadHistory(m.connIdent())
}

func (m Model) handleHistoryLoadedMsg(msg types.HistoryLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.StatusMsg = "History error: " + msg.Err.Error()
		return m, nil
	}
	items := msg.Items
	sort.SliceStable(items, func(i, j int) bool { return items[i].Timestamp.After(items[j].Timestamp) })
	if len(items) == 0 {
		m.StatusMsg = "No query history yet"
		return m, nil
	}
	m.HistoryItems = items
	m.HistoryCursor = 0
	m.Screen = types.ScreenHistory
	return m, nil
}

func (m Model) handleHistoryScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.Screen = types.ScreenBrowse
		return m, nil
	case "up", "k":
		if m.HistoryCursor > 0 {
			m.HistoryCursor--
		}
	case "down", "j":
		if m.HistoryCursor < len(m.HistoryItems)-1 {
			m.HistoryCursor++
		}
	case "enter":
		if m.HistoryCursor < len(m.HistoryItems) {
			m.EditorContent = m.HistoryItems[m.HistoryCursor].QueryText
			return m.openEditor()
		}
	}
	return m, nil
}

func (m Model) viewHistory() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Query History"))
	b.WriteString("\n\n")
	if len(m.HistoryItems) == 0 {
		b.WriteString(dimStyle.Render("(empty)"))
		return m.renderModal(b.String())
	}

	const maxVisible = 12
	start := 0
	if m.HistoryCursor >= maxVisible {
		start = m.HistoryCursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(m.HistoryItems))
	innerW := min(80, max(m.Width-14, 30))

	for i := start; i < end; i++ {
		q := strings.ReplaceAll(m.HistoryItems[i].QueryText, "\n", " ")
		line := truncateRunes(q, innerW)
		if i == m.HistoryCursor {
			b.WriteString(selectedRowStyle.Render("▶ " + line))
		} else {
			b.WriteString(normalStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}
	if len(m.HistoryItems) > maxVisible {
		b.WriteString(dimStyle.Render(fmt.Sprintf("\n  %d of %d", m.HistoryCursor+1, len(m.HistoryItems))))
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ select  enter:load into editor  esc:cancel"))
	return m.renderModal(b.String())
}
