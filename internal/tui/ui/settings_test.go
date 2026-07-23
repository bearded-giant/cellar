package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/commands"
	"github.com/bearded-giant/cellar/internal/tui/config"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

func settingsModel(t *testing.T) (Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[[database]]\nname = \"prod\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	m := Model{Cmds: commands.New(cfg), Screen: types.ScreenConnections}
	m.Width, m.Height = 100, 30
	return m, path
}

func settingIndex(t *testing.T, key string) int {
	t.Helper()
	for i, it := range settingsItems {
		if it.key == key {
			return i
		}
	}
	t.Fatalf("setting %s not in settingsItems", key)
	return -1
}

func TestSettings_OpenEditSavePersists(t *testing.T) {
	m, path := settingsModel(t)

	res, _ := m.handleConnectionsScreen(keyMsg(','))
	m = res.(Model)
	if m.Screen != types.ScreenSettings || m.SettingsReturn != types.ScreenConnections {
		t.Fatalf("comma should open settings (return=connections), got %v/%v", m.Screen, m.SettingsReturn)
	}

	m.SettingsCursor = settingIndex(t, "DefaultPageSize")
	res, _ = m.handleSettingsScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)
	if !m.SettingsEditing {
		t.Fatal("enter on an int setting should start an edit")
	}
	m.SettingsInput.SetValue("250")
	res, _ = m.handleSettingsScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)
	if m.SettingsEditing || m.SettingsErr != "" {
		t.Fatalf("save should close the edit cleanly, err=%q", m.SettingsErr)
	}

	// live-applied: new browse states pick it up
	if got := m.pageSize(); got != 250 {
		t.Errorf("pageSize = %d, want live-applied 250", got)
	}
	// persisted: file has it, connections survive
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "250") || !strings.Contains(string(b), "prod") {
		t.Errorf("config file missing setting or connections:\n%s", b)
	}
}

func TestSettings_ToggleBoolAndValidation(t *testing.T) {
	m, _ := settingsModel(t)
	res, _ := m.openSettings()
	m = res.(Model)

	m.SettingsCursor = settingIndex(t, "DisableSidebar")
	res, _ = m.handleSettingsScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)
	if ac := m.Cmds.AppConfig(); !ac.DisableSidebar {
		t.Error("enter on a bool should toggle it true")
	}

	m.SettingsCursor = settingIndex(t, "QueryRowLimit")
	res, _ = m.handleSettingsScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)
	m.SettingsInput.SetValue("not-a-number")
	res, _ = m.handleSettingsScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = res.(Model)
	if m.SettingsErr == "" || !m.SettingsEditing {
		t.Error("bad int must surface an error and stay in the edit")
	}

	res, _ = m.handleSettingsScreen(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = res.(Model)
	res, _ = m.handleSettingsScreen(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = res.(Model)
	if m.Screen != types.ScreenConnections {
		t.Errorf("esc should return to %v, got %v", types.ScreenConnections, m.Screen)
	}
}

func TestSettings_BrowseCommaAndNilConfig(t *testing.T) {
	m := browseModel()
	m.Width, m.Height = 100, 30
	res, _ := m.handleBrowseScreen(keyMsg(','))
	m = res.(Model)
	// browseModel has no config loaded — must refuse gracefully
	if m.Screen != types.ScreenBrowse {
		t.Errorf("nil config: settings must not open, got %v", m.Screen)
	}

	m2, _ := settingsModel(t)
	m2.Screen = types.ScreenBrowse
	m2.initBrowse(nil)
	res, _ = m2.handleBrowseScreen(keyMsg(','))
	m2 = res.(Model)
	if m2.Screen != types.ScreenSettings || m2.SettingsReturn != types.ScreenBrowse {
		t.Errorf("browse comma should open settings returning to browse, got %v/%v", m2.Screen, m2.SettingsReturn)
	}
}

func TestSettings_ExportBackupKeybind(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	if err := os.MkdirAll(filepath.Join(root, "cellar"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cellar", "config.toml"), []byte("[[database]]\nname = \"p\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	m, _ := settingsModel(t)
	bd := filepath.Join(root, "backups")
	m.Cmds.AppConfig().BackupDir = bd
	res, _ := m.openSettings()
	m = res.(Model)

	res, cmd := m.handleSettingsScreen(keyMsg('x'))
	m = res.(Model)
	if cmd == nil {
		t.Fatal("x should fire the export command")
	}
	done, ok := cmd().(types.BackupDoneMsg)
	if !ok || done.Err != nil {
		t.Fatalf("export failed: %+v err=%v", done, done.Err)
	}
	if filepath.Dir(done.Path) != bd {
		t.Errorf("archive in %s, want BackupDir %s", filepath.Dir(done.Path), bd)
	}

	res2, _ := m.Update(done)
	if got := res2.(Model).StatusMsg; !strings.Contains(got, done.Path) {
		t.Errorf("status %q should carry the archive path", got)
	}
}
