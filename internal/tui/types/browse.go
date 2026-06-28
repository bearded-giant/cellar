package types

// Focus selects which pane in the browse screen receives key input. Bubble Tea
// has no focus manager, so the browse screen hand-rolls one.
type Focus int

const (
	FocusTree Focus = iota
	FocusGrid
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
