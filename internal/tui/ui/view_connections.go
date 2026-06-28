package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
	"github.com/jorgerojas26/lazysql/models"
)

func (m Model) View() string {
	if m.Width > 0 && (m.Width < 50 || m.Height < 15) {
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center,
			"Terminal too small.\nResize to at least 50x15.")
	}

	if m.Screen == types.ScreenBrowse {
		return m.viewBrowse()
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
		types.ScreenSSHTunnel, types.ScreenTestConnection, types.ScreenConfirmDelete:
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
	verBadge := dimStyle.Render("lazytea " + ver)
	if msg == "" {
		return verBadge
	}
	return msg + dimStyle.Render("  •  ") + verBadge
}

func (m Model) viewConnections() string {
	var b strings.Builder

	logo := `
 ██╗      █████╗ ███████╗██╗   ██╗████████╗███████╗ █████╗
 ██║     ██╔══██╗╚══███╔╝╚██╗ ██╔╝╚══██╔══╝██╔════╝██╔══██╗
 ██║     ███████║  ███╔╝  ╚████╔╝    ██║   █████╗  ███████║
 ██║     ██╔══██║ ███╔╝    ╚██╔╝     ██║   ██╔══╝  ██╔══██║
 ███████╗██║  ██║███████╗   ██║      ██║   ███████╗██║  ██║
 ╚══════╝╚═╝  ╚═╝╚══════╝   ╚═╝      ╚═╝   ╚══════╝╚═╝  ╚═╝`

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

	sectionTitle := fmt.Sprintf("╭─ Saved Connections (%d) ", len(m.Connections))
	if pad := 50 - len([]rune(sectionTitle)); pad > 0 {
		sectionTitle += strings.Repeat("─", pad)
	}
	sectionTitle += "╮"
	b.WriteString(accentStyle.Render(sectionTitle))
	b.WriteString("\n")

	if len(m.Connections) == 0 {
		b.WriteString("\n")
		emptyBox := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(1, 2).
			Render("  No connections saved.\n\n  Press 'a' to add your first connection.")
		b.WriteString(emptyBox)
		b.WriteString("\n")
	} else {
		b.WriteString("\n")

		maxVisible := max((m.Height-22)/4, 3)

		selectedIdx := m.SelectedConnIdx
		if selectedIdx >= len(m.Connections) {
			selectedIdx = len(m.Connections) - 1
		}
		if selectedIdx < 0 {
			selectedIdx = 0
		}

		startIdx := 0
		if selectedIdx >= maxVisible {
			startIdx = selectedIdx - maxVisible + 1
		}
		endIdx := startIdx + maxVisible
		if endIdx > len(m.Connections) {
			endIdx = len(m.Connections)
			if endIdx-startIdx < maxVisible {
				startIdx = max(endIdx-maxVisible, 0)
			}
		}

		for i := startIdx; i < endIdx; i++ {
			b.WriteString(m.renderConnCard(m.Connections[i], i == selectedIdx))
			b.WriteString("\n")
		}

		if len(m.Connections) > maxVisible {
			scrollInfo := fmt.Sprintf("  ↕ %d-%d of %d connections", startIdx+1, endIdx, len(m.Connections))
			b.WriteString(dimStyle.Render(scrollInfo))
			b.WriteString("\n")
		}
	}

	b.WriteString(accentStyle.Render("╰" + strings.Repeat("─", 54) + "╯"))
	b.WriteString("\n\n")

	b.WriteString(m.connFooterHelp())

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
		{"enter", "connect"},
		{"b", "browse"},
		{"t", "test"},
		{"a", "add"},
		{"e", "edit"},
		{"D", "duplicate"},
		{"d", "delete"},
		{"r", "reload"},
		{"q", "quit"},
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
	return m.renderModal(b.String())
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
	return m.renderModal(b.String())
}

// renderConnForm renders the shared connection form: Name, URL, Provider text
// inputs, a ReadOnly toggle, and the SSH tri-state summary line.
func (m Model) renderConnForm() string {
	var b strings.Builder

	labels := []string{"Name", "URL", "Provider"}
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

	return m.renderModal(b.String())
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

	b.WriteString(warningStyle.Render("Confirm Delete"))
	b.WriteString("\n\n")

	if m.ConfirmType == "connection" {
		if conn, ok := m.ConfirmData.(models.Connection); ok {
			b.WriteString(normalStyle.Render(fmt.Sprintf("Delete connection '%s'?", conn.Name)))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("[y] confirm  [n/esc] cancel"))

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("1")).
		Padding(1, 2).
		Width(50)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalStyle.Render(b.String()))
}
