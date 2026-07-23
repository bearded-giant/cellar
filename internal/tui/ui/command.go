package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/config"
	"github.com/bearded-giant/cellar/internal/tui/types"
)

// paletteCommand is one `:` command. run returns the next model/cmd; keep is
// true when the palette should stay open (e.g. to show an error).
type paletteCommand struct {
	name    string
	aliases []string
	usage   string
	desc    string
	run     func(m Model, args []string) (Model, tea.Cmd, bool)
}

// paletteCommands is the registry — new features add an entry here instead of
// burning another keybind.
var paletteCommands = []paletteCommand{
	{
		name: "backup", aliases: []string{"export"}, usage: "[path]",
		desc: "export connections/queries/history to a tar.gz",
		run: func(m Model, args []string) (Model, tea.Cmd, bool) {
			out := ""
			if len(args) > 0 {
				out = strings.Join(args, " ")
			}
			m.StatusMsg = "Exporting backup…"
			return m, m.Cmds.ExportBackup(out), false
		},
	},
	{
		name: "import", usage: "<backup.tar.gz>",
		desc: "restore a backup (current config kept aside)",
		run: func(m Model, args []string) (Model, tea.Cmd, bool) {
			if len(args) == 0 {
				m.CommandErr = "usage: import <backup.tar.gz>"
				return m, nil, true
			}
			m.StatusMsg = "Restoring backup…"
			return m, m.Cmds.ImportBackup(strings.Join(args, " ")), false
		},
	},
	{
		name: "settings",
		desc: "open the settings screen",
		run: func(m Model, args []string) (Model, tea.Cmd, bool) {
			res, cmd := m.openSettings()
			return res.(Model), cmd, false
		},
	},
	{
		name: "set", usage: "<key> <value>",
		desc: "write an [application] setting (e.g. set BackupDir ~/backups)",
		run: func(m Model, args []string) (Model, tea.Cmd, bool) {
			if len(args) < 2 {
				m.CommandErr = "usage: set <key> <value>"
				return m, nil, true
			}
			name, err := applySetting(m.Cmds, args[0], strings.Join(args[1:], " "))
			if err != nil {
				m.CommandErr = err.Error()
				return m, nil, true
			}
			m.StatusMsg = name + " saved"
			return m, nil, false
		},
	},
	{
		name: "get", usage: "<key>",
		desc: "show an [application] setting",
		run: func(m Model, args []string) (Model, tea.Cmd, bool) {
			if len(args) != 1 {
				m.CommandErr = "usage: get <key>"
				return m, nil, true
			}
			if m.Cmds.AppConfig() == nil {
				m.CommandErr = "no config loaded"
				return m, nil, true
			}
			val, err := config.AppValue(m.Cmds.AppConfig(), args[0])
			if err != nil {
				m.CommandErr = err.Error()
				return m, nil, true
			}
			m.StatusMsg = args[0] + " = " + val
			return m, nil, false
		},
	},
	{
		name: "help",
		desc: "open the keybinding cheatsheet",
		run: func(m Model, args []string) (Model, tea.Cmd, bool) {
			res, cmd := m.openHelp()
			return res.(Model), cmd, false
		},
	},
	{
		name: "quit", aliases: []string{"q"},
		desc: "quit cellar",
		run: func(m Model, args []string) (Model, tea.Cmd, bool) {
			return m, tea.Quit, false
		},
	},
}

func lookupPaletteCommand(name string) *paletteCommand {
	for i := range paletteCommands {
		c := &paletteCommands[i]
		if c.name == name {
			return c
		}
		for _, a := range c.aliases {
			if a == name {
				return c
			}
		}
	}
	return nil
}

func (m Model) openCommandPalette() (tea.Model, tea.Cmd) {
	ti := textinput.New()
	ti.Placeholder = "command…"
	ti.SetWidth(48)
	ti.Focus()
	m.CommandInput = ti
	m.CommandErr = ""
	m.CommandReturn = m.Screen
	m.Screen = types.ScreenCommand
	return m, nil
}

func (m Model) handleCommandScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.Screen = m.CommandReturn
		return m, nil
	case "enter":
		line := strings.TrimSpace(m.CommandInput.Value())
		if line == "" {
			m.Screen = m.CommandReturn
			return m, nil
		}
		fields := strings.Fields(line)
		cmdDef := lookupPaletteCommand(strings.ToLower(fields[0]))
		if cmdDef == nil {
			m.CommandErr = "unknown command: " + fields[0]
			return m, nil
		}
		// leave the palette before running so commands that open a screen
		// (settings, help) return to the palette's origin, not the palette
		m.Screen = m.CommandReturn
		m.CommandErr = ""
		next, cmd, keep := cmdDef.run(m, fields[1:])
		if keep {
			next.Screen = types.ScreenCommand
		}
		return next, cmd
	}
	var cmd tea.Cmd
	m.CommandInput, cmd = m.CommandInput.Update(msg)
	return m, cmd
}

func (m Model) viewCommand() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Command") + "\n\n")
	b.WriteString(accentStyle.Render(": ") + m.CommandInput.View() + "\n")
	if m.CommandErr != "" {
		b.WriteString("\n" + errorStyle.Render(m.CommandErr) + "\n")
	}
	b.WriteString("\n")
	for _, c := range paletteCommands {
		name := c.name
		if len(c.aliases) > 0 {
			name += " (" + strings.Join(c.aliases, ", ") + ")"
		}
		if c.usage != "" {
			name += " " + c.usage
		}
		b.WriteString(normalStyle.Render(padRunes(name, 34)) + dimStyle.Render(c.desc) + "\n")
	}
	b.WriteString("\n" + helpStyle.Render("enter:run · esc:cancel"))
	return m.renderModalW(b.String(), 90)
}
