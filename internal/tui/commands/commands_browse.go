package commands

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/internal/history"
	"github.com/bearded-giant/cellar/internal/saved"
	"github.com/bearded-giant/cellar/internal/state"
	"github.com/bearded-giant/cellar/internal/tui/types"
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
	q := strings.ToLower(stripLeadingComments(query))
	for _, p := range selectPrefixes {
		if strings.HasPrefix(q, p) {
			return true
		}
	}
	return false
}

// stripLeadingComments removes -- and /* */ comments (and whitespace) ahead of
// the first real token, so a commented statement still classifies by its verb.
func stripLeadingComments(q string) string {
	for {
		q = strings.TrimSpace(q)
		if strings.HasPrefix(q, "--") {
			i := strings.IndexByte(q, '\n')
			if i < 0 {
				return ""
			}
			q = q[i+1:]
			continue
		}
		if strings.HasPrefix(q, "/*") {
			i := strings.Index(q, "*/")
			if i < 0 {
				return ""
			}
			q = q[i+2:]
			continue
		}
		return q
	}
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

// LoadViews lists the views in a database, grouped like LoadTables. Postgres
// includes materialized views.
func (c *Commands) LoadViews(driver drivers.Driver, db string) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.ViewsLoadedMsg{DB: db}
		}
		views, err := driver.GetViews(db)
		return types.ViewsLoadedMsg{DB: db, Views: views, Err: err}
	}
}

// LoadViewDefinition fetches a view's SQL definition. name is schema-qualified
// ("schema.view") for schema drivers, bare otherwise.
func (c *Commands) LoadViewDefinition(driver drivers.Driver, db, name string) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.ViewDefinitionLoadedMsg{View: name}
		}
		def, err := driver.GetViewDefinition(db, name)
		return types.ViewDefinitionLoadedMsg{View: name, Definition: def, Err: err}
	}
}

// LoadTableDDL fetches a table's CREATE DDL. table is schema-qualified
// ("schema.table") for schema drivers, bare otherwise.
func (c *Commands) LoadTableDDL(driver drivers.Driver, db, table string) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.TableDDLLoadedMsg{Table: table}
		}
		ddl, err := driver.GetTableDDL(db, table)
		return types.TableDDLLoadedMsg{Table: table, DDL: ddl, Err: err}
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
			return types.MetaLoadedMsg{Kind: int(kind), Table: table}
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
		return types.MetaLoadedMsg{Kind: int(kind), Table: table, Rows: rows, Err: err}
	}
}

// SaveQuery persists a named query for the connection (internal/saved, TOML).
func (c *Commands) SaveQuery(connIdent, name, query string) tea.Cmd {
	return func() tea.Msg {
		err := saved.SaveQuery(connIdent, name, query)
		return types.SavedQuerySavedMsg{Name: name, Query: query, Err: err}
	}
}

// UpdateSavedQuery re-saves an already-named query in place.
func (c *Commands) UpdateSavedQuery(connIdent, name, query string) tea.Cmd {
	return func() tea.Msg {
		err := saved.UpdateSavedQuery(connIdent, name, query)
		return types.SavedQuerySavedMsg{Name: name, Query: query, Err: err}
	}
}

// RunQueries executes statements in order (notebook-style), stopping at the
// first error, and reports the last statement's result so the grid shows the
// final SELECT (or the last DML's status).
func (c *Commands) RunQueries(driver drivers.Driver, stmts []string, readOnly bool, connIdent string) tea.Cmd {
	return func() tea.Msg {
		if driver == nil || len(stmts) == 0 {
			return types.QueryExecutedMsg{}
		}
		ctx, done := StartQueryContext()
		defer done()
		var last types.QueryExecutedMsg
		ok := 0
		for i, q := range stmts {
			if readOnly { // both branches: WITH-wrapped DML routes as a "select"
				if err := drivers.ValidateQueryForReadOnly(q); err != nil {
					return types.QueryExecutedMsg{Query: q, Err: fmt.Errorf("statement %d of %d: %w", i+1, len(stmts), err)}
				}
			}
			if isSelectQuery(q) {
				limit := c.queryRowLimit()
				rows, total, err := driver.ExecuteQuery(ctx, q, limit)
				rows, total, truncated := capQueryRows(rows, total, limit)
				recordHistory(connIdent, q)
				last = types.QueryExecutedMsg{Query: q, IsSelect: true, Rows: rows, Total: total, Truncated: truncated, Err: err}
				if err != nil {
					last.Err = fmt.Errorf("statement %d of %d: %w", i+1, len(stmts), err)
					return last
				}
			} else {
				info, err := driver.ExecuteDMLStatement(ctx, q)
				recordHistory(connIdent, q)
				last = types.QueryExecutedMsg{Query: q, Info: info, Err: err}
				if err != nil {
					last.Err = fmt.Errorf("statement %d of %d: %w", i+1, len(stmts), err)
					return last
				}
			}
			ok++
		}
		if !last.IsSelect {
			last.Info = fmt.Sprintf("ran %d statements — %s", ok, last.Info)
		}
		return last
	}
}

// LoadQueryState reads the connection's persisted query buffers (restore on
// connect).
func (c *Commands) LoadQueryState(connIdent string) tea.Cmd {
	return func() tea.Msg {
		st, err := state.Load(connIdent)
		return types.QueryStateLoadedMsg{State: st, Err: err}
	}
}

// SaveQueryState persists the connection's query buffers (autosave on run;
// disconnect/quit backstops).
func (c *Commands) SaveQueryState(connIdent string, st state.State) tea.Cmd {
	return func() tea.Msg {
		if connIdent == "" {
			return types.QueryStateSavedMsg{}
		}
		return types.QueryStateSavedMsg{Err: state.Save(connIdent, st)}
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
		ctx, done := StartQueryContext()
		defer done()
		rows, total, _, err := driver.GetRecords(ctx, db, table, where, sort, offset, limit)
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

// LoadColumns fetches a table's column names for the editor autocompleter. db
// is the database; table is driver-qualified (schema.table for schema drivers,
// bare otherwise); register is the key the completer stores them under. Errors
// are non-fatal (autocomplete is best-effort).
func (c *Commands) LoadColumns(driver drivers.Driver, db, table, register string) tea.Cmd {
	return func() tea.Msg {
		if driver == nil {
			return types.ColumnsLoadedMsg{Table: register}
		}
		rows, err := driver.GetTableColumns(db, table)
		if err != nil {
			return types.ColumnsLoadedMsg{Table: register, Err: err}
		}
		var cols []string
		for i, r := range rows {
			if i == 0 || len(r) == 0 { // rows[0] is the column-metadata header
				continue
			}
			cols = append(cols, r[0])
		}
		return types.ColumnsLoadedMsg{Table: register, Columns: cols}
	}
}

// DeleteHistory removes one history entry (matched by text + timestamp) and
// returns the remaining items via HistoryLoadedMsg so the modal refreshes.
func (c *Commands) DeleteHistory(connIdent, queryText string, ts time.Time) tea.Cmd {
	return func() tea.Msg {
		items, err := history.DeleteQueryFromHistory(connIdent, queryText, ts)
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
		ctx, done := StartQueryContext()
		defer done()
		// read-only validation runs on BOTH branches: WITH-wrapped DML and
		// EXPLAIN ANALYZE <dml> route through the SELECT branch but still write
		if readOnly {
			if err := drivers.ValidateQueryForReadOnly(query); err != nil {
				return types.QueryExecutedMsg{Query: query, Err: err}
			}
		}
		if isSelectQuery(query) {
			limit := c.queryRowLimit()
			rows, total, err := driver.ExecuteQuery(ctx, query, limit)
			rows, total, truncated := capQueryRows(rows, total, limit)
			recordHistory(connIdent, query)
			return types.QueryExecutedMsg{Query: query, IsSelect: true, Rows: rows, Total: total, Truncated: truncated, Err: err}
		}
		info, err := driver.ExecuteDMLStatement(ctx, query)
		recordHistory(connIdent, query)
		return types.QueryExecutedMsg{Query: query, Info: info, Err: err}
	}
}
