package ui

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/internal/tui/types"
	"github.com/jorgerojas26/lazysql/models"
)

func gridModel() Model {
	m := browseModel()
	m.Browse.Table = "widgets"
	m.Browse.TableDB = "app"
	m.Browse.Label = "widgets"
	m.Browse.Columns = []string{"id", "name"}
	m.Browse.Rows = [][]string{{"1", "alpha"}, {"2", "beta"}}
	m.Browse.PkColumns = []string{"id"}
	m.Focus = types.FocusGrid
	return m
}

func TestApplyCellEdit_AndCancelOnOriginal(t *testing.T) {
	m := gridModel()
	m.Browse.RowCursor, m.Browse.EditCol = 0, 1

	m.applyCellEdit("ALPHA")
	if m.Browse.Edited[[2]int{0, 1}] != "ALPHA" {
		t.Fatalf("edit not staged: %v", m.Browse.Edited)
	}
	// editing back to the original value cancels the staged change
	m.applyCellEdit("alpha")
	if _, ok := m.Browse.Edited[[2]int{0, 1}]; ok {
		t.Error("edit back to original should remove the staged change")
	}
}

func TestToggleDeleteRow(t *testing.T) {
	m := gridModel()
	m.Browse.RowCursor = 1
	res, _ := m.toggleDeleteRow()
	m = res.(Model)
	if !m.Browse.Deleted[1] {
		t.Fatal("row 1 should be staged for delete")
	}
	res, _ = m.toggleDeleteRow() // toggle off
	if res.(Model).Browse.Deleted[1] {
		t.Error("second delete should toggle the row back")
	}
}

func TestAppendInsertRow_DefaultsThenEdit(t *testing.T) {
	m := gridModel()
	res, _ := m.appendInsertRow()
	m = res.(Model)
	if len(m.Browse.Inserts) != 1 {
		t.Fatalf("expected 1 insert row, got %d", len(m.Browse.Inserts))
	}
	if m.Browse.RowCursor != 2 { // appended after the 2 existing rows
		t.Errorf("cursor should move to the new row (idx 2), got %d", m.Browse.RowCursor)
	}
	if m.Browse.Inserts[0][0].typ != models.Default {
		t.Error("new insert cells should default to DB-default")
	}
	// editing an insert-row cell records a literal value
	m.Browse.EditCol = 0
	m.applyCellEdit("99")
	if m.Browse.Inserts[0][0].typ != models.String || m.Browse.Inserts[0][0].val != "99" {
		t.Errorf("insert cell edit not applied: %+v", m.Browse.Inserts[0][0])
	}
}

func TestReadOnlyBlocksEdits(t *testing.T) {
	m := gridModel()
	m.CurrentConn = &models.Connection{ReadOnly: true}
	res, _ := m.openCellEdit()
	if res.(Model).Screen == types.ScreenCellEdit {
		t.Error("read-only must not open the cell editor")
	}
	res, _ = m.toggleDeleteRow()
	if len(res.(Model).Browse.Deleted) != 0 {
		t.Error("read-only must not stage a delete")
	}
}

func TestBuildDMLChanges_Update(t *testing.T) {
	m := gridModel()
	edited := map[[2]int]string{{0, 1}: "ALPHA"}
	changes := buildDMLChanges("app", "widgets", m.Browse.Columns, m.Browse.Rows, m.Browse.PkColumns, edited, nil, nil)
	if len(changes) != 1 {
		t.Fatalf("want 1 change, got %d", len(changes))
	}
	c := changes[0]
	if c.Type != models.DMLUpdateType {
		t.Errorf("type = %v, want update", c.Type)
	}
	if len(c.PrimaryKeyInfo) != 1 || c.PrimaryKeyInfo[0].Name != "id" || c.PrimaryKeyInfo[0].Value != "1" {
		t.Errorf("PK = %+v, want id=1", c.PrimaryKeyInfo)
	}
	if len(c.Values) != 1 || c.Values[0].Column != "name" || c.Values[0].Value != "ALPHA" {
		t.Errorf("values = %+v", c.Values)
	}
}

func TestBuildDMLChanges_DeleteAndInsert(t *testing.T) {
	cols := []string{"id", "name"}
	rows := [][]string{{"1", "a"}}
	del := map[int]bool{0: true}
	ins := [][]insertCell{{{val: "5", typ: models.String}, {typ: models.Default}}}
	changes := buildDMLChanges("app", "widgets", cols, rows, []string{"id"}, nil, del, ins)
	if len(changes) != 2 {
		t.Fatalf("want delete+insert = 2 changes, got %d", len(changes))
	}
	var sawDelete, sawInsert bool
	for _, c := range changes {
		switch c.Type {
		case models.DMLDeleteType:
			sawDelete = true
			if c.PrimaryKeyInfo[0].Value != "1" {
				t.Errorf("delete PK = %v", c.PrimaryKeyInfo)
			}
		case models.DMLInsertType:
			sawInsert = true
			if c.Values[0].Type != models.String || c.Values[0].Value != "5" {
				t.Errorf("insert col0 = %+v, want string 5", c.Values[0])
			}
			if c.Values[1].Type != models.Default {
				t.Errorf("untouched insert cell should be Default, got %v", c.Values[1].Type)
			}
		}
	}
	if !sawDelete || !sawInsert {
		t.Error("expected one delete and one insert change")
	}
}

func TestPkInfoForRow_WholeRowFallback(t *testing.T) {
	cols := []string{"a", "b"}
	row := []string{"x", "y"}
	infos := pkInfoForRow(cols, row, nil) // no PK -> whole row
	if len(infos) != 2 {
		t.Fatalf("whole-row fallback should yield %d keys, got %d", len(cols), len(infos))
	}
	if infos[0].Name != "a" || infos[0].Value != "x" || infos[1].Value != "y" {
		t.Errorf("fallback infos = %+v", infos)
	}
}

// TestDML_SQLiteRoundTrip drives the full DML chain — buildDMLChanges ->
// driver.ExecutePendingChanges -> re-read — against a real SQLite database.
func TestDML_SQLiteRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dml.db")
	seed, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := seed.Exec(`CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := seed.Exec(`INSERT INTO widgets (id, name) VALUES (1,'alpha'),(2,'beta')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_ = seed.Close()

	driver := &drivers.SQLite{}
	if err := driver.Connect(path); err != nil {
		t.Fatalf("connect: %v", err)
	}

	cols := []string{"id", "name"}
	rows := [][]string{{"1", "alpha"}, {"2", "beta"}}
	changes := buildDMLChanges("", "widgets", cols, rows, []string{"id"},
		map[[2]int]string{{0, 1}: "ALPHA"}, // update row0 name -> ALPHA
		map[int]bool{1: true},              // delete row1 (id=2)
		[][]insertCell{{{typ: models.Default}, {val: "gamma", typ: models.String}}}, // insert (name=gamma)
	)
	if err := driver.ExecutePendingChanges(changes); err != nil {
		t.Fatalf("ExecutePendingChanges: %v", err)
	}

	got, _, _, err := driver.GetRecords("", "widgets", "", "id ASC", 0, 100)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	data := got[1:] // strip header
	names := map[string]bool{}
	for _, r := range data {
		names[r[1]] = true
	}
	if !names["ALPHA"] {
		t.Error("update did not apply (ALPHA missing)")
	}
	if names["beta"] {
		t.Error("delete did not apply (beta still present)")
	}
	if !names["gamma"] {
		t.Error("insert did not apply (gamma missing)")
	}
	if len(data) != 2 {
		t.Errorf("row count = %d, want 2 (alpha->ALPHA, gamma; beta deleted)", len(data))
	}
}

func TestPendingCount(t *testing.T) {
	m := gridModel()
	m.Browse.Edited[[2]int{0, 0}] = "x"
	m.Browse.Edited[[2]int{0, 1}] = "y" // same row -> counts once
	m.Browse.Deleted[1] = true
	m.Browse.Inserts = [][]insertCell{{}}
	if got := m.pendingCount(); got != 3 { // 1 edited row + 1 delete + 1 insert
		t.Errorf("pendingCount = %d, want 3", got)
	}
}
