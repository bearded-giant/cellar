package commands

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/internal/history"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

func TestLoadDatabases(t *testing.T) {
	stub := &stubDriver{databases: []string{"app", "metrics"}}
	c := &Commands{}
	msg := c.LoadDatabases(stub)().(types.DatabasesLoadedMsg)
	if msg.Err != nil {
		t.Fatalf("unexpected error: %v", msg.Err)
	}
	if !reflect.DeepEqual(msg.Databases, []string{"app", "metrics"}) {
		t.Errorf("Databases = %v", msg.Databases)
	}
}

func TestLoadTables(t *testing.T) {
	stub := &stubDriver{tables: map[string][]string{"public": {"users", "orders"}}}
	c := &Commands{}
	msg := c.LoadTables(stub, "app")().(types.TablesLoadedMsg)
	if msg.DB != "app" {
		t.Errorf("DB = %q, want app", msg.DB)
	}
	if !reflect.DeepEqual(msg.Tables, map[string][]string{"public": {"users", "orders"}}) {
		t.Errorf("Tables = %v", msg.Tables)
	}
}

func TestLoadViews(t *testing.T) {
	stub := &stubDriver{views: map[string][]string{"public": {"v_users"}}}
	c := &Commands{}
	msg := c.LoadViews(stub, "app")().(types.ViewsLoadedMsg)
	if msg.DB != "app" {
		t.Errorf("DB = %q, want app", msg.DB)
	}
	if !reflect.DeepEqual(msg.Views, map[string][]string{"public": {"v_users"}}) {
		t.Errorf("Views = %v", msg.Views)
	}
	if nilMsg := c.LoadViews(nil, "x")().(types.ViewsLoadedMsg); nilMsg.DB != "x" || nilMsg.Views != nil {
		t.Errorf("nil driver should echo DB with no views: %+v", nilMsg)
	}
}

func TestLoadViewDefinition(t *testing.T) {
	stub := &stubDriver{viewDef: "CREATE VIEW v AS SELECT 1"}
	c := &Commands{}
	msg := c.LoadViewDefinition(stub, "app", "public.v")().(types.ViewDefinitionLoadedMsg)
	if msg.View != "public.v" {
		t.Errorf("View = %q", msg.View)
	}
	if msg.Definition != "CREATE VIEW v AS SELECT 1" {
		t.Errorf("Definition = %q", msg.Definition)
	}
	if nilMsg := c.LoadViewDefinition(nil, "app", "v")().(types.ViewDefinitionLoadedMsg); nilMsg.View != "v" {
		t.Errorf("nil driver should echo view name: %+v", nilMsg)
	}
}

func TestLoadTableDDL(t *testing.T) {
	stub := &stubDriver{ddl: "CREATE TABLE t (id INTEGER);"}
	c := &Commands{}
	msg := c.LoadTableDDL(stub, "app", "public.t")().(types.TableDDLLoadedMsg)
	if msg.Table != "public.t" {
		t.Errorf("Table = %q", msg.Table)
	}
	if msg.DDL != "CREATE TABLE t (id INTEGER);" {
		t.Errorf("DDL = %q", msg.DDL)
	}
	if nilMsg := c.LoadTableDDL(nil, "app", "t")().(types.TableDDLLoadedMsg); nilMsg.Table != "t" {
		t.Errorf("nil driver should echo table name: %+v", nilMsg)
	}
}

func TestLoadRecords_PassesArgsAndReturnsRows(t *testing.T) {
	stub := &stubDriver{
		records:      [][]string{{"id", "name"}, {"1", "alpha"}},
		recordsTotal: 7,
	}
	c := &Commands{}
	msg := c.LoadRecords(stub, "app", "public.users", "WHERE id > 0", "id DESC", 100, 50)().(types.RecordsLoadedMsg)
	if msg.Err != nil {
		t.Fatalf("unexpected error: %v", msg.Err)
	}
	if msg.Table != "public.users" || msg.DB != "app" {
		t.Errorf("DB/Table = %q/%q", msg.DB, msg.Table)
	}
	if msg.Total != 7 {
		t.Errorf("Total = %d, want 7", msg.Total)
	}
	if msg.Offset != 100 {
		t.Errorf("Offset = %d, want 100 (echoed for stale-drop)", msg.Offset)
	}
	want := []string{"app", "public.users", "WHERE id > 0", "id DESC"}
	if !reflect.DeepEqual(stub.lastGetArgs, want) {
		t.Errorf("driver GetRecords args = %v, want %v", stub.lastGetArgs, want)
	}
}

func TestBrowseCommands_NilDriverNoPanic(t *testing.T) {
	c := &Commands{}
	if msg := c.LoadDatabases(nil)().(types.DatabasesLoadedMsg); msg.Err != nil {
		t.Errorf("nil driver should yield empty msg, got err %v", msg.Err)
	}
	if msg := c.LoadTables(nil, "x")().(types.TablesLoadedMsg); msg.DB != "x" {
		t.Errorf("nil driver LoadTables should still echo DB")
	}
	_ = c.LoadRecords(nil, "x", "t", "", "", 0, 10)().(types.RecordsLoadedMsg)
}

// TestBrowse_SQLiteEndToEnd drives the real SQLite driver through the browse
// command factories — no server, no tunnel — proving the tree->records path
// against an actual database.
func TestBrowse_SQLiteEndToEnd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browse.db")

	seed, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	if _, err := seed.Exec(`CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := seed.Exec(`INSERT INTO widgets (name) VALUES ('alpha'), ('beta')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	_ = seed.Close()

	driver := &drivers.SQLite{}
	if err := driver.Connect(path); err != nil {
		t.Fatalf("driver connect: %v", err)
	}
	c := &Commands{}

	dbs := c.LoadDatabases(driver)().(types.DatabasesLoadedMsg)
	if dbs.Err != nil || len(dbs.Databases) == 0 {
		t.Fatalf("LoadDatabases: %v / %v", dbs.Err, dbs.Databases)
	}
	db := dbs.Databases[0]

	tbl := c.LoadTables(driver, db)().(types.TablesLoadedMsg)
	if tbl.Err != nil {
		t.Fatalf("LoadTables: %v", tbl.Err)
	}
	var found bool
	for _, names := range tbl.Tables {
		for _, n := range names {
			if n == "widgets" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("widgets table not listed: %v", tbl.Tables)
	}

	rec := c.LoadRecords(driver, db, "widgets", "", "", 0, 100)().(types.RecordsLoadedMsg)
	if rec.Err != nil {
		t.Fatalf("LoadRecords: %v", rec.Err)
	}
	if rec.Total != 2 {
		t.Errorf("Total = %d, want 2", rec.Total)
	}
	if len(rec.Rows) != 3 { // header + 2 data rows
		t.Fatalf("Rows = %d, want 3 (header + 2)", len(rec.Rows))
	}
	if !reflect.DeepEqual(rec.Rows[0], []string{"id", "name"}) {
		t.Errorf("header = %v, want [id name]", rec.Rows[0])
	}
}

func TestIsSelectQuery(t *testing.T) {
	sel := []string{"SELECT 1", "select * from t", "  WITH x AS (...)", "explain analyze", "SHOW TABLES", "describe t", "desc t",
		"-- note\nselect * from t",
		"-- 6b8a355d-1812-4d27-b228-700a18f40412\n\nselect * from \"StorefrontUser\" limit 10;",
		"/* block */ select 1",
		"/* multi\nline */\n-- and a line comment\nselect 1"}
	for _, q := range sel {
		if !isSelectQuery(q) {
			t.Errorf("isSelectQuery(%q) = false, want true", q)
		}
	}
	dml := []string{"UPDATE t SET x=1", "delete from t", "insert into t values (1)", "CREATE TABLE t (...)", "",
		"-- comment only",
		"/* unterminated select",
		"-- note\nupdate t set x=1"}
	for _, q := range dml {
		if isSelectQuery(q) {
			t.Errorf("isSelectQuery(%q) = true, want false", q)
		}
	}
}

func TestRunQuery_SelectUsesExecuteQuery(t *testing.T) {
	stub := &stubDriver{queryRows: [][]string{{"id"}, {"1"}}, queryTotal: 1}
	c := &Commands{}
	msg := c.RunQuery(stub, "SELECT * FROM widgets", false, "")().(types.QueryExecutedMsg)
	if !msg.IsSelect {
		t.Error("SELECT should set IsSelect")
	}
	if stub.ranQuery != "SELECT * FROM widgets" || stub.ranDML != "" {
		t.Errorf("should call ExecuteQuery only; ranQuery=%q ranDML=%q", stub.ranQuery, stub.ranDML)
	}
	if msg.Total != 1 || len(msg.Rows) != 2 {
		t.Errorf("rows/total = %v / %d", msg.Rows, msg.Total)
	}
}

func TestRunQuery_DMLUsesExecuteDML(t *testing.T) {
	stub := &stubDriver{dmlInfo: "1 row affected"}
	c := &Commands{}
	msg := c.RunQuery(stub, "UPDATE widgets SET name='x'", false, "")().(types.QueryExecutedMsg)
	if msg.IsSelect {
		t.Error("UPDATE should not be IsSelect")
	}
	if stub.ranDML != "UPDATE widgets SET name='x'" || stub.ranQuery != "" {
		t.Errorf("should call ExecuteDMLStatement only; ranDML=%q ranQuery=%q", stub.ranDML, stub.ranQuery)
	}
	if msg.Info != "1 row affected" {
		t.Errorf("Info = %q", msg.Info)
	}
}

func TestRunQuery_ReadOnlyBlocksDML(t *testing.T) {
	stub := &stubDriver{}
	c := &Commands{}
	msg := c.RunQuery(stub, "DELETE FROM widgets", true, "")().(types.QueryExecutedMsg)
	if msg.Err == nil {
		t.Error("read-only mode must reject a DML query")
	}
	if stub.ranDML != "" {
		t.Error("read-only DML must not reach the driver")
	}
}

func TestRunQuery_RecordsHistory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c := &Commands{}
	_ = c.RunQuery(&stubDriver{queryRows: [][]string{{"x"}}}, "SELECT 1", false, "testconn")()

	path, err := history.GetHistoryFilePath("testconn")
	if err != nil {
		t.Fatalf("history path: %v", err)
	}
	items, err := history.ReadHistory(path, 0)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	if len(items) != 1 || items[0].QueryText != "SELECT 1" {
		t.Errorf("recorded history = %+v, want one 'SELECT 1'", items)
	}
}

func TestRunQuery_ReadOnlyAllowsSelect(t *testing.T) {
	stub := &stubDriver{queryRows: [][]string{{"id"}}, queryTotal: 0}
	c := &Commands{}
	msg := c.RunQuery(stub, "SELECT 1", true, "")().(types.QueryExecutedMsg)
	if msg.Err != nil {
		t.Errorf("read-only must allow SELECT, got %v", msg.Err)
	}
	if stub.ranQuery != "SELECT 1" {
		t.Error("SELECT should reach ExecuteQuery even in read-only")
	}
}
