package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

// openCommitPreview synthesizes the pending changes, renders each as SQL via
// the driver, and shows a reviewable/prunable list before committing.
func (m Model) openCommitPreview() (tea.Model, tea.Cmd) {
	if !m.editable() {
		return m, nil
	}
	if m.readOnly() {
		m.StatusMsg = "Cannot save changes: Connection is in read-only mode"
		return m, nil
	}
	changes := buildDMLChanges(m.Browse.TableDB, m.Browse.Table, m.Browse.Columns,
		m.Browse.Rows, m.Browse.PkColumns, m.Browse.Edited, m.Browse.Deleted, m.Browse.Inserts)
	if len(changes) == 0 {
		m.StatusMsg = "No pending changes"
		return m, nil
	}
	sqls := make([]string, len(changes))
	for i, ch := range changes {
		if m.ActiveDriver == nil {
			break
		}
		s, err := m.ActiveDriver.DMLChangeToQueryString(ch)
		if err != nil {
			s = "-- error rendering: " + err.Error()
		}
		sqls[i] = s
	}
	m.PreviewChanges = changes
	m.PreviewSQL = sqls
	m.PreviewCursor = 0
	m.Screen = types.ScreenCommitPreview
	return m, nil
}

func (m Model) handleCommitPreviewScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.Screen = m.GridReturnScreen
		return m, nil
	case "up", "k":
		if m.PreviewCursor > 0 {
			m.PreviewCursor--
		}
	case "down", "j":
		if m.PreviewCursor < len(m.PreviewSQL)-1 {
			m.PreviewCursor++
		}
	case "d":
		if m.PreviewCursor < len(m.PreviewChanges) {
			m.PreviewChanges = append(m.PreviewChanges[:m.PreviewCursor], m.PreviewChanges[m.PreviewCursor+1:]...)
			m.PreviewSQL = append(m.PreviewSQL[:m.PreviewCursor], m.PreviewSQL[m.PreviewCursor+1:]...)
			if m.PreviewCursor >= len(m.PreviewSQL) {
				m.PreviewCursor = max(len(m.PreviewSQL)-1, 0)
			}
			if len(m.PreviewChanges) == 0 {
				m.Screen = m.GridReturnScreen
				m.StatusMsg = "All changes removed from this commit"
			}
		}
	case "ctrl+s", "enter", "y":
		if len(m.PreviewChanges) == 0 {
			m.Screen = m.GridReturnScreen
			return m, nil
		}
		m.Screen = m.GridReturnScreen
		m.Browse.GridLoading = true
		m.StatusMsg = "Committing..."
		return m, m.Cmds.CommitChanges(m.ActiveDriver, m.PreviewChanges, m.connIdent())
	}
	return m, nil
}

func (m Model) viewCommitPreview() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Commit %d change(s)", len(m.PreviewSQL))))
	b.WriteString("\n\n")

	innerW := min(90, max(m.Width-14, 30))
	const maxVisible = 14
	start := 0
	if m.PreviewCursor >= maxVisible {
		start = m.PreviewCursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(m.PreviewSQL))
	for i := start; i < end; i++ {
		sql := strings.ReplaceAll(m.PreviewSQL[i], "\n", " ")
		line := truncateRunes(sql, innerW)
		if i == m.PreviewCursor {
			b.WriteString(selectedRowStyle.Render("▶ " + line))
		} else {
			b.WriteString(normalStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("ctrl+s/enter: execute all  d: remove one  ↑/↓: scroll  esc: cancel"))
	return m.renderModal(b.String())
}
