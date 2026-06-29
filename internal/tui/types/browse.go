package types

import "github.com/bearded-giant/cellar/models"

type HistoryLoadedMsg struct {
	Items []models.QueryHistoryItem
	Err   error
}

type SavedQuerySavedMsg struct {
	Name string
	Err  error
}

type SavedQueriesLoadedMsg struct {
	Items []models.SavedQuery
	Err   error
}

// Focus selects which pane in the browse screen receives key input. Bubble Tea
// has no focus manager, so the browse screen hand-rolls one.
type Focus int

const (
	FocusTree Focus = iota
	FocusGrid
	FocusEditor
)

type DatabasesLoadedMsg struct {
	Databases []string
	Err       error
}

type TablesLoadedMsg struct {
	// DB is the database the tables belong to (the tree node that was expanded).
	DB string
	// Tables maps group -> table names: schema -> tables when the driver uses
	// schemas, otherwise a single database -> tables entry.
	Tables map[string][]string
	Err    error
}

type RecordsLoadedMsg struct {
	DB    string
	Table string
	// Rows[0] is the column header row; Rows[1:] are data rows.
	Rows   [][]string
	Total  int
	Offset int
	Err    error
}

type ExportDoneMsg struct {
	Path string
	Rows int
	Err  error
}

type PrimaryKeyLoadedMsg struct {
	Table   string
	Columns []string
	Err     error
}

type ChangesCommittedMsg struct {
	Count int
	Err   error
}

type MetaLoadedMsg struct {
	Kind int
	Rows [][]string
	Err  error
}

type ForeignKeysLoadedMsg struct {
	Table string
	FKs   [][]string
	Err   error
}

// ColumnsLoadedMsg carries a table's column names fetched on demand to feed the
// SQL editor autocompleter. Table is the key the completer registers under.
type ColumnsLoadedMsg struct {
	Table   string
	Columns []string
	Err     error
}

// QueryExecutedMsg carries the result of a SQL editor execution. SELECT-ish
// queries fill Rows (Rows[0] = header) + Total; DML fills Info.
type QueryExecutedMsg struct {
	Query    string
	IsSelect bool
	Rows     [][]string
	Total    int
	Info     string
	Err      error
}
