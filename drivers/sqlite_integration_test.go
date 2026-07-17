package drivers

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xo/dburl"
)

// connectRealSQLite opens a real modernc-backed database and registers cleanup.
func connectRealSQLite(t *testing.T, urlstr string) *SQLite {
	t.Helper()
	db := &SQLite{}
	if err := db.Connect(urlstr); err != nil {
		t.Fatalf("Connect(%q) failed: %v", urlstr, err)
	}
	t.Cleanup(func() { _ = db.Connection.Close() })
	return db
}

// seedRealSQLite creates users (PK) + orders (FK to users, indexed), a view
// over users, and rows. users has 10 rows; user10 has a NULL email.
func seedRealSQLite(t *testing.T, db *SQLite) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT)`,
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, amount REAL, FOREIGN KEY (user_id) REFERENCES users (id))`,
		`CREATE INDEX idx_orders_user_id ON orders (user_id)`,
		`CREATE VIEW user_emails AS SELECT id, email FROM users WHERE email IS NOT NULL`,
	}
	for _, stmt := range stmts {
		if _, err := db.Connection.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	for i := 1; i <= 10; i++ {
		email := any(fmt.Sprintf("user%d@example.com", i))
		if i == 10 {
			email = nil
		}
		if _, err := db.Connection.Exec(`INSERT INTO users (id, name, email) VALUES (?, ?, ?)`, i, fmt.Sprintf("user%d", i), email); err != nil {
			t.Fatalf("seed users: %v", err)
		}
	}
	if _, err := db.Connection.Exec(`INSERT INTO orders (id, user_id, amount) VALUES (1, 1, 9.99), (2, 2, 19.99)`); err != nil {
		t.Fatalf("seed orders: %v", err)
	}
}

func seededRealSQLite(t *testing.T) *SQLite {
	t.Helper()
	db := connectRealSQLite(t, filepath.Join(t.TempDir(), "cellar_test.db"))
	seedRealSQLite(t, db)
	return db
}

func TestSQLite_Connect_RealFile_URLForms(t *testing.T) {
	testCases := []struct {
		name   string
		urlstr func(path string) string
	}{
		{name: "sqlite scheme double slash", urlstr: func(p string) string { return "sqlite://" + p }},
		{name: "sqlite scheme single colon", urlstr: func(p string) string { return "sqlite:" + p }},
		{name: "sqlite3 scheme", urlstr: func(p string) string { return "sqlite3://" + p }},
		{name: "bare absolute path", urlstr: func(p string) string { return p }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "conn_test.db")
			db := connectRealSQLite(t, tc.urlstr(path))

			if db.GetProvider() != DriverSqlite {
				t.Errorf("provider = %q, want %q", db.GetProvider(), DriverSqlite)
			}

			if _, err := db.Connection.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
				t.Fatalf("create table: %v", err)
			}
			if _, err := db.Connection.Exec(`INSERT INTO t (id, v) VALUES (1, 'x')`); err != nil {
				t.Fatalf("insert: %v", err)
			}
			var v string
			if err := db.Connection.QueryRow(`SELECT v FROM t WHERE id = 1`).Scan(&v); err != nil {
				t.Fatalf("select: %v", err)
			}
			if v != "x" {
				t.Errorf("round trip got %q, want %q", v, "x")
			}
		})
	}
}

func TestSQLite_Connect_RealFile_URLAndBarePathHitSameFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.db")

	viaURL := connectRealSQLite(t, "sqlite://"+path)
	if _, err := viaURL.Connection.Exec(`CREATE TABLE marker (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	viaPath := connectRealSQLite(t, path)
	tables, err := viaPath.GetTables("shared.db")
	if err != nil {
		t.Fatalf("GetTables: %v", err)
	}
	found := false
	for _, tbl := range tables["shared.db"] {
		if tbl == "marker" {
			found = true
		}
	}
	if !found {
		t.Errorf("bare-path connection did not see table created via sqlite:// URL; got %v", tables)
	}
}

// provider inference at connection-add time relies on dburl mapping the sqlite
// scheme to the same name as DriverSqlite.
func TestSQLite_ProviderInference_FromURLScheme(t *testing.T) {
	for _, urlstr := range []string{
		"sqlite:///abs/path/app.db",
		"sqlite:/abs/path/app.db",
		"sqlite3:///abs/path/app.db",
	} {
		parsed, err := dburl.Parse(urlstr)
		if err != nil {
			t.Fatalf("dburl.Parse(%q): %v", urlstr, err)
		}
		if parsed.Driver != DriverSqlite {
			t.Errorf("dburl.Parse(%q).Driver = %q, want %q", urlstr, parsed.Driver, DriverSqlite)
		}
	}
}

func TestSQLite_RealFile_GetDatabases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mydata.db")
	db := connectRealSQLite(t, "sqlite://"+path)
	seedRealSQLite(t, db)

	databases, err := db.GetDatabases()
	if err != nil {
		t.Fatalf("GetDatabases: %v", err)
	}
	if !reflect.DeepEqual(databases, []string{"mydata.db"}) {
		t.Errorf("GetDatabases = %v, want [mydata.db]", databases)
	}
}

func TestSQLite_RealFile_GetTables(t *testing.T) {
	db := seededRealSQLite(t)

	tables, err := db.GetTables("cellar_test.db")
	if err != nil {
		t.Fatalf("GetTables: %v", err)
	}
	got := tables["cellar_test.db"]
	want := map[string]bool{"users": true, "orders": true}
	for _, tbl := range got {
		delete(want, tbl)
		if tbl == "user_emails" {
			t.Errorf("GetTables must exclude views; got %v", got)
		}
	}
	if len(want) != 0 {
		t.Errorf("GetTables missing %v; got %v", want, got)
	}
}

func TestSQLite_RealFile_GetViews(t *testing.T) {
	db := seededRealSQLite(t)

	views, err := db.GetViews("cellar_test.db")
	if err != nil {
		t.Fatalf("GetViews: %v", err)
	}
	if !reflect.DeepEqual(views, map[string][]string{"cellar_test.db": {"user_emails"}}) {
		t.Errorf("GetViews = %v, want map[cellar_test.db:[user_emails]]", views)
	}
}

func TestSQLite_RealFile_GetViewDefinition(t *testing.T) {
	db := seededRealSQLite(t)

	def, err := db.GetViewDefinition("", "user_emails")
	if err != nil {
		t.Fatalf("GetViewDefinition: %v", err)
	}
	want := "CREATE VIEW user_emails AS SELECT id, email FROM users WHERE email IS NOT NULL"
	if def != want {
		t.Errorf("definition = %q, want %q", def, want)
	}

	if _, err := db.GetViewDefinition("", "missing_view"); err == nil {
		t.Error("expected error for missing view")
	}
}

func TestSQLite_RealFile_GetTableDDL(t *testing.T) {
	db := seededRealSQLite(t)

	ddl, err := db.GetTableDDL("", "orders")
	if err != nil {
		t.Fatalf("GetTableDDL: %v", err)
	}
	want := "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, amount REAL, FOREIGN KEY (user_id) REFERENCES users (id));\n\nCREATE INDEX idx_orders_user_id ON orders (user_id);"
	if ddl != want {
		t.Errorf("DDL = %q, want %q", ddl, want)
	}

	if _, err := db.GetTableDDL("", "missing_table"); err == nil {
		t.Error("expected error for missing table")
	}
}

func TestSQLite_RealFile_GetTableDDL_NoIndexes(t *testing.T) {
	db := seededRealSQLite(t)

	ddl, err := db.GetTableDDL("", "users")
	if err != nil {
		t.Fatalf("GetTableDDL: %v", err)
	}
	want := "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT);"
	if ddl != want {
		t.Errorf("DDL = %q, want %q", ddl, want)
	}
}

func TestSQLite_RealFile_GetTableColumns(t *testing.T) {
	db := seededRealSQLite(t)

	cols, err := db.GetTableColumns("", "users")
	if err != nil {
		t.Fatalf("GetTableColumns: %v", err)
	}

	wantHeader := []string{"name", "type", "notnull", "dflt_value", "pk"}
	if !reflect.DeepEqual(cols[0], wantHeader) {
		t.Fatalf("header = %v, want %v", cols[0], wantHeader)
	}
	if len(cols) != 4 {
		t.Fatalf("expected header + 3 columns, got %d rows: %v", len(cols), cols)
	}
	wantRows := [][]string{
		{"id", "INTEGER", "0", "", "1"},
		{"name", "TEXT", "1", "", "0"},
		{"email", "TEXT", "0", "", "0"},
	}
	if !reflect.DeepEqual(cols[1:], wantRows) {
		t.Errorf("columns = %v, want %v", cols[1:], wantRows)
	}
}

func TestSQLite_RealFile_GetIndexes(t *testing.T) {
	db := seededRealSQLite(t)

	indexes, err := db.GetIndexes("", "orders")
	if err != nil {
		t.Fatalf("GetIndexes: %v", err)
	}
	if len(indexes) < 2 {
		t.Fatalf("expected header + at least 1 index, got %v", indexes)
	}
	nameIdx := -1
	for i, col := range indexes[0] {
		if col == "name" {
			nameIdx = i
		}
	}
	if nameIdx == -1 {
		t.Fatalf("no name column in index header %v", indexes[0])
	}
	found := false
	for _, row := range indexes[1:] {
		if row[nameIdx] == "idx_orders_user_id" {
			found = true
		}
	}
	if !found {
		t.Errorf("idx_orders_user_id not in %v", indexes)
	}
}

func TestSQLite_RealFile_GetForeignKeys(t *testing.T) {
	db := seededRealSQLite(t)

	fks, err := db.GetForeignKeys("", "orders")
	if err != nil {
		t.Fatalf("GetForeignKeys: %v", err)
	}
	if len(fks) != 2 {
		t.Fatalf("expected header + 1 FK, got %v", fks)
	}
	header, row := fks[0], fks[1]
	get := func(col string) string {
		for i, c := range header {
			if c == col {
				return row[i]
			}
		}
		t.Fatalf("column %q missing from FK header %v", col, header)
		return ""
	}
	if get("table") != "users" || get("from") != "user_id" || get("to") != "id" {
		t.Errorf("FK = table %q from %q to %q, want users/user_id/id", get("table"), get("from"), get("to"))
	}
}

func TestSQLite_RealFile_GetPrimaryKeyColumnNames(t *testing.T) {
	db := seededRealSQLite(t)

	keys, err := db.GetPrimaryKeyColumnNames("", "users")
	if err != nil {
		t.Fatalf("GetPrimaryKeyColumnNames: %v", err)
	}
	if !reflect.DeepEqual(keys, []string{"id"}) {
		t.Errorf("primary keys = %v, want [id]", keys)
	}
}

func TestSQLite_RealFile_ExecuteQuery(t *testing.T) {
	db := seededRealSQLite(t)

	results, count, err := db.ExecuteQuery(`SELECT id, name FROM users WHERE id <= 2 ORDER BY id`)
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}
	expected := [][]string{
		{"id", "name"},
		{"1", "user1"},
		{"2", "user2"},
	}
	if !reflect.DeepEqual(results, expected) {
		t.Errorf("results = %v, want %v", results, expected)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestSQLite_RealFile_ExecuteDMLStatement(t *testing.T) {
	db := seededRealSQLite(t)

	result, err := db.ExecuteDMLStatement(`UPDATE users SET name = 'renamed' WHERE id = 1`)
	if err != nil {
		t.Fatalf("ExecuteDMLStatement: %v", err)
	}
	if result != "1 rows affected" {
		t.Errorf("result = %q, want %q", result, "1 rows affected")
	}

	var name string
	if err := db.Connection.QueryRow(`SELECT name FROM users WHERE id = 1`).Scan(&name); err != nil {
		t.Fatalf("verify select: %v", err)
	}
	if name != "renamed" {
		t.Errorf("name after DML = %q, want %q", name, "renamed")
	}
}

func TestSQLite_RealFile_GetRecords_Pagination(t *testing.T) {
	db := seededRealSQLite(t)

	firstPage, total, queryString, err := db.GetRecords("", "users", "", "id", 0, 4)
	if err != nil {
		t.Fatalf("GetRecords page 1: %v", err)
	}
	if total != 10 {
		t.Errorf("total = %d, want 10", total)
	}
	if len(firstPage) != 5 {
		t.Fatalf("expected header + 4 rows, got %d: %v", len(firstPage), firstPage)
	}
	if !reflect.DeepEqual(firstPage[0], []string{"id", "name", "email"}) {
		t.Errorf("header = %v", firstPage[0])
	}
	for i, wantID := range []string{"1", "2", "3", "4"} {
		if firstPage[i+1][0] != wantID {
			t.Errorf("page 1 row %d id = %q, want %q", i, firstPage[i+1][0], wantID)
		}
	}
	if !strings.Contains(queryString, "LIMIT 0, 4") {
		t.Errorf("queryString %q missing interpolated LIMIT 0, 4", queryString)
	}

	secondPage, total, _, err := db.GetRecords("", "users", "", "id", 4, 4)
	if err != nil {
		t.Fatalf("GetRecords page 2: %v", err)
	}
	if total != 10 {
		t.Errorf("page 2 total = %d, want 10", total)
	}
	for i, wantID := range []string{"5", "6", "7", "8"} {
		if secondPage[i+1][0] != wantID {
			t.Errorf("page 2 row %d id = %q, want %q", i, secondPage[i+1][0], wantID)
		}
	}
}

func TestSQLite_RealFile_GetRecords_WhereAndNullMarker(t *testing.T) {
	db := seededRealSQLite(t)

	records, total, _, err := db.GetRecords("", "users", "WHERE id > 8", "id", 0, DefaultRowLimit)
	if err != nil {
		t.Fatalf("GetRecords with where: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	expected := [][]string{
		{"id", "name", "email"},
		{"9", "user9", "user9@example.com"},
		{"10", "user10", "NULL&"},
	}
	if !reflect.DeepEqual(records, expected) {
		t.Errorf("records = %v, want %v", records, expected)
	}
}
