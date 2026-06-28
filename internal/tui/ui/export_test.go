package ui

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

func TestExportAll_SQLiteCSV(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "e.db")
	seed, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := seed.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := seed.Exec(`INSERT INTO t (name) VALUES ('a'),('b'),('c')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_ = seed.Close()

	driver := &drivers.SQLite{}
	if err := driver.Connect(dbPath); err != nil {
		t.Fatalf("connect: %v", err)
	}

	out := filepath.Join(dir, "out.csv")
	msg := exportAllCmd(driver, "", "t", "", "id ASC", out)().(types.ExportDoneMsg)
	if msg.Err != nil {
		t.Fatalf("export: %v", msg.Err)
	}
	if msg.Rows != 3 {
		t.Errorf("rows = %d, want 3", msg.Rows)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	body := strings.TrimRight(string(data), "\n")
	if lines := strings.Count(body, "\n") + 1; lines != 4 {
		t.Errorf("csv lines = %d, want 4 (header + 3 rows)\n%s", lines, body)
	}
	if !strings.Contains(body, "name") {
		t.Error("csv missing header")
	}
}

func TestExportAll_JSON(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "e.db")
	seed, _ := sql.Open("sqlite", dbPath)
	_, _ = seed.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`)
	_, _ = seed.Exec(`INSERT INTO t (name) VALUES ('x'),('y')`)
	_ = seed.Close()

	driver := &drivers.SQLite{}
	_ = driver.Connect(dbPath)
	out := filepath.Join(dir, "out.json")
	msg := exportAllCmd(driver, "", "t", "", "id ASC", out)().(types.ExportDoneMsg)
	if msg.Err != nil || msg.Rows != 2 {
		t.Fatalf("export json: err=%v rows=%d", msg.Err, msg.Rows)
	}
	data, _ := os.ReadFile(out)
	if !strings.Contains(string(data), `"name"`) {
		t.Errorf("json missing name key:\n%s", data)
	}
}
