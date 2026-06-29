package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

// fkRef is a foreign-key target: the referenced (schema.)table and column.
type fkRef struct {
	schema string
	table  string
	col    string
}

func (r fkRef) qualified() string {
	if r.schema != "" {
		return r.schema + "." + r.table
	}
	return r.table
}

// crumb snapshots a browse position so FK jumps can be unwound.
type crumb struct {
	db, table, label, where, sort string
	offset                        int
}

// parseForeignKeys maps local column -> FK target from a driver's GetForeignKeys
// output by matching known header names (sqlite PRAGMA + postgres). Returns nil
// for shapes it doesn't recognize, so FK jump just stays inert.
func parseForeignKeys(fks [][]string) map[string]fkRef {
	if len(fks) < 2 {
		return nil
	}
	header := fks[0]
	idx := func(names ...string) int {
		for i, h := range header {
			hl := strings.ToLower(h)
			for _, n := range names {
				if hl == n {
					return i
				}
			}
		}
		return -1
	}
	localI := idx("from", "column_name")
	tableI := idx("table", "foreign_table_name")
	colI := idx("to", "foreign_column_name")
	schemaI := idx("foreign_table_schema")
	if localI < 0 || tableI < 0 || colI < 0 {
		return nil
	}

	out := map[string]fkRef{}
	for _, row := range fks[1:] {
		if localI >= len(row) || tableI >= len(row) || colI >= len(row) {
			continue
		}
		ref := fkRef{table: row[tableI], col: row[colI]}
		if schemaI >= 0 && schemaI < len(row) {
			ref.schema = row[schemaI]
		}
		out[row[localI]] = ref
	}
	return out
}

func (m Model) handleForeignKeysLoadedMsg(msg types.ForeignKeysLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil || msg.Table != m.Browse.Table {
		return m, nil
	}
	m.Browse.FKMap = parseForeignKeys(msg.FKs)
	return m, nil
}

// jumpFK navigates from a foreign-key cell to the referenced row(s).
func (m Model) jumpFK() (tea.Model, tea.Cmd) {
	if m.Browse.MetaKind != metaRecords || m.Browse.Table == "" || m.ActiveDriver == nil {
		return m, nil
	}
	if m.Browse.RowCursor >= len(m.Browse.Rows) || m.Browse.ColCursor >= len(m.Browse.Columns) {
		return m, nil
	}
	ref, ok := m.Browse.FKMap[m.Browse.Columns[m.Browse.ColCursor]]
	if !ok {
		return m, nil
	}
	row := m.Browse.Rows[m.Browse.RowCursor]
	raw := ""
	if m.Browse.ColCursor < len(row) {
		raw = row[m.Browse.ColCursor]
	}
	if raw == "NULL&" || raw == "" {
		m.StatusMsg = "FK value is NULL"
		return m, nil
	}

	m.Browse.Crumbs = append(m.Browse.Crumbs, crumb{
		db: m.Browse.TableDB, table: m.Browse.Table, label: m.Browse.Label,
		where: m.Browse.Where, sort: m.Browse.Sort, offset: m.Browse.Offset,
	})
	where := "WHERE " + ref.col + " = " + m.ActiveDriver.FormatArgForQueryString(displayCell(raw))
	return m.navigateTable(m.Browse.TableDB, ref.qualified(), ref.table, where, "")
}

func (m Model) popCrumb() (tea.Model, tea.Cmd) {
	n := len(m.Browse.Crumbs)
	if n == 0 {
		return m, nil
	}
	c := m.Browse.Crumbs[n-1]
	m.Browse.Crumbs = m.Browse.Crumbs[:n-1]
	return m.navigateTable(c.db, c.table, c.label, c.where, c.sort)
}

// navigateTable points the browse grid at a (possibly different) table and
// loads its records + primary key + foreign keys. Does not touch the crumb
// stack (callers manage it).
func (m Model) navigateTable(db, table, label, where, sort string) (tea.Model, tea.Cmd) {
	m.Browse.TableDB = db
	m.Browse.Table = table
	m.Browse.Label = label
	m.Browse.Where = where
	m.Browse.Sort = sort
	m.Browse.Offset = 0
	m.Browse.RowCursor = 0
	m.Browse.ColCursor = 0
	m.Browse.MetaKind = metaRecords
	m.Browse.ViewJSON = false
	m.Browse.PkColumns = nil
	m.Browse.FKMap = nil
	m.clearStagedEdits()
	m.Browse.GridLoading = true
	m.Focus = types.FocusGrid
	return m, tea.Batch(
		m.Cmds.LoadRecords(m.ActiveDriver, db, table, where, sort, 0, m.Browse.Limit),
		m.Cmds.LoadPrimaryKey(m.ActiveDriver, db, table),
		m.Cmds.LoadForeignKeys(m.ActiveDriver, db, table),
	)
}
