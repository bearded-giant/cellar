package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func (m Model) openHistory() (tea.Model, tea.Cmd) {
	m.HistoryCursor = 0
	return m, m.Cmds.LoadHistory(m.connIdent())
}

func (m Model) handleHistoryLoadedMsg(msg types.HistoryLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.StatusMsg = "History error: " + msg.Err.Error()
		return m, nil
	}
	items := msg.Items
	sort.SliceStable(items, func(i, j int) bool { return items[i].Timestamp.After(items[j].Timestamp) })
	m.HistoryItems = items
	if len(items) == 0 {
		// nothing left (empty on open, or last entry just deleted): close the modal
		m.StatusMsg = "No query history"
		m.Screen = m.GridReturnScreen
		return m, nil
	}
	if m.HistoryCursor >= len(items) {
		m.HistoryCursor = len(items) - 1
	}
	m.Screen = types.ScreenHistory
	return m, nil
}

func (m Model) handleHistoryScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.Screen = m.GridReturnScreen
		return m, nil
	case "up", "k":
		if m.HistoryCursor > 0 {
			m.HistoryCursor--
		}
	case "down", "j":
		if m.HistoryCursor < len(m.HistoryItems)-1 {
			m.HistoryCursor++
		}
	case "d", "x":
		if m.HistoryCursor < len(m.HistoryItems) {
			it := m.HistoryItems[m.HistoryCursor]
			return m, m.Cmds.DeleteHistory(m.connIdent(), it.QueryText, it.Timestamp)
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

	const maxVisible = 18
	modalW := min(120, max(m.Width-10, 40))
	const tsW = 12 // "01-02 15:04 "
	innerW := max(modalW-6, 20)
	queryW := max(innerW-tsW-2, 10)

	start := 0
	if m.HistoryCursor >= maxVisible {
		start = m.HistoryCursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(m.HistoryItems))

	for i := start; i < end; i++ {
		it := m.HistoryItems[i]
		ts := it.Timestamp.Local().Format("01-02 15:04")
		q := truncateRunes(strings.ReplaceAll(it.QueryText, "\n", " "), queryW)
		if i == m.HistoryCursor {
			b.WriteString(selectedRowStyle.Render("▶ " + padRunes(ts+"  "+q, innerW)))
		} else {
			b.WriteString(dimStyle.Render("  "+ts+"  ") + normalStyle.Render(q))
		}
		b.WriteString("\n")
	}
	if len(m.HistoryItems) > maxVisible {
		b.WriteString(dimStyle.Render(fmt.Sprintf("\n  %d of %d", m.HistoryCursor+1, len(m.HistoryItems))))
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ select  enter:load into editor  d:delete  esc:cancel"))
	return m.renderModalW(b.String(), modalW)
}
