package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/commands"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

// metadata views shown in the grid (metaRecords is the normal editable table).
const (
	metaRecords = iota
	metaColumns
	metaConstraints
	metaIndexes
	metaForeignKeys
)

var metaNames = []string{"records", "columns", "constraints", "indexes", "foreign keys"}

// reloadRecords re-fetches the current table page with the active where/sort.
func (m Model) reloadRecords() (tea.Model, tea.Cmd) {
	if m.Browse.Table == "" {
		return m, nil
	}
	m.Browse.GridLoading = true
	return m, m.Cmds.LoadRecords(m.ActiveDriver, m.Browse.TableDB, m.Browse.Table,
		m.Browse.Where, m.Browse.Sort, m.Browse.Offset, m.Browse.Limit)
}

// cycleSort rotates the sort on the current column: none -> ASC -> DESC -> none.
func (m Model) cycleSort() (tea.Model, tea.Cmd) {
	if m.Browse.Table == "" || m.Browse.MetaKind != metaRecords || len(m.Browse.Columns) == 0 {
		return m, nil
	}
	col := m.Browse.Columns[m.Browse.ColCursor]
	switch m.Browse.Sort {
	case col + " ASC":
		m.Browse.Sort = col + " DESC"
	case col + " DESC":
		m.Browse.Sort = ""
	default:
		m.Browse.Sort = col + " ASC"
	}
	m.Browse.Offset = 0
	m.Browse.RowCursor = 0
	return m.reloadRecords()
}

func (m Model) cycleMeta() (tea.Model, tea.Cmd) {
	if m.Browse.Table == "" {
		return m, nil
	}
	m.Browse.MetaKind = (m.Browse.MetaKind + 1) % len(metaNames)
	m.Browse.RowCursor = 0
	m.Browse.ColCursor = 0
	m.Browse.GridLoading = true
	if m.Browse.MetaKind == metaRecords {
		return m.reloadRecords()
	}
	return m, m.Cmds.LoadMeta(m.ActiveDriver, m.Browse.TableDB, m.Browse.Table, commands.MetaKind(m.Browse.MetaKind-1))
}

func (m Model) handleMetaLoadedMsg(msg types.MetaLoadedMsg) (tea.Model, tea.Cmd) {
	m.Browse.GridLoading = false
	if msg.Err != nil {
		m.Browse.GridErr = "Error loading " + metaNames[clampMeta(msg.Kind+1)] + ": " + msg.Err.Error()
		m.Browse.Columns = nil
		m.Browse.Rows = nil
		return m, nil
	}
	m.Browse.GridErr = ""
	if len(msg.Rows) > 0 {
		m.Browse.Columns = msg.Rows[0]
		m.Browse.Rows = msg.Rows[1:]
	} else {
		m.Browse.Columns = nil
		m.Browse.Rows = nil
	}
	m.Browse.Total = len(m.Browse.Rows)
	m.Browse.RowCursor = 0
	m.refreshJSONView()
	return m, nil
}

func clampMeta(i int) int {
	if i < 0 || i >= len(metaNames) {
		return metaRecords
	}
	return i
}

// ---- Filter (WHERE clause) ----

func (m Model) openFilter() (tea.Model, tea.Cmd) {
	if m.Browse.Table == "" || m.Browse.MetaKind != metaRecords {
		return m, nil
	}
	ti := textinput.New()
	ti.SetValue(m.Browse.Where)
	ti.Placeholder = "WHERE id > 100"
	ti.SetWidth(50)
	ti.Focus()
	ti.CursorEnd()
	m.FilterInput = ti
	m.Screen = types.ScreenFilter
	return m, nil
}

func (m Model) handleFilterScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.Screen = m.GridReturnScreen
		return m, nil
	case "enter":
		where := strings.TrimSpace(m.FilterInput.Value())
		if where != "" && !strings.HasPrefix(strings.ToUpper(where), "WHERE") {
			where = "WHERE " + where
		}
		m.Browse.Where = where
		m.Browse.Offset = 0
		m.Browse.RowCursor = 0
		m.Screen = m.GridReturnScreen
		return m.reloadRecords()
	}
	var cmd tea.Cmd
	m.FilterInput, cmd = m.FilterInput.Update(msg)
	return m, cmd
}

func (m Model) viewFilter() string {
	body := titleStyle.Render("Filter") + "\n\n" +
		keyStyle.Render("WHERE clause (blank clears):") + "\n" +
		m.FilterInput.View() + "\n\n" +
		helpStyle.Render("enter:apply  esc:cancel")
	return m.renderModal(body)
}
