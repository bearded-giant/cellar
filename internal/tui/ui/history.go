package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

// openHistory opens the history side of the query picker; tab/h/l toggles back
// to saved queries.
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
	// an empty list still opens (renders "(empty)") so the saved side stays
	// reachable via toggle
	m.HistoryItems = items
	if m.HistoryCursor >= len(items) {
		m.HistoryCursor = max(len(items)-1, 0)
	}
	m.Screen = types.ScreenHistory
	return m, nil
}

func (m Model) handleHistoryScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.Screen = m.GridReturnScreen
		return m, nil
	case "tab", "h", "l":
		return m.openSavedQueries()
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
			// recalled query is a fresh scratch buffer (no saved binding)
			return m.openQueryInEditor(m.HistoryItems[m.HistoryCursor].QueryText, "", "")
		}
	}
	return m, nil
}

func (m Model) viewHistory() string {
	var b strings.Builder
	b.WriteString(pickerHeader(true))
	b.WriteString("\n\n")
	if len(m.HistoryItems) == 0 {
		b.WriteString(dimStyle.Render("(empty)"))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("tab:saved  esc:close"))
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
	b.WriteString(helpStyle.Render("↑/↓ select  enter:load into editor  d:delete  tab:saved  esc:close"))
	return m.renderModalW(b.String(), modalW)
}
