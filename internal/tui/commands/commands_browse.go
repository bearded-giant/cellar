package commands

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

// LoadDatabases lists the databases on the live connection.
func (c *Commands) LoadDatabases(driver drivers.Driver) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.DatabasesLoadedMsg{}
		}
		dbs, err := driver.GetDatabases()
		return types.DatabasesLoadedMsg{Databases: dbs, Err: err}
	}
}

// LoadTables lists the tables in a database, grouped by schema when the driver
// uses schemas (Postgres) or under the database name when it does not.
func (c *Commands) LoadTables(driver drivers.Driver, db string) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.TablesLoadedMsg{DB: db}
		}
		tables, err := driver.GetTables(db)
		return types.TablesLoadedMsg{DB: db, Tables: tables, Err: err}
	}
}

// LoadRecords fetches one page of rows. table must be schema-qualified
// ("schema.table") for schema drivers, bare otherwise — the tree builds it.
// Rows[0] is the header row; Total is the unpaginated row count.
func (c *Commands) LoadRecords(driver drivers.Driver, db, table, where, sort string, offset, limit int) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.RecordsLoadedMsg{DB: db, Table: table, Offset: offset}
		}
		rows, total, _, err := driver.GetRecords(db, table, where, sort, offset, limit)
		return types.RecordsLoadedMsg{DB: db, Table: table, Rows: rows, Total: total, Offset: offset, Err: err}
	}
}
