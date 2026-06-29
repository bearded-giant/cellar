package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func TestExportCmd_CSV(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.csv")
	cols := []string{"id", "name"}
	rows := [][]string{{"1", "a"}, {"2", "b"}, {"3", "c"}}

	msg := exportCmd(out, cols, rows)().(types.ExportDoneMsg)
	if msg.Err != nil {
		t.Fatalf("export: %v", msg.Err)
	}
	if msg.Rows != 3 {
		t.Errorf("rows = %d, want 3", msg.Rows)
	}
	body := strings.TrimRight(readFile(t, out), "\n")
	if lines := strings.Count(body, "\n") + 1; lines != 4 {
		t.Errorf("csv lines = %d, want 4 (header + 3 rows)\n%s", lines, body)
	}
	if !strings.Contains(body, "name") {
		t.Error("csv missing header")
	}
}

func TestExportCmd_JSON(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.json")
	msg := exportCmd(out, []string{"id", "name"}, [][]string{{"1", "x"}, {"2", "y"}})().(types.ExportDoneMsg)
	if msg.Err != nil || msg.Rows != 2 {
		t.Fatalf("export json: err=%v rows=%d", msg.Err, msg.Rows)
	}
	if !strings.Contains(readFile(t, out), `"name"`) {
		t.Error("json missing name key")
	}
}

// exportRows returns QueryRows (full result) when present, else the page.
func TestExportRows_PrefersQueryRows(t *testing.T) {
	m := browseModel()
	m.Browse.Rows = [][]string{{"page"}}
	m.Browse.QueryRows = [][]string{{"a"}, {"b"}}
	if got := m.exportRows(); len(got) != 2 {
		t.Errorf("exportRows should return the full QueryRows (2), got %d", len(got))
	}
	m.Browse.QueryRows = nil
	if got := m.exportRows(); len(got) != 1 {
		t.Errorf("exportRows should fall back to the page (1), got %d", len(got))
	}
}

func TestOpenExport_BlockedOnTablePreview(t *testing.T) {
	m := gridModel() // gridModel sets Browse.Table = "widgets" (a real table preview)
	res, _ := m.openExport()
	m = res.(Model)
	if m.Screen == types.ScreenExport {
		t.Error("export must not open on a table preview (unbounded)")
	}
	if !strings.Contains(m.StatusMsg, "query") {
		t.Errorf("expected a query-results hint, got %q", m.StatusMsg)
	}

	// a query result (Table == "") opens the export modal
	m.Browse.Table = ""
	m.Browse.QueryRows = [][]string{{"1", "a"}}
	res, _ = m.openExport()
	if res.(Model).Screen != types.ScreenExport {
		t.Error("export should open on a query result")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
