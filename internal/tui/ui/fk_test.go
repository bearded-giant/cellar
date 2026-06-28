package ui

import (
	"testing"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

func TestParseForeignKeys_SQLite(t *testing.T) {
	// PRAGMA foreign_key_list: id, seq, table, from, to, on_update, on_delete, match
	fks := [][]string{
		{"id", "seq", "table", "from", "to", "on_update", "on_delete", "match"},
		{"0", "0", "users", "user_id", "id", "NO ACTION", "NO ACTION", "NONE"},
	}
	m := parseForeignKeys(fks)
	ref, ok := m["user_id"]
	if !ok {
		t.Fatalf("user_id FK not parsed: %v", m)
	}
	if ref.table != "users" || ref.col != "id" || ref.schema != "" {
		t.Errorf("ref = %+v, want {users,id}", ref)
	}
	if ref.qualified() != "users" {
		t.Errorf("qualified = %q, want users", ref.qualified())
	}
}

func TestParseForeignKeys_Postgres(t *testing.T) {
	fks := [][]string{
		{"constraint_name", "column_name", "foreign_table_schema", "foreign_table_name", "foreign_column_name"},
		{"orders_user_fk", "user_id", "public", "users", "id"},
	}
	m := parseForeignKeys(fks)
	ref := m["user_id"]
	if ref.qualified() != "public.users" || ref.col != "id" {
		t.Errorf("ref = %+v, want public.users.id", ref)
	}
}

func TestParseForeignKeys_UnknownShape(t *testing.T) {
	if m := parseForeignKeys([][]string{{"a", "b"}, {"1", "2"}}); m != nil {
		t.Errorf("unrecognized header should yield nil, got %v", m)
	}
}

func TestJumpFK_PushesCrumbAndFilters(t *testing.T) {
	m := gridModel() // table widgets, cols [id name], rows [[1 alpha][2 beta]]
	m.ActiveDriver = &drivers.SQLite{}
	m.Browse.FKMap = map[string]fkRef{"id": {table: "owners", col: "oid"}}
	m.Browse.RowCursor, m.Browse.ColCursor = 0, 0 // cell id=1, a FK

	res, cmd := m.jumpFK()
	m = res.(Model)
	if cmd == nil {
		t.Fatal("jumpFK should issue load commands")
	}
	if len(m.Browse.Crumbs) != 1 || m.Browse.Crumbs[0].table != "widgets" {
		t.Errorf("breadcrumb not pushed: %+v", m.Browse.Crumbs)
	}
	if m.Browse.Table != "owners" {
		t.Errorf("did not navigate to FK target, table = %q", m.Browse.Table)
	}
	if m.Browse.Where != "WHERE oid = '1'" {
		t.Errorf("filter = %q, want \"WHERE oid = '1'\"", m.Browse.Where)
	}

	// popping the crumb returns to widgets
	res2, _ := m.popCrumb()
	m2 := res2.(Model)
	if m2.Browse.Table != "widgets" || len(m2.Browse.Crumbs) != 0 {
		t.Errorf("popCrumb should return to widgets with empty stack; table=%q crumbs=%d", m2.Browse.Table, len(m2.Browse.Crumbs))
	}
}

func TestForeignKeysLoaded_StoresMap(t *testing.T) {
	m := gridModel()
	res, _ := m.handleForeignKeysLoadedMsg(types.ForeignKeysLoadedMsg{
		Table: "widgets",
		FKs: [][]string{
			{"id", "seq", "table", "from", "to"},
			{"0", "0", "owners", "owner_id", "oid"},
		},
	})
	if _, ok := res.(Model).Browse.FKMap["owner_id"]; !ok {
		t.Error("FK map should be stored for the current table")
	}
}
