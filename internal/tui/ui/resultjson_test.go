package ui

import (
	"encoding/json"
	"testing"
)

func TestRecordsToJSON(t *testing.T) {
	cols := []string{"id", "name", "note"}
	rows := [][]string{
		{"1", "alpha", "NULL&"},
		{"2", "EMPTY&", "ok"},
	}
	out := recordsToJSON(cols, rows)

	// must be valid JSON and decode to the expected shape
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(decoded) != 2 {
		t.Fatalf("want 2 objects, got %d", len(decoded))
	}
	if decoded[0]["note"] != nil {
		t.Errorf("NULL& should decode to JSON null, got %v", decoded[0]["note"])
	}
	if decoded[1]["name"] != "" {
		t.Errorf("EMPTY& should decode to empty string, got %v", decoded[1]["name"])
	}
	if decoded[0]["name"] != "alpha" {
		t.Errorf("name = %v, want alpha", decoded[0]["name"])
	}
}

func TestRecordsToJSON_Empty(t *testing.T) {
	if got := recordsToJSON(nil, nil); got != "[]" {
		t.Errorf("empty = %q, want []", got)
	}
}
