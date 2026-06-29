package ui

import (
	"strings"
	"testing"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

func gridModel() Model {
	m := browseModel()
	m.Browse.Table = "widgets"
	m.Browse.TableDB = "app"
	m.Browse.Label = "widgets"
	m.Browse.Columns = []string{"id", "name"}
	m.Browse.Rows = [][]string{{"1", "alpha"}, {"2", "beta"}}
	m.Browse.PkColumns = []string{"id"}
	m.ActiveDriver = &drivers.SQLite{}
	m.Focus = types.FocusGrid
	return m
}

func TestGenerateDelete_UsesPK(t *testing.T) {
	m := gridModel()
	m.Browse.RowCursor = 0

	res, _ := m.generateDelete()
	m = res.(Model)
	if m.Screen != types.ScreenEditor {
		t.Fatalf("generateDelete should open the editor, screen=%v", m.Screen)
	}
	sql := m.EditorContent
	if !strings.Contains(sql, "DELETE FROM widgets") {
		t.Errorf("missing DELETE target:\n%s", sql)
	}
	if !strings.Contains(sql, "id = ") || !strings.Contains(sql, "1") {
		t.Errorf("WHERE should target the pk id=1:\n%s", sql)
	}
	if strings.Contains(sql, "name = ") {
		t.Errorf("PK delete must not include non-pk columns:\n%s", sql)
	}
}

func TestGenerateDelete_NoPKUsesAllColumns(t *testing.T) {
	m := gridModel()
	m.Browse.PkColumns = nil // no PK -> WHERE every column
	m.Browse.RowCursor = 0

	res, _ := m.generateDelete()
	sql := res.(Model).EditorContent
	if !strings.Contains(sql, "id = ") || !strings.Contains(sql, "name = ") {
		t.Errorf("no-PK delete should constrain on all columns:\n%s", sql)
	}
	if !strings.Contains(sql, " AND ") {
		t.Errorf("no-PK delete should AND the column predicates:\n%s", sql)
	}
}

func TestGenerateDelete_NullCellUsesIsNull(t *testing.T) {
	m := gridModel()
	m.Browse.PkColumns = nil
	m.Browse.Rows = [][]string{{"1", "NULL&"}} // name is SQL NULL
	m.Browse.RowCursor = 0

	res, _ := m.generateDelete()
	sql := res.(Model).EditorContent
	if !strings.Contains(sql, "name IS NULL") {
		t.Errorf("NULL cell should produce IS NULL, not = 'NULL':\n%s", sql)
	}
}

func TestGenerateInsert_Template(t *testing.T) {
	m := gridModel()
	res, _ := m.generateInsert()
	m = res.(Model)
	if m.Screen != types.ScreenEditor {
		t.Fatalf("generateInsert should open the editor, screen=%v", m.Screen)
	}
	sql := m.EditorContent
	if !strings.Contains(sql, "INSERT INTO widgets (id, name)") {
		t.Errorf("missing INSERT column list:\n%s", sql)
	}
	if !strings.Contains(sql, "<id>") || !strings.Contains(sql, "<name>") {
		t.Errorf("VALUES should be column placeholders:\n%s", sql)
	}
}

func TestAppendToEditor_KeepsExistingContent(t *testing.T) {
	m := gridModel()
	m.EditorContent = "SELECT 1;"
	m.Browse.RowCursor = 0

	res, _ := m.generateDelete()
	sql := res.(Model).EditorContent
	if !strings.HasPrefix(sql, "SELECT 1;") {
		t.Errorf("existing editor content should be preserved:\n%s", sql)
	}
	if !strings.Contains(sql, "DELETE FROM widgets") {
		t.Errorf("new statement should be appended:\n%s", sql)
	}
}
