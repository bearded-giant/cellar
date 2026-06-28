package commands

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/internal/tui/types"
	"github.com/jorgerojas26/lazysql/models"
)

// selectPrefixes route a query to ExecuteQuery (rows) rather than
// ExecuteDMLStatement (info string). Mirrors components/results_table.go.
var selectPrefixes = []string{"select", "with", "explain", "show", "describe", "desc"}

func isSelectQuery(query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	for _, p := range selectPrefixes {
		if strings.HasPrefix(q, p) {
			return true
		}
	}
	return false
}

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

// LoadPrimaryKey fetches a table's primary key column names (used to target
// UPDATE/DELETE rows; empty -> whole-row fallback).
func (c *Commands) LoadPrimaryKey(driver drivers.Driver, db, table string) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.PrimaryKeyLoadedMsg{Table: table}
		}
		cols, err := driver.GetPrimaryKeyColumnNames(db, table)
		return types.PrimaryKeyLoadedMsg{Table: table, Columns: cols, Err: err}
	}
}

// CommitChanges applies staged DML changes in one transaction.
func (c *Commands) CommitChanges(driver drivers.Driver, changes []models.DBDMLChange) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.ChangesCommittedMsg{}
		}
		if err := driver.ExecutePendingChanges(changes); err != nil {
			return types.ChangesCommittedMsg{Err: err}
		}
		return types.ChangesCommittedMsg{Count: len(changes)}
	}
}

// RunQuery executes a SQL editor query. SELECT-ish queries return rows via
// ExecuteQuery; everything else runs ExecuteDMLStatement (gated by readOnly).
// ponytail: history recording deferred — internal/history imports app (tview
// init); sever that coupling before calling history.AddQueryToHistory here.
func (c *Commands) RunQuery(driver drivers.Driver, query string, readOnly bool) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.QueryExecutedMsg{Query: query}
		}
		if isSelectQuery(query) {
			rows, total, err := driver.ExecuteQuery(query)
			return types.QueryExecutedMsg{Query: query, IsSelect: true, Rows: rows, Total: total, Err: err}
		}
		if readOnly {
			if err := drivers.ValidateQueryForReadOnly(query); err != nil {
				return types.QueryExecutedMsg{Query: query, Err: err}
			}
		}
		info, err := driver.ExecuteDMLStatement(query)
		return types.QueryExecutedMsg{Query: query, Info: info, Err: err}
	}
}
