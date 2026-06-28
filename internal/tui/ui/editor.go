package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kujtimiihoxha/vimtea"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

// newSQLEditor builds a fresh vimtea editor seeded with content. The .sql
// filename triggers vimtea's built-in SQL syntax highlighting (chroma).
func newSQLEditor(content string, w, h int) vimtea.Editor {
	ed := vimtea.NewEditor(
		vimtea.WithContent(content),
		vimtea.WithFileName("query.sql"),
		vimtea.WithEnableStatusBar(true),
		vimtea.WithEnableModeCommand(true),
	)
	sized, _ := ed.SetSize(editorSize(w, h))
	return sized.(vimtea.Editor)
}

// editorSize reserves three rows (title + footer + status) around the editor.
func editorSize(w, h int) (int, int) {
	bw := w
	if bw < 10 {
		bw = 10
	}
	bh := h - 3
	if bh < 3 {
		bh = 3
	}
	return bw, bh
}

func (m Model) openEditor() (tea.Model, tea.Cmd) {
	ed := newSQLEditor(m.EditorContent, m.Width, m.Height)
	m.Editor = ed
	m.Screen = types.ScreenEditor
	return m, ed.Init()
}

// forwardToEditor passes a message through to vimtea. Update returns the
// tea.Model interface, so the result MUST be asserted back to vimtea.Editor.
func (m Model) forwardToEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.Editor == nil {
		return m, nil
	}
	updated, cmd := m.Editor.Update(msg)
	m.Editor = updated.(vimtea.Editor)
	return m, cmd
}

func (m Model) handleEditorScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+q":
		if m.Editor != nil {
			m.EditorContent = m.Editor.GetBuffer().Text()
		}
		m.Screen = types.ScreenBrowse
		return m, nil
	case "ctrl+r":
		if m.Editor == nil {
			return m, nil
		}
		query := m.Editor.GetBuffer().Text()
		m.EditorContent = query
		if strings.TrimSpace(query) == "" {
			return m, nil
		}
		m.Browse.GridLoading = true
		m.StatusMsg = "Running query..."
		readOnly := m.CurrentConn != nil && m.CurrentConn.ReadOnly
		return m, m.Cmds.RunQuery(m.ActiveDriver, query, readOnly)
	}
	return m.forwardToEditor(msg)
}

func (m Model) handleQueryExecutedMsg(msg types.QueryExecutedMsg) (tea.Model, tea.Cmd) {
	m.Browse.GridLoading = false
	m.Screen = types.ScreenBrowse
	m.Focus = types.FocusGrid

	// Query results are not a table: blank Table/TableDB so grid paging (n/p)
	// stays disabled — ExecuteQuery returns the full set in one shot.
	m.Browse.Table = ""
	m.Browse.TableDB = ""
	m.Browse.Offset = 0
	m.Browse.RowCursor = 0
	m.resetPending()

	if msg.Err != nil {
		m.Browse.GridErr = "Query error: " + msg.Err.Error()
		m.Browse.Columns = nil
		m.Browse.Rows = nil
		m.Browse.Label = "query"
		m.StatusMsg = "Query failed"
		return m, nil
	}
	m.Browse.GridErr = ""

	if msg.IsSelect {
		m.Browse.Label = "query result"
		if len(msg.Rows) > 0 {
			m.Browse.Columns = msg.Rows[0]
			m.Browse.Rows = msg.Rows[1:]
		} else {
			m.Browse.Columns = nil
			m.Browse.Rows = nil
		}
		m.Browse.Total = msg.Total
		m.refreshJSONView()
		m.StatusMsg = fmt.Sprintf("Query OK — %d rows", msg.Total)
		return m, nil
	}

	// DML: no result grid, surface the driver's info string.
	m.Browse.Label = "query"
	m.Browse.Columns = nil
	m.Browse.Rows = nil
	m.Browse.Total = 0
	info := msg.Info
	if info == "" {
		info = "OK"
	}
	m.StatusMsg = "DML: " + info
	return m, nil
}

func (m Model) viewEditor() string {
	if m.Editor == nil {
		return ""
	}
	title := accentStyle.Render("SQL Query")
	if m.CurrentConn != nil {
		title += dimStyle.Render("  ·  " + m.CurrentConn.Name)
	}
	return title + "\n" + m.Editor.View() + "\n" + m.editorFooter() + "\n" + m.getStatusBar()
}

func (m Model) editorFooter() string {
	kb := []struct{ key, desc string }{
		{"i", "insert"}, {"esc", "normal"}, {"ctrl+r", "run"}, {"ctrl+q", "back"},
	}
	var b strings.Builder
	for i, k := range kb {
		b.WriteString(badge(k.key, "236", "255"))
		b.WriteString(" ")
		b.WriteString(dimStyle.Render(k.desc))
		if i < len(kb)-1 {
			b.WriteString("  ")
		}
	}
	return b.String()
}
