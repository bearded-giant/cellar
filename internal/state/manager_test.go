package state

import (
	"os"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	const conn = "testconn"

	st := State{Tabs: []Tab{
		{SQL: "select 1", Active: true, SavedName: "one"},
		{SQL: "select 2", Name: "scratch"},
	}}
	if err := Save(conn, st); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load(conn)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Tabs) != 2 {
		t.Fatalf("want 2 tabs, got %d", len(got.Tabs))
	}
	if got.Tabs[0].SQL != "select 1" || !got.Tabs[0].Active || got.Tabs[0].SavedName != "one" {
		t.Errorf("tab 0 mismatch: %+v", got.Tabs[0])
	}
	if got.Tabs[1].Name != "scratch" || got.Tabs[1].Active {
		t.Errorf("tab 1 mismatch: %+v", got.Tabs[1])
	}
	if got.Updated.IsZero() {
		t.Error("Updated not stamped on save")
	}
}

func TestLoadMissingIsEmptyNotError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got, err := Load("nope")
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(got.Tabs) != 0 {
		t.Fatalf("want empty state, got %+v", got)
	}
}

func TestLoadCorruptReturnsErrorAndEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, _ := GetStateFilePath("bad")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load("bad")
	if err == nil {
		t.Fatal("corrupt file should error")
	}
	if len(got.Tabs) != 0 {
		t.Fatalf("corrupt load should return empty state, got %+v", got)
	}
}

func TestSaveEmptyRemovesFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	const conn = "clearme"

	if err := Save(conn, State{Tabs: []Tab{{SQL: "select 1"}}}); err != nil {
		t.Fatal(err)
	}
	path, _ := GetStateFilePath(conn)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file should exist: %v", err)
	}

	// all-blank tabs -> file removed, and removing again is a no-op
	if err := Save(conn, State{Tabs: []Tab{{SQL: "   \n"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("empty save should remove the state file")
	}
	if err := Save(conn, State{}); err != nil {
		t.Fatalf("empty save with no file should be a no-op: %v", err)
	}
}
