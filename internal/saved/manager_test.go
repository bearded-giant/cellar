package saved

import "testing"

func TestUpdateSavedQuery(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := SaveQuery("c", "a", "SELECT 1"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := UpdateSavedQuery("c", "a", "SELECT 2"); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if err := UpdateSavedQuery("c", "b", "SELECT 3"); err != nil {
		t.Fatalf("append: %v", err)
	}

	items, err := ReadSavedQueries("c")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d: %+v", len(items), items)
	}
	got := map[string]string{}
	for _, it := range items {
		got[it.Name] = it.Query
	}
	if got["a"] != "SELECT 2" {
		t.Errorf("a = %q, want 'SELECT 2' (overwrite in place)", got["a"])
	}
	if got["b"] != "SELECT 3" {
		t.Errorf("b = %q, want 'SELECT 3' (appended)", got["b"])
	}
}
