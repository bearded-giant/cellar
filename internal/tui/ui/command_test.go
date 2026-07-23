package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func enterCommand(t *testing.T, m Model, line string) (Model, tea.Cmd) {
	t.Helper()
	res, _ := m.openCommandPalette()
	m = res.(Model)
	m.CommandInput.SetValue(line)
	res, cmd := m.handleCommandScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	return res.(Model), cmd
}

func TestPalette_OpenFromScreensAndEsc(t *testing.T) {
	m, _ := settingsModel(t)
	res, _ := m.handleConnectionsScreen(keyMsg(':'))
	m = res.(Model)
	if m.Screen != types.ScreenCommand || m.CommandReturn != types.ScreenConnections {
		t.Fatalf("colon should open palette from connections, got %v/%v", m.Screen, m.CommandReturn)
	}
	res, _ = m.handleCommandScreen(tea.KeyPressMsg{Code: tea.KeyEscape})
	if res.(Model).Screen != types.ScreenConnections {
		t.Error("esc should restore the origin screen")
	}

	b := browseModel()
	b.Width, b.Height = 100, 30
	res, _ = b.handleBrowseScreen(keyMsg(':'))
	if res.(Model).Screen != types.ScreenCommand {
		t.Error("colon should open palette from browse")
	}
}

func TestPalette_SetGetRoundTrip(t *testing.T) {
	m, path := settingsModel(t)
	m, cmd := enterCommand(t, m, "set BackupDir ~/cellar backups dir")
	if m.CommandErr != "" || cmd != nil {
		t.Fatalf("set failed: %q", m.CommandErr)
	}
	if m.Screen != types.ScreenConnections {
		t.Errorf("after set, screen = %v, want origin", m.Screen)
	}
	// multi-word value survives, persisted to file
	if got := m.Cmds.AppConfig().BackupDir; got != "~/cellar backups dir" {
		t.Errorf("live BackupDir = %q", got)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "cellar backups dir") {
		t.Error("set did not persist to config file")
	}

	m, _ = enterCommand(t, m, "get backupdir")
	if !strings.Contains(m.StatusMsg, "cellar backups dir") {
		t.Errorf("get status = %q", m.StatusMsg)
	}
}

func TestPalette_UnknownAndUsageErrorsKeepOpen(t *testing.T) {
	m, _ := settingsModel(t)
	m, _ = enterCommand(t, m, "frobnicate now")
	if m.Screen != types.ScreenCommand || !strings.Contains(m.CommandErr, "unknown command") {
		t.Fatalf("unknown command should stay open with error, got %v %q", m.Screen, m.CommandErr)
	}

	m2, _ := settingsModel(t)
	m2, _ = enterCommand(t, m2, "set OnlyKey")
	if m2.Screen != types.ScreenCommand || !strings.Contains(m2.CommandErr, "usage: set") {
		t.Errorf("bad args should stay open with usage, got %v %q", m2.Screen, m2.CommandErr)
	}
}

func TestPalette_BackupImportQuitSettings(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	if err := os.MkdirAll(filepath.Join(root, "cellar"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cellar", "config.toml"), []byte("[[database]]\nname = \"p\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m, _ := settingsModel(t)

	out := filepath.Join(root, "pal.tar.gz")
	m, cmd := enterCommand(t, m, "backup "+out)
	if cmd == nil {
		t.Fatal("backup should fire a command")
	}
	done, ok := cmd().(types.BackupDoneMsg)
	if !ok || done.Err != nil || done.Path != out {
		t.Fatalf("backup msg = %+v", done)
	}

	m, cmd = enterCommand(t, m, "import "+out)
	if cmd == nil {
		t.Fatal("import should fire a command")
	}
	restored, ok := cmd().(types.BackupRestoredMsg)
	if !ok || restored.Err != nil {
		t.Fatalf("import msg = %+v", restored)
	}
	res, followUp := m.Update(restored)
	if followUp == nil {
		t.Error("restore should trigger a config reload")
	}
	if got := res.(Model).StatusMsg; !strings.Contains(got, "Restored") {
		t.Errorf("restore status = %q", got)
	}

	m, cmd = enterCommand(t, m, "q")
	if cmd == nil {
		t.Fatal("q should fire tea.Quit")
	}

	m, _ = enterCommand(t, m, "settings")
	if m.Screen != types.ScreenSettings || m.SettingsReturn != types.ScreenConnections {
		t.Errorf("settings command: screen %v return %v", m.Screen, m.SettingsReturn)
	}
}
