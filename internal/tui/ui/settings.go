package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/history"
	"github.com/bearded-giant/cellar/internal/tui/commands"
	"github.com/bearded-giant/cellar/internal/tui/config"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

// settingsItems is the curated UX subset of [application] — deeper knobs stay
// in config.toml (the screen footer points there).
var settingsItems = []struct {
	key    string
	isBool bool
	desc   string
}{
	{"BackupDir", false, "where `cellar export` writes archives (empty = current dir)"},
	{"DefaultPageSize", false, "rows per page when browsing tables"},
	{"QueryRowLimit", false, "editor SELECT fetch cap (0 = default 5000, -1 = unlimited)"},
	{"MaxQueryHistoryPerConnection", false, "history entries kept per connection"},
	{"DisableSidebar", true, "open the SQL editor with the schema sidebar hidden"},
}

func (m Model) openSettings() (tea.Model, tea.Cmd) {
	if m.Cmds.AppConfig() == nil {
		m.StatusMsg = "Settings unavailable — no config loaded"
		return m, nil
	}
	m.SettingsReturn = m.Screen
	m.Screen = types.ScreenSettings
	m.SettingsCursor = 0
	m.SettingsEditing = false
	m.SettingsErr = ""
	return m, nil
}

func (m Model) handleSettingsScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.SettingsEditing {
		switch msg.String() {
		case "esc":
			m.SettingsEditing = false
			m.SettingsErr = ""
			return m, nil
		case "enter":
			return m.saveSettingEdit(strings.TrimSpace(m.SettingsInput.Value()))
		}
		var cmd tea.Cmd
		m.SettingsInput, cmd = m.SettingsInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "esc", "q", ",":
		m.Screen = m.SettingsReturn
		return m, nil
	case "up", "k":
		if m.SettingsCursor > 0 {
			m.SettingsCursor--
		}
		m.SettingsErr = ""
	case "down", "j":
		if m.SettingsCursor < len(settingsItems)-1 {
			m.SettingsCursor++
		}
		m.SettingsErr = ""
	case "x": // run `cellar export` from here — path lands in the status bar
		m.StatusMsg = "Exporting backup…"
		return m, m.Cmds.ExportBackup("")
	case "enter", " ", "space":
		it := settingsItems[m.SettingsCursor]
		if it.isBool {
			cur, _ := config.AppValue(m.Cmds.AppConfig(), it.key)
			return m.saveSettingEdit(strconv.FormatBool(cur != "true"))
		}
		cur, _ := config.AppValue(m.Cmds.AppConfig(), it.key)
		ti := textinput.New()
		ti.SetValue(cur)
		ti.CursorEnd()
		ti.SetWidth(40)
		ti.Focus()
		m.SettingsInput = ti
		m.SettingsEditing = true
		m.SettingsErr = ""
	}
	return m, nil
}

// applySetting validates + applies a setting in memory, persists it to the
// config file, and live-applies side effects where they're cheap. Shared by
// the settings screen and the `:set` palette command.
func applySetting(cmds *commands.Commands, key, value string) (string, error) {
	if cmds.AppConfig() == nil {
		return "", fmt.Errorf("no config loaded")
	}
	name, err := config.ApplyAppSetting(cmds.AppConfig(), key, value)
	if err != nil {
		return "", err
	}
	if path := cmds.ConfigPath(); path != "" {
		if _, err := config.SetAppSetting(path, key, value); err != nil {
			return name, fmt.Errorf("save failed: %w", err)
		}
	}
	if name == "MaxQueryHistoryPerConnection" {
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			history.MaxPerConnection = n
		}
	}
	return name, nil
}

func (m Model) saveSettingEdit(value string) (tea.Model, tea.Cmd) {
	key := settingsItems[m.SettingsCursor].key
	name, err := applySetting(m.Cmds, key, value)
	if err != nil {
		m.SettingsErr = err.Error()
		return m, nil
	}
	m.SettingsEditing = false
	m.SettingsErr = ""
	m.StatusMsg = name + " saved"
	return m, nil
}

func (m Model) viewSettings() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Settings") + "\n\n")

	ac := m.Cmds.AppConfig()
	rule := dimStyle.Render(strings.Repeat("─", 72))
	for i, it := range settingsItems {
		val, _ := config.AppValue(ac, it.key)
		if i == m.SettingsCursor && m.SettingsEditing {
			val = m.SettingsInput.View()
		} else if val == "" {
			val = dimStyle.Render("(unset)")
		}
		name := padRunes(it.key, 34)
		switch {
		case i == m.SettingsCursor && m.SettingsEditing:
			b.WriteString(accentStyle.Render("  ▶ ") + normalStyle.Render(name) + val + "\n")
		case i == m.SettingsCursor:
			b.WriteString(accentStyle.Render("  ▶ ") + normalStyle.Render(name) + accentStyle.Render(val) + "\n")
		default:
			b.WriteString("    " + normalStyle.Render(name) + dimStyle.Render(val) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(rule + "\n\n")
	b.WriteString("  " + dimStyle.Render(settingsItems[m.SettingsCursor].desc) + "\n")
	if m.SettingsErr != "" {
		b.WriteString("\n  " + errorStyle.Render(m.SettingsErr) + "\n")
	}
	b.WriteString("\n  " + renderKeyHints([]kbd{
		{"↑/↓", "select"}, {"enter", "edit / toggle"}, {"x", "export backup"}, {"esc", "back"},
	}, max(m.Width-8, 40)))
	b.WriteString("\n\n  " + dimStyle.Render(fmt.Sprintf("deeper settings: %s (or `cellar config list`)", m.settingsConfigHint())))
	return b.String()
}

func (m Model) settingsConfigHint() string {
	if p := m.Cmds.ConfigPath(); p != "" {
		return p
	}
	return "~/.config/cellar/config.toml"
}
