package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func (b browseState) gridRowCount() int { return len(b.Rows) }

// cellValue returns the display string for a grid cell.
func (m Model) cellValue(row, col int) string {
	if row < 0 || row >= len(m.Browse.Rows) {
		return ""
	}
	if col < len(m.Browse.Rows[row]) {
		return m.Browse.Rows[row][col]
	}
	return ""
}

func (m Model) cellStyle(row, col int) lipgloss.Style {
	if m.Focus == types.FocusGrid && row == m.Browse.RowCursor && col == m.Browse.ColCursor {
		return selectedRowStyle
	}
	return normalStyle
}

func (m Model) handlePrimaryKeyLoadedMsg(msg types.PrimaryKeyLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Table == m.Browse.Table {
		m.Browse.PkColumns = msg.Columns
	}
	return m, nil
}

// generateDelete writes a DELETE for the cursor row into the SQL editor (it is
// not executed — the user reviews and runs it). WHERE targets the primary key,
// falling back to every column when the table has none.
func (m Model) generateDelete() (tea.Model, tea.Cmd) {
	if m.Browse.Table == "" || m.Browse.MetaKind != metaRecords || m.ActiveDriver == nil {
		return m, nil
	}
	if m.Browse.IsView {
		m.StatusMsg = "Views are read-only — no DELETE generation"
		return m, nil
	}
	if m.Browse.RowCursor < 0 || m.Browse.RowCursor >= len(m.Browse.Rows) {
		m.StatusMsg = "No row selected"
		return m, nil
	}
	row := m.Browse.Rows[m.Browse.RowCursor]
	stmt := fmt.Sprintf("DELETE FROM %s\nWHERE %s;", m.Browse.Table, m.rowWhereClause(row))
	return m.appendToEditor(stmt)
}

// generateInsert writes an INSERT template (column list + <col> placeholders)
// for the current table into the SQL editor.
func (m Model) generateInsert() (tea.Model, tea.Cmd) {
	if m.Browse.Table == "" || m.Browse.MetaKind != metaRecords || len(m.Browse.Columns) == 0 {
		return m, nil
	}
	if m.Browse.IsView {
		m.StatusMsg = "Views are read-only — no INSERT generation"
		return m, nil
	}
	placeholders := make([]string, len(m.Browse.Columns))
	for i, c := range m.Browse.Columns {
		placeholders[i] = "<" + c + ">"
	}
	stmt := fmt.Sprintf("INSERT INTO %s (%s)\nVALUES (%s);",
		m.Browse.Table, strings.Join(m.Browse.Columns, ", "), strings.Join(placeholders, ", "))
	return m.appendToEditor(stmt)
}

// rowWhereClause builds an equality WHERE for a row: primary-key columns when
// known, else every column (matches the row but verbose). NULL cells use IS NULL.
func (m Model) rowWhereClause(row []string) string {
	cols := m.Browse.PkColumns
	if len(cols) == 0 {
		cols = m.Browse.Columns
	}
	var parts []string
	for _, name := range cols {
		idx := columnIndex(m.Browse.Columns, name)
		if idx < 0 || idx >= len(row) {
			continue
		}
		raw := row[idx]
		if raw == "NULL&" {
			parts = append(parts, name+" IS NULL")
			continue
		}
		parts = append(parts, name+" = "+m.ActiveDriver.FormatArgForQueryString(displayCell(raw)))
	}
	return strings.Join(parts, " AND ")
}

// appendToEditor appends a statement to the editor buffer (blank line between
// statements) and opens the query workspace focused on it.
func (m Model) appendToEditor(stmt string) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(m.EditorContent) == "" {
		m.EditorContent = stmt
	} else {
		m.EditorContent = strings.TrimRight(m.EditorContent, "\n") + "\n\n" + stmt
	}
	m.StatusMsg = "Added SQL to the editor — review, then ctrl+r to run"
	return m.openEditor()
}

func columnIndex(cols []string, name string) int {
	for i, c := range cols {
		if c == name {
			return i
		}
	}
	return -1
}
