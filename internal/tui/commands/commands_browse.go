package commands

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/internal/history"
	"github.com/bearded-giant/cellar/internal/saved"
	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/models"
)

func recordHistory(connIdent, query string) {
	if connIdent != "" {
		_ = history.AddQueryToHistory(connIdent, query)
	}
}

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

// MetaKind selects which table-metadata view LoadMeta fetches.
type MetaKind int

const (
	MetaColumns MetaKind = iota
	MetaConstraints
	MetaIndexes
	MetaForeignKeys
)

// LoadMeta fetches a table-metadata view (columns/constraints/indexes/FKs).
// Each driver method returns [][]string with row 0 = header, matching the grid.
func (c *Commands) LoadMeta(driver drivers.Driver, db, table string, kind MetaKind) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.MetaLoadedMsg{Kind: int(kind)}
		}
		var (
			rows [][]string
			err  error
		)
		switch kind {
		case MetaColumns:
			rows, err = driver.GetTableColumns(db, table)
		case MetaConstraints:
			rows, err = driver.GetConstraints(db, table)
		case MetaIndexes:
			rows, err = driver.GetIndexes(db, table)
		case MetaForeignKeys:
			rows, err = driver.GetForeignKeys(db, table)
		}
		return types.MetaLoadedMsg{Kind: int(kind), Rows: rows, Err: err}
	}
}

// SaveQuery persists a named query for the connection (internal/saved, TOML).
func (c *Commands) SaveQuery(connIdent, name, query string) tea.Cmd {
	return func() tea.Msg {
		err := saved.SaveQuery(connIdent, name, query)
		return types.SavedQuerySavedMsg{Name: name, Err: err}
	}
}

// LoadSavedQueries reads the connection's saved queries.
func (c *Commands) LoadSavedQueries(connIdent string) tea.Cmd {
	return func() tea.Msg {
		items, err := saved.ReadSavedQueries(connIdent)
		return types.SavedQueriesLoadedMsg{Items: items, Err: err}
	}
}

// LoadForeignKeys fetches a table's foreign keys (for FK jump). Row 0 is the
// header; column names vary by driver and are parsed in the UI layer.
func (c *Commands) LoadForeignKeys(driver drivers.Driver, db, table string) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.ForeignKeysLoadedMsg{Table: table}
		}
		fks, err := driver.GetForeignKeys(db, table)
		return types.ForeignKeysLoadedMsg{Table: table, FKs: fks, Err: err}
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

// CommitChanges applies staged DML changes in one transaction, then records
// each committed statement to query history (best-effort).
func (c *Commands) CommitChanges(driver drivers.Driver, changes []models.DBDMLChange, connIdent string) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.ChangesCommittedMsg{}
		}
		if err := driver.ExecutePendingChanges(changes); err != nil {
			return types.ChangesCommittedMsg{Err: err}
		}
		for _, ch := range changes {
			if q, err := driver.DMLChangeToQueryString(ch); err == nil {
				recordHistory(connIdent, q)
			}
		}
		return types.ChangesCommittedMsg{Count: len(changes)}
	}
}

// LoadHistory reads the saved query history for a connection (newest first via
// the modal's own sort).
func (c *Commands) LoadHistory(connIdent string) tea.Cmd {
	return func() tea.Msg {
		path, err := history.GetHistoryFilePath(connIdent)
		if err != nil {
			return types.HistoryLoadedMsg{Err: err}
		}
		items, err := history.ReadHistory(path, 0)
		return types.HistoryLoadedMsg{Items: items, Err: err}
	}
}

// RunQuery executes a SQL editor query. SELECT-ish queries return rows via
// ExecuteQuery; everything else runs ExecuteDMLStatement (gated by readOnly).
// ponytail: history recording deferred — internal/history imports app (tview
// init); sever that coupling before calling history.AddQueryToHistory here.
func (c *Commands) RunQuery(driver drivers.Driver, query string, readOnly bool, connIdent string) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.QueryExecutedMsg{Query: query}
		}
		if isSelectQuery(query) {
			rows, total, err := driver.ExecuteQuery(query)
			recordHistory(connIdent, query)
			return types.QueryExecutedMsg{Query: query, IsSelect: true, Rows: rows, Total: total, Err: err}
		}
		if readOnly {
			if err := drivers.ValidateQueryForReadOnly(query); err != nil {
				return types.QueryExecutedMsg{Query: query, Err: err}
			}
		}
		info, err := driver.ExecuteDMLStatement(query)
		recordHistory(connIdent, query)
		return types.QueryExecutedMsg{Query: query, Info: info, Err: err}
	}
}
