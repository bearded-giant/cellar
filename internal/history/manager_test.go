package history

import "testing"

func TestAddAndDeleteHistory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	const conn = "testconn"

	// "select 1" twice must collapse to one entry (dedupe)
	for _, q := range []string{"select 1", "select 2", "select 1", "select 3"} {
		if err := AddQueryToHistory(conn, q); err != nil {
			t.Fatalf("add %q: %v", q, err)
		}
	}

	path, _ := GetHistoryFilePath(conn)
	items, err := ReadHistory(path, 0)
	if err != nil || len(items) != 3 {
		t.Fatalf("want 3 deduped items, got %d (err %v)", len(items), err)
	}

	// delete the middle one (match by text + timestamp)
	victim := items[1]
	remaining, err := DeleteQueryFromHistory(conn, victim.QueryText, victim.Timestamp)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("want 2 remaining, got %d", len(remaining))
	}
	for _, it := range remaining {
		if it.QueryText == victim.QueryText && it.Timestamp.Equal(victim.Timestamp) {
			t.Errorf("deleted entry still present: %q", victim.QueryText)
		}
	}

	// persisted to disk
	reloaded, _ := ReadHistory(path, 0)
	if len(reloaded) != 2 {
		t.Errorf("disk should hold 2 after delete, got %d", len(reloaded))
	}
}
