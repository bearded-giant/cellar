package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/models"
)

func (m Model) View() tea.View {
	v := tea.NewView(m.viewContent())
	v.AltScreen = true
	// no mouse reporting in the editor so the terminal does native
	// drag-to-select/copy; everywhere else the browse grid wants wheel/click.
	if m.Screen == types.ScreenEditor {
		v.MouseMode = tea.MouseModeNone
	} else {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}

func (m Model) viewContent() string {
	if m.Width > 0 && (m.Width < 50 || m.Height < 15) {
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center,
			"Terminal too small.\nResize to at least 50x15.")
	}

	if m.Screen == types.ScreenBrowse {
		return m.composeInspector(m.composePeek(m.viewBrowse()))
	}
	if m.Screen == types.ScreenEditor {
		return m.composeInspector(m.composePeek(m.viewEditor()))
	}
	if m.Screen == types.ScreenCellView {
		return m.viewCellView()
	}
	if m.Screen == types.ScreenHelp {
		return m.renderModalW(m.viewHelp(), 110) // wide: two padded columns of binds
	}
	if m.Connecting {
		body := m.Spinner.View() + accentStyle.Render(" Connecting") + "\n\n" +
			normalStyle.Render(m.ConnectingTo) + "\n\n" +
			dimStyle.Render("opening the tunnel + database…")
		return m.renderModal(body)
	}

	content := m.getScreenView()
	status := m.getStatusBar()
	fullContent := content + "\n\n" + status

	if m.Width == 0 {
		return fullContent
	}

	vPos := lipgloss.Position(lipgloss.Top)
	switch m.Screen {
	case types.ScreenConnections, types.ScreenAddConnection, types.ScreenEditConnection,
		types.ScreenSSHTunnel, types.ScreenTestConnection, types.ScreenConfirmDelete,
		types.ScreenExport, types.ScreenHistory,
		types.ScreenFilter,
		types.ScreenSaveQuery, types.ScreenSavedQueries,
		types.ScreenYank, types.ScreenTreeFilter, types.ScreenConnFilter,
		types.ScreenSettings, types.ScreenCommand:
		vPos = lipgloss.Center
	}

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, vPos, fullContent,
		lipgloss.WithWhitespaceChars(" "))
}

// getScreenView returns the view for the current screen. A switch (not a map)
// to avoid per-frame heap allocation.
func (m Model) getScreenView() string {
	switch m.Screen {
	case types.ScreenConnections:
		return m.viewConnections()
	case types.ScreenAddConnection:
		return m.viewAddConnection()
	case types.ScreenEditConnection:
		return m.viewEditConnection()
	case types.ScreenSSHTunnel:
		return m.viewSSHTunnel()
	case types.ScreenTestConnection:
		return m.viewTestConnection()
	case types.ScreenConfirmDelete:
		return m.viewConfirmDelete()
	case types.ScreenBrowse:
		return m.viewBrowse()
	case types.ScreenEditor:
		return m.viewEditor()
	case types.ScreenExport:
		return m.viewExport()
	case types.ScreenHistory:
		return m.viewHistory()
	case types.ScreenFilter:
		return m.viewFilter()
	case types.ScreenSaveQuery:
		return m.viewSaveQuery()
	case types.ScreenSavedQueries:
		return m.viewSavedQueries()
	case types.ScreenYank:
		return m.viewYank()
	case types.ScreenCellView:
		return m.viewCellView()
	case types.ScreenTreeFilter:
		return m.viewTreeFilter()
	case types.ScreenConnFilter:
		return m.viewConnFilter()
	case types.ScreenSettings:
		return m.viewSettings()
	case types.ScreenCommand:
		return m.viewCommand()
	default:
		return m.viewConnections()
	}
}

func (m Model) getStatusBar() string {
	var msg string
	switch {
	case m.Loading:
		msg = dimStyle.Render("Loading...")
	case m.StatusMsg != "":
		if strings.HasPrefix(m.StatusMsg, "Error") {
			msg = errorStyle.Render(m.StatusMsg)
		} else {
			msg = successStyle.Render(m.StatusMsg)
		}
	}
	ver := m.Version
	if ver == "" {
		ver = "dev"
	}
	verBadge := dimStyle.Render("cellar " + ver)
	if msg == "" {
		return verBadge
	}
	return msg + dimStyle.Render("  •  ") + verBadge
}

func (m Model) viewConnections() string {
	var b strings.Builder

	logo := `
  ██████╗███████╗██╗     ██╗      █████╗ ██████╗
 ██╔════╝██╔════╝██║     ██║     ██╔══██╗██╔══██╗
 ██║     █████╗  ██║     ██║     ███████║██████╔╝
 ██║     ██╔══╝  ██║     ██║     ██╔══██║██╔══██╗
 ╚██████╗███████╗███████╗███████╗██║  ██║██║  ██║
  ╚═════╝╚══════╝╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝`

	b.WriteString(logoStyle.Render(logo))
	b.WriteString("\n\n")

	b.WriteString(m.buildStatsBar())
	b.WriteString("\n\n")

	if m.ConnectionError != "" {
		errorBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196")).
			Foreground(lipgloss.Color("196")).
			Padding(0, 2).
			Width(min(60, max(m.Width-10, 30))).
			Render(fmt.Sprintf("Connection Failed\n%s", dimStyle.Render(m.ConnectionError)))
		b.WriteString(errorBox)
		b.WriteString("\n\n")
	}

	vis := m.visibleConnIndices()
	filtering := m.Screen == types.ScreenConnFilter || m.ConnFilter != ""

	sectionTitle := fmt.Sprintf("╭─ Saved Connections (%d) ", len(m.Connections))
	if filtering {
		sectionTitle = fmt.Sprintf("╭─ Saved Connections (%d/%d) ", len(vis), len(m.Connections))
	}
	if pad := 50 - len([]rune(sectionTitle)); pad > 0 {
		sectionTitle += strings.Repeat("─", pad)
	}
	sectionTitle += "╮"
	b.WriteString(accentStyle.Render(sectionTitle))
	b.WriteString("\n")

	if m.Screen == types.ScreenConnFilter {
		b.WriteString(" " + keyStyle.Render("/") + " " + m.ConnFilterInput.View())
		b.WriteString("\n")
	} else if m.ConnFilter != "" {
		b.WriteString(" " + keyStyle.Render("/") + " " + accentStyle.Render(m.ConnFilter) + dimStyle.Render("  esc clears"))
		b.WriteString("\n")
	}

	if len(m.Connections) == 0 {
		b.WriteString("\n")
		emptyBox := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(1, 2).
			Render("  No connections saved.\n\n  Press 'a' to add your first connection.")
		b.WriteString(emptyBox)
		b.WriteString("\n")
	} else if len(vis) == 0 {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  No connections match the filter."))
		b.WriteString("\n\n")
	} else {
		b.WriteString("\n")

		maxVisible := max((m.Height-22)/4, 3)
		selectedIdx := clampIndex(m.SelectedConnIdx, len(vis))
		startIdx, endIdx := scrollWindow(selectedIdx, len(vis), maxVisible)

		for i := startIdx; i < endIdx; i++ {
			b.WriteString(m.renderConnCard(m.Connections[vis[i]], i == selectedIdx))
			b.WriteString("\n")
		}

		if len(vis) > maxVisible {
			scrollInfo := fmt.Sprintf("  ↕ %d-%d of %d connections", startIdx+1, endIdx, len(vis))
			b.WriteString(dimStyle.Render(scrollInfo))
			b.WriteString("\n")
		}
	}

	b.WriteString(accentStyle.Render("╰" + strings.Repeat("─", 54) + "╯"))
	b.WriteString("\n\n")

	if m.Screen == types.ScreenConnFilter {
		b.WriteString(helpStyle.Render("type to filter name/host · enter:apply · esc:clear"))
	} else {
		b.WriteString(m.connFooterHelp())
	}

	return b.String()
}

func (m Model) renderConnCard(conn models.Connection, selected bool) string {
	var card strings.Builder

	icon := "○"
	if selected {
		icon = "●"
	}

	nameStyle := normalStyle
	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	}
	fmt.Fprintf(&card, " %s %s", icon, nameStyle.Render(conn.Name))
	card.WriteString("\n")

	// Type + SSH/RO badges only — never the URL (it carries the DB password).
	card.WriteString("   ")
	var badges []string
	if conn.Provider != "" {
		badges = append(badges, badge(conn.Provider, "236", "245"))
	}
	if conn.UseSSH {
		badges = append(badges, badge("SSH", "22", "46"))
	}
	if conn.ReadOnly {
		badges = append(badges, badge("RO", "52", "214"))
	}
	card.WriteString(strings.Join(badges, " "))

	style := connCardStyle
	if selected {
		style = connCardSelectedStyle
	}
	style = style.Width(min(80, max(m.Width-10, 30)))
	return style.Render(card.String())
}

func badge(text, bg, fg string) string {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(bg)).
		Foreground(lipgloss.Color(fg)).
		Padding(0, 1).
		Render(text)
}

func (m Model) buildStatsBar() string {
	boxes := []struct {
		label string
		value string
		color string
	}{
		{"Connections", fmt.Sprintf("%d saved", len(m.Connections)), "39"},
	}
	if m.CurrentConn != nil {
		boxes = append(boxes, struct {
			label string
			value string
			color string
		}{"Active", m.CurrentConn.Name, "2"})
	}

	var statsBoxes []string
	for _, box := range boxes {
		content := fmt.Sprintf("%s\n%s",
			dimStyle.Render(box.label),
			lipgloss.NewStyle().Foreground(lipgloss.Color(box.color)).Bold(true).Render(box.value),
		)
		statsBoxes = append(statsBoxes, statsBoxStyle.Width(18).Render(content))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, statsBoxes...)
}

func (m Model) connFooterHelp() string {
	keybindings := []struct{ key, desc string }{
		{"↑/↓", "navigate"},
		{"enter", "open"},
		{"K/J", "move up/down"},
		{"t", "test"},
		{"a", "add"},
		{"e", "edit"},
		{"D", "duplicate"},
		{"d", "delete"},
		{"/", "filter"},
		{"r", "reload"},
		{",", "settings"},
		{":", "command"},
		{"?", "help"},
		{"ctrl+c", "quit"},
	}

	var help strings.Builder
	for i, kb := range keybindings {
		help.WriteString(badge(kb.key, "236", "255"))
		help.WriteString(" ")
		help.WriteString(dimStyle.Render(kb.desc))
		if i < len(keybindings)-1 {
			help.WriteString("  ")
		}
	}
	return help.String()
}

func (m Model) viewAddConnection() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Add Connection"))
	b.WriteString("\n\n")
	b.WriteString(m.renderConnForm())
	b.WriteString(badge("Ctrl+T", "22", "46"))
	b.WriteString(dimStyle.Render(" test  "))
	b.WriteString(badge("Ctrl+S", "22", "46"))
	b.WriteString(dimStyle.Render(" SSH tunnel"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("tab:next  space:toggle  enter:save  esc:cancel"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("in a field: ctrl+a/ctrl+e home/end  ctrl+u/ctrl+k clear to start/end"))
	return m.renderModalW(b.String(), m.formModalWidth())
}

func (m Model) viewEditConnection() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Edit Connection"))
	b.WriteString("\n\n")
	b.WriteString(m.renderConnForm())
	b.WriteString(badge("Ctrl+T", "22", "46"))
	b.WriteString(dimStyle.Render(" test  "))
	b.WriteString(badge("Ctrl+S", "22", "46"))
	b.WriteString(dimStyle.Render(" SSH tunnel"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("tab:next  space:toggle  enter:save  esc:cancel"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("in a field: ctrl+a/ctrl+e home/end  ctrl+u/ctrl+k clear to start/end"))
	return m.renderModalW(b.String(), m.formModalWidth())
}

// renderConnForm renders the shared connection form: Name, URL, Provider text
// inputs, a ReadOnly toggle, and the SSH tri-state summary line.
func (m Model) renderConnForm() string {
	var b strings.Builder

	labels := []string{"Name", "URL", "Provider", "Schema", "Vault Command"}
	for i := range labels {
		b.WriteString(focusedLabelStyle(m.ConnFocusIdx == i).Render(labels[i] + ":"))
		b.WriteString("\n")
		b.WriteString(m.ConnInputs[i].View())
		b.WriteString("\n\n")
	}

	toggleIdx := m.connReadOnlyFocusIdx()
	b.WriteString(focusedLabelStyle(m.ConnFocusIdx == toggleIdx).Render("Read Only:"))
	b.WriteString("\n")
	checkbox := "[ ] Read-only mode"
	if m.ConnReadOnly {
		checkbox = "[x] Read-only mode"
	}
	cbStyle := normalStyle
	if m.ConnFocusIdx == toggleIdx {
		cbStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	}
	b.WriteString(cbStyle.Render(checkbox))
	b.WriteString("\n\n")

	sshStatus := "not configured"
	if m.SSHEnabled && m.PendingSSH != nil {
		sshStatus = "enabled (" + m.PendingSSH.SSHHost + ")"
	} else if m.PendingSSH != nil {
		sshStatus = "configured (disabled)"
	}
	b.WriteString(keyStyle.Render("SSH:"))
	b.WriteString(" ")
	b.WriteString(normalStyle.Render(sshStatus))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("ctrl+s: configure SSH tunnel"))
	b.WriteString("\n\n")

	return b.String()
}

func (m Model) viewSSHTunnel() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("SSH Tunnel"))
	b.WriteString("\n\n")

	labels := []string{
		"Bastion Host", "Bastion Port", "SSH User",
		"Private Key Path", "Passphrase", "SSH Password",
		"Proxy Command (SSM/jump)",
	}
	for i, label := range labels {
		b.WriteString(focusedLabelStyle(m.SSHFocusIdx == i).Render(label + ":"))
		b.WriteString("\n")
		b.WriteString(m.SSHInputs[i].View())
		b.WriteString("\n\n")
	}

	b.WriteString(focusedLabelStyle(m.SSHFocusIdx == sshToggleIdx).Render("Enable SSH:"))
	b.WriteString("\n")
	checkbox := "[ ] Use SSH tunnel"
	if m.SSHEnabled {
		checkbox = "[x] Use SSH tunnel"
	}
	cbStyle := normalStyle
	if m.SSHFocusIdx == sshToggleIdx {
		cbStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	}
	b.WriteString(cbStyle.Render(checkbox))
	b.WriteString("\n\n")

	if m.SSHTunnelStatus != "" {
		statusStyle := normalStyle
		if strings.HasPrefix(m.SSHTunnelStatus, "SSH failed") {
			statusStyle = errorStyle
		} else if m.SSHTunnelStatus == "SSH OK" {
			statusStyle = successStyle
		}
		b.WriteString(statusStyle.Render(m.SSHTunnelStatus))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("Strict known_hosts. Add host first via `ssh user@host`."))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("tab:next  space:toggle  ctrl+t:test  enter:save  esc:cancel"))

	return m.renderModalW(b.String(), m.formModalWidth())
}

func (m Model) viewTestConnection() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Test Connection"))
	b.WriteString("\n\n")

	switch {
	case m.Loading:
		b.WriteString(dimStyle.Render("Testing connection..."))
	case m.TestResult != "":
		if strings.HasPrefix(m.TestResult, "Failed") {
			b.WriteString(errorStyle.Render(m.TestResult))
		} else {
			b.WriteString(successStyle.Render(m.TestResult))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("esc:back"))
	return m.renderModal(b.String())
}

func (m Model) viewConfirmDelete() string {
	var b strings.Builder
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)

	title, msg := "Confirm Delete", ""
	switch m.ConfirmType {
	case "connection":
		if conn, ok := m.ConfirmData.(models.Connection); ok {
			msg = fmt.Sprintf("Delete connection '%s'?", conn.Name)
		}
	case "disconnect":
		title = "Disconnect"
		name := ""
		if m.CurrentConn != nil {
			name = m.CurrentConn.Name
		}
		msg = fmt.Sprintf("Disconnect from '%s'?", name)
	}

	b.WriteString(warningStyle.Render(title))
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render(msg))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("[y] confirm  [n/esc/q] cancel"))

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("1")).
		Padding(1, 2).
		Width(50)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalStyle.Render(b.String()))
}
