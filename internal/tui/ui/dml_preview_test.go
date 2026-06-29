package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

func TestCommitPreview_RendersAndCommits(t *testing.T) {
	m := gridModel()
	m.ActiveDriver = &drivers.SQLite{}
	m.Browse.Edited[[2]int{0, 1}] = "X" // update row 0, col name -> X

	res, _ := m.openCommitPreview()
	m = res.(Model)
	if m.Screen != types.ScreenCommitPreview {
		t.Fatal("should open the commit preview")
	}
	if len(m.PreviewSQL) != 1 {
		t.Fatalf("want 1 preview SQL, got %d", len(m.PreviewSQL))
	}
	if !strings.Contains(strings.ToUpper(m.PreviewSQL[0]), "UPDATE") {
		t.Errorf("preview SQL should be an UPDATE, got %q", m.PreviewSQL[0])
	}

	res2, cmd := m.handleCommitPreviewScreen(tea.KeyMsg{Type: tea.KeyEnter})
	if res2.(Model).Screen != types.ScreenBrowse {
		t.Error("commit should return to browse")
	}
	if cmd == nil {
		t.Error("enter should fire the commit command")
	}
}

func TestCommitPreview_RemoveOne(t *testing.T) {
	m := gridModel()
	m.ActiveDriver = &drivers.SQLite{}
	m.Browse.Edited[[2]int{0, 1}] = "X"
	m.Browse.Deleted[1] = true // a second change

	res, _ := m.openCommitPreview()
	m = res.(Model)
	if len(m.PreviewSQL) != 2 {
		t.Fatalf("want 2 changes, got %d", len(m.PreviewSQL))
	}
	res2, _ := m.handleCommitPreviewScreen(keyMsg('d')) // remove the focused one
	m = res2.(Model)
	if len(m.PreviewChanges) != 1 {
		t.Errorf("after remove, want 1 change, got %d", len(m.PreviewChanges))
	}
	if m.Screen != types.ScreenCommitPreview {
		t.Error("removing one (of two) should stay in the preview")
	}
}
