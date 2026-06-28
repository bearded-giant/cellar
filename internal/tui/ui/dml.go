package ui

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
	"github.com/jorgerojas26/lazysql/models"
)

const readOnlyEditMsg = "Cannot modify data: Connection is in read-only mode"

// insertCell is one cell of a staged INSERT row. A default (untouched) cell
// lets the DB fill its own default; an edited cell carries a literal value.
type insertCell struct {
	val       string
	isDefault bool
}

func (b browseState) gridRowCount() int { return len(b.Rows) + len(b.Inserts) }

// editable reports whether the grid currently allows DML: a real table is
// loaded (not a query result) and we are in table view.
func (m Model) editable() bool {
	return m.Browse.Table != "" && !m.Browse.ViewJSON && len(m.Browse.Columns) > 0
}

func (m Model) readOnly() bool {
	return m.CurrentConn != nil && m.CurrentConn.ReadOnly
}

func (m Model) pendingCount() int {
	rows := map[int]bool{}
	for k := range m.Browse.Edited {
		rows[k[0]] = true
	}
	return len(rows) + len(m.Browse.Deleted) + len(m.Browse.Inserts)
}

// cellValue returns the display string for a grid cell, honoring staged edits
// and insert rows.
func (m Model) cellValue(row, col int) string {
	if row < len(m.Browse.Rows) {
		if v, ok := m.Browse.Edited[[2]int{row, col}]; ok {
			return v
		}
		if col < len(m.Browse.Rows[row]) {
			return m.Browse.Rows[row][col]
		}
		return ""
	}
	ins := m.Browse.Inserts[row-len(m.Browse.Rows)]
	if col < len(ins) {
		if ins[col].isDefault {
			return "DEFAULT"
		}
		return ins[col].val
	}
	return ""
}

func (m Model) cellStyle(row, col int) lipgloss.Style {
	if m.Focus == types.FocusGrid && row == m.Browse.RowCursor && col == m.Browse.ColCursor {
		return selectedRowStyle
	}
	if row >= len(m.Browse.Rows) {
		return dmlInsertStyle
	}
	if m.Browse.Deleted[row] {
		return dmlDeleteStyle
	}
	if _, ok := m.Browse.Edited[[2]int{row, col}]; ok {
		return dmlChangeStyle
	}
	return normalStyle
}

func (m Model) openCellEdit() (tea.Model, tea.Cmd) {
	if !m.editable() {
		return m, nil
	}
	if m.readOnly() {
		m.StatusMsg = readOnlyEditMsg
		return m, nil
	}
	ti := textinput.New()
	ti.SetValue(m.cellValue(m.Browse.RowCursor, m.Browse.ColCursor))
	ti.Width = 50
	ti.Focus()
	ti.CursorEnd()
	m.CellInput = ti
	m.Browse.EditCol = m.Browse.ColCursor
	m.Screen = types.ScreenCellEdit
	return m, nil
}

func (m Model) handleCellEditScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.Screen = types.ScreenBrowse
		return m, nil
	case "enter":
		m.applyCellEdit(m.CellInput.Value())
		m.Screen = types.ScreenBrowse
		return m, nil
	}
	var cmd tea.Cmd
	m.CellInput, cmd = m.CellInput.Update(msg)
	return m, cmd
}

func (m *Model) applyCellEdit(val string) {
	row, col := m.Browse.RowCursor, m.Browse.EditCol
	if row >= len(m.Browse.Rows) {
		idx := row - len(m.Browse.Rows)
		if idx < len(m.Browse.Inserts) && col < len(m.Browse.Inserts[idx]) {
			m.Browse.Inserts[idx][col] = insertCell{val: val}
		}
		return
	}
	original := ""
	if col < len(m.Browse.Rows[row]) {
		original = m.Browse.Rows[row][col]
	}
	key := [2]int{row, col}
	if val == original {
		delete(m.Browse.Edited, key) // edit-back-to-original cancels the change
	} else {
		m.Browse.Edited[key] = val
	}
}

func (m Model) toggleDeleteRow() (tea.Model, tea.Cmd) {
	if !m.editable() {
		return m, nil
	}
	if m.readOnly() {
		m.StatusMsg = readOnlyEditMsg
		return m, nil
	}
	row := m.Browse.RowCursor
	if row >= len(m.Browse.Rows) {
		idx := row - len(m.Browse.Rows)
		m.Browse.Inserts = append(m.Browse.Inserts[:idx], m.Browse.Inserts[idx+1:]...)
		if m.Browse.RowCursor >= m.Browse.gridRowCount() {
			m.Browse.RowCursor = max(m.Browse.gridRowCount()-1, 0)
		}
		return m, nil
	}
	if m.Browse.Deleted[row] {
		delete(m.Browse.Deleted, row) // delete is a toggle
	} else {
		m.Browse.Deleted[row] = true
	}
	return m, nil
}

func (m Model) appendInsertRow() (tea.Model, tea.Cmd) {
	if !m.editable() {
		return m, nil
	}
	if m.readOnly() {
		m.StatusMsg = readOnlyEditMsg
		return m, nil
	}
	row := make([]insertCell, len(m.Browse.Columns))
	for i := range row {
		row[i] = insertCell{isDefault: true}
	}
	m.Browse.Inserts = append(m.Browse.Inserts, row)
	m.Browse.RowCursor = m.Browse.gridRowCount() - 1
	m.Browse.ColCursor = 0
	return m, nil
}

func (m Model) discardPending() (tea.Model, tea.Cmd) {
	if m.pendingCount() == 0 {
		return m, nil
	}
	m.resetPending()
	m.StatusMsg = "Discarded pending changes"
	return m, nil
}

func (m Model) openCommitConfirm() (tea.Model, tea.Cmd) {
	if !m.editable() {
		return m, nil
	}
	if m.readOnly() {
		m.StatusMsg = "Cannot save changes: Connection is in read-only mode"
		return m, nil
	}
	if m.pendingCount() == 0 {
		m.StatusMsg = "No pending changes"
		return m, nil
	}
	m.ConfirmType = "commit_dml"
	m.ConfirmReturnScreen = types.ScreenBrowse
	m.Screen = types.ScreenConfirmDelete
	return m, nil
}

func (m Model) commitChanges() (tea.Model, tea.Cmd) {
	m.Screen = types.ScreenBrowse
	changes := buildDMLChanges(m.Browse.TableDB, m.Browse.Table, m.Browse.Columns,
		m.Browse.Rows, m.Browse.PkColumns, m.Browse.Edited, m.Browse.Deleted, m.Browse.Inserts)
	if len(changes) == 0 {
		return m, nil
	}
	m.Browse.GridLoading = true
	m.StatusMsg = "Committing..."
	return m, m.Cmds.CommitChanges(m.ActiveDriver, changes)
}

func (m Model) handleChangesCommittedMsg(msg types.ChangesCommittedMsg) (tea.Model, tea.Cmd) {
	m.Browse.GridLoading = false
	if msg.Err != nil {
		m.Browse.GridErr = "Commit failed: " + msg.Err.Error()
		m.StatusMsg = "Commit failed"
		return m, nil // keep pending so the user can fix or discard
	}
	m.resetPending()
	m.StatusMsg = fmt.Sprintf("Committed %d change(s)", msg.Count)
	return m, m.Cmds.LoadRecords(m.ActiveDriver, m.Browse.TableDB, m.Browse.Table, "", "", m.Browse.Offset, m.Browse.Limit)
}

func (m Model) handlePrimaryKeyLoadedMsg(msg types.PrimaryKeyLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Table == m.Browse.Table {
		m.Browse.PkColumns = msg.Columns
	}
	return m, nil
}

func (m Model) viewCellEdit() string {
	col := ""
	if m.Browse.EditCol < len(m.Browse.Columns) {
		col = m.Browse.Columns[m.Browse.EditCol]
	}
	body := titleStyle.Render("Edit cell") + "\n\n" +
		keyStyle.Render(col+":") + "\n" +
		m.CellInput.View() + "\n\n" +
		helpStyle.Render("enter:apply  esc:cancel")
	return m.renderModal(body)
}

// buildDMLChanges synthesizes the driver change list from the staged maps.
// Edits are grouped per row into one UPDATE; deletes and inserts each become
// one change. PrimaryKeyInfo targets existing rows (whole-row fallback when the
// table has no PK).
func buildDMLChanges(db, table string, columns []string, rows [][]string, pk []string,
	edited map[[2]int]string, deleted map[int]bool, inserts [][]insertCell,
) []models.DBDMLChange {
	var changes []models.DBDMLChange

	editsByRow := map[int][]models.CellValue{}
	for k, v := range edited {
		row, col := k[0], k[1]
		if deleted[row] {
			continue // a row staged for delete ignores its edits
		}
		editsByRow[row] = append(editsByRow[row], models.CellValue{
			Value: v, Column: columns[col], TableColumnIndex: col, TableRowIndex: row, Type: models.String,
		})
	}
	for _, row := range sortedIntKeys(editsByRow) {
		vals := editsByRow[row]
		sort.Slice(vals, func(i, j int) bool { return vals[i].TableColumnIndex < vals[j].TableColumnIndex })
		changes = append(changes, models.DBDMLChange{
			Database: db, Table: table, Type: models.DMLUpdateType,
			PrimaryKeyInfo: pkInfoForRow(columns, rows[row], pk), Values: vals,
		})
	}

	for _, row := range sortedBoolKeys(deleted) {
		changes = append(changes, models.DBDMLChange{
			Database: db, Table: table, Type: models.DMLDeleteType,
			PrimaryKeyInfo: pkInfoForRow(columns, rows[row], pk),
		})
	}

	for _, ins := range inserts {
		var vals []models.CellValue
		for ci, cell := range ins {
			col := ""
			if ci < len(columns) {
				col = columns[ci]
			}
			cv := models.CellValue{Column: col, TableColumnIndex: ci, Type: models.String, Value: cell.val}
			if cell.isDefault {
				cv.Type = models.Default
				cv.Value = "DEFAULT"
			}
			vals = append(vals, cv)
		}
		changes = append(changes, models.DBDMLChange{
			Database: db, Table: table, Type: models.DMLInsertType, Values: vals,
		})
	}
	return changes
}

func pkInfoForRow(columns, row, pk []string) []models.PrimaryKeyInfo {
	if len(pk) > 0 {
		var infos []models.PrimaryKeyInfo
		for _, name := range pk {
			idx := indexOf(columns, name)
			if idx < 0 || idx >= len(row) {
				continue
			}
			infos = append(infos, models.PrimaryKeyInfo{Name: name, Value: row[idx]})
		}
		if len(infos) > 0 {
			return infos
		}
	}
	// whole-row fallback (matches tview: WHERE every column = its value)
	infos := make([]models.PrimaryKeyInfo, 0, len(columns))
	for i, name := range columns {
		v := ""
		if i < len(row) {
			v = row[i]
		}
		infos = append(infos, models.PrimaryKeyInfo{Name: name, Value: v})
	}
	return infos
}

func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

func sortedIntKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func sortedBoolKeys(m map[int]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
