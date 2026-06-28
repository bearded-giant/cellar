package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
	"github.com/jorgerojas26/lazysql/models"
)

// uniqueDupName returns a name for a duplicated connection that does not collide
// with any existing name. Tries "{base}-copy", then "{base}-copy-2", etc. Names
// are the de-dupe key for lazysql connections, so this must never clash.
func uniqueDupName(base string, existing []models.Connection) string {
	taken := make(map[string]struct{}, len(existing))
	for _, c := range existing {
		taken[c.Name] = struct{}{}
	}
	candidate := base + "-copy"
	if _, clash := taken[candidate]; !clash {
		return candidate
	}
	for i := 2; ; i++ {
		candidate = fmt.Sprintf("%s-copy-%d", base, i)
		if _, clash := taken[candidate]; !clash {
			return candidate
		}
	}
}

// formatConnectError surfaces the strict-known_hosts ssh-keyscan hint when it
// rides along on a tunnel error so the user can act on it from the UI.
func formatConnectError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (m Model) handleConnectionsScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.SelectedConnIdx > 0 {
			m.SelectedConnIdx--
			m.ConnectionError = ""
		}
	case "down", "j":
		if m.SelectedConnIdx < len(m.Connections)-1 {
			m.SelectedConnIdx++
			m.ConnectionError = ""
		}
	case "enter":
		if len(m.Connections) > 0 && m.SelectedConnIdx < len(m.Connections) {
			conn := m.Connections[m.SelectedConnIdx]
			m.CurrentConn = &conn
			m.Loading = true
			m.StatusMsg = "Connecting..."
			m.ConnectionError = ""
			return m, m.Cmds.Connect(conn, false)
		}
	case "b":
		if len(m.Connections) > 0 && m.SelectedConnIdx < len(m.Connections) {
			conn := m.Connections[m.SelectedConnIdx]
			m.CurrentConn = &conn
			m.Loading = true
			m.StatusMsg = "Connecting..."
			m.ConnectionError = ""
			return m, m.Cmds.Connect(conn, true)
		}
	case "t":
		if len(m.Connections) > 0 && m.SelectedConnIdx < len(m.Connections) {
			conn := m.Connections[m.SelectedConnIdx]
			m.Loading = true
			m.TestResult = ""
			m.TestReturnScreen = types.ScreenConnections
			m.Screen = types.ScreenTestConnection
			return m, m.Cmds.TestConnection(conn)
		}
	case "a", "n":
		m.Screen = types.ScreenAddConnection
		m.resetConnInputs()
		m.EditingConnection = nil
		m.DuplicatingFrom = nil
	case "e":
		if len(m.Connections) > 0 && m.SelectedConnIdx < len(m.Connections) {
			conn := m.Connections[m.SelectedConnIdx]
			m.EditingConnection = &conn
			m.DuplicatingFrom = nil
			m.populateConnInputs(conn)
			m.Screen = types.ScreenEditConnection
		}
	case "D":
		if len(m.Connections) > 0 && m.SelectedConnIdx < len(m.Connections) {
			src := m.Connections[m.SelectedConnIdx]
			srcCopy := src
			m.DuplicatingFrom = &srcCopy
			dup := src
			dup.Name = uniqueDupName(src.Name, m.Connections)
			m.EditingConnection = nil
			m.populateConnInputs(dup)
			m.Screen = types.ScreenAddConnection
			m.ConnectionError = ""
		}
	case "d", "delete", "backspace":
		if len(m.Connections) > 0 && m.SelectedConnIdx < len(m.Connections) {
			m.ConfirmType = "connection"
			m.ConfirmData = m.Connections[m.SelectedConnIdx]
			m.ConfirmReturnScreen = types.ScreenConnections
			m.Screen = types.ScreenConfirmDelete
		}
	case "r":
		m.Loading = true
		return m, m.Cmds.LoadConnections()
	}
	return m, nil
}

func (m Model) handleAddConnectionScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	return m.handleConnFormScreen(msg)
}

func (m Model) handleEditConnectionScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	return m.handleConnFormScreen(msg)
}

// handleConnFormScreen drives the shared Add/Edit connection form. The two
// screens differ only in the enter (save vs update) and esc (which staged
// pointer to clear) arms, gated on whether an EditingConnection is set.
func (m Model) handleConnFormScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fieldCount := m.connFieldCount()
	toggleIdx := m.connReadOnlyFocusIdx()
	isEdit := m.EditingConnection != nil

	switch msg.String() {
	case "tab", "down":
		m.blurConnField()
		m.ConnFocusIdx = (m.ConnFocusIdx + 1) % fieldCount
		m.focusConnField()
	case "shift+tab", "up":
		m.blurConnField()
		m.ConnFocusIdx--
		if m.ConnFocusIdx < 0 {
			m.ConnFocusIdx = fieldCount - 1
		}
		m.focusConnField()
	case " ":
		if m.ConnFocusIdx == toggleIdx {
			m.ConnReadOnly = !m.ConnReadOnly
			return m, nil
		}
		return m.updateConnInputs(msg)
	case "enter":
		if m.ConnFocusIdx == toggleIdx {
			m.ConnReadOnly = !m.ConnReadOnly
			return m, nil
		}
		if !m.connFormValid() {
			m.ConnectionError = "Name and URL are required"
			return m, nil
		}
		m.Loading = true
		if isEdit {
			conn := m.convertCurrentInputsToConnection(m.ConnInputs, ActionEdit)
			return m, m.Cmds.SaveConnection(conn, m.Connections, true, m.EditingConnection.Name)
		}
		conn := m.convertCurrentInputsToConnection(m.ConnInputs, ActionAdd)
		return m, m.Cmds.SaveConnection(conn, m.Connections, false, "")
	case "ctrl+t":
		m.Loading = true
		m.TestResult = ""
		m.TestReturnScreen = m.Screen
		m.Screen = types.ScreenTestConnection
		conn := m.convertCurrentInputsToConnection(m.ConnInputs, ActionTest)
		return m, m.Cmds.TestConnection(conn)
	case "ctrl+s":
		m.populateSSHInputs(m.PendingSSH)
		m.Screen = types.ScreenSSHTunnel
		return m, nil
	case "esc":
		m.Screen = types.ScreenConnections
		if isEdit {
			m.EditingConnection = nil
		} else {
			m.DuplicatingFrom = nil
		}
		m.resetConnInputs()
	default:
		return m.updateConnInputs(msg)
	}
	return m, nil
}

// connFormValid requires a name and a URL. Provider is inferred from the URL
// scheme via helpers.ParseConnectionString at save/connect time when blank.
func (m Model) connFormValid() bool {
	return strings.TrimSpace(m.ConnInputs[0].Value()) != "" &&
		strings.TrimSpace(m.ConnInputs[1].Value()) != ""
}

// connInputIndex maps a ConnFocusIdx to the ConnInputs array index. Indices
// 0..len-1 map directly; the last focus index is the ReadOnly toggle (no input).
func connInputIndex(focusIdx, inputCount int) int {
	if focusIdx >= 0 && focusIdx < inputCount {
		return focusIdx
	}
	return -1
}

func (m *Model) blurConnField() {
	idx := connInputIndex(m.ConnFocusIdx, len(m.ConnInputs))
	if idx >= 0 && idx < len(m.ConnInputs) {
		m.ConnInputs[idx].Blur()
	}
}

func (m *Model) focusConnField() {
	idx := connInputIndex(m.ConnFocusIdx, len(m.ConnInputs))
	if idx >= 0 && idx < len(m.ConnInputs) {
		m.ConnInputs[idx].Focus()
	}
}

func (m Model) updateConnInputs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	idx := connInputIndex(m.ConnFocusIdx, len(m.ConnInputs))
	if idx >= 0 && idx < len(m.ConnInputs) {
		var inputCmd tea.Cmd
		m.ConnInputs[idx], inputCmd = m.ConnInputs[idx].Update(msg)
		return m, inputCmd
	}
	return m, nil
}

func (m Model) handleTestConnectionScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.Screen = m.TestReturnScreen
	}
	return m, nil
}

func (m Model) handleConfirmDeleteScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cancelScreen := types.ScreenConnections
	if m.ConfirmReturnScreen != types.ScreenConnections {
		cancelScreen = m.ConfirmReturnScreen
	}
	switch msg.String() {
	case "y", "Y", "enter":
		if m.ConfirmType == "connection" {
			if conn, ok := m.ConfirmData.(models.Connection); ok {
				m.Loading = true
				return m, m.Cmds.DeleteConnection(conn.Name, m.Connections)
			}
		}
		m.Screen = cancelScreen
	case "n", "N", "esc":
		m.Screen = cancelScreen
	}
	return m, nil
}

// SSH tunnel sub-screen. Reached via Ctrl+S from Add/Edit connection.
//
//	0 host, 1 port, 2 user, 3 key path, 4 passphrase, 5 password, 6 proxy command
//
// Plus a focusable "SSH enabled" toggle at index 7.
const sshFieldCount = 8
const sshToggleIdx = 7

func (m Model) handleSSHTunnelScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		m.blurSSHField()
		m.SSHFocusIdx = (m.SSHFocusIdx + 1) % sshFieldCount
		m.focusSSHField()
	case "shift+tab", "up":
		m.blurSSHField()
		m.SSHFocusIdx--
		if m.SSHFocusIdx < 0 {
			m.SSHFocusIdx = sshFieldCount - 1
		}
		m.focusSSHField()
	case " ":
		if m.SSHFocusIdx == sshToggleIdx {
			m.SSHEnabled = !m.SSHEnabled
			return m, nil
		}
		return m.updateSSHInputs(msg)
	case "ctrl+t":
		staged := m.convertSSHInputs()
		if staged == nil {
			m.SSHTunnelStatus = "host required"
			return m, nil
		}
		m.SSHTunnelStatus = "testing..."
		return m, m.Cmds.TestSSH(*staged)
	case "enter":
		if m.SSHFocusIdx == sshToggleIdx {
			m.SSHEnabled = !m.SSHEnabled
			return m, nil
		}
		m.PendingSSH = m.convertSSHInputs()
		if m.PendingSSH == nil {
			m.SSHEnabled = false
		}
		if m.EditingConnection != nil {
			m.Screen = types.ScreenEditConnection
		} else {
			m.Screen = types.ScreenAddConnection
		}
		return m, nil
	case "esc":
		if m.EditingConnection != nil {
			m.Screen = types.ScreenEditConnection
		} else {
			m.Screen = types.ScreenAddConnection
		}
		return m, nil
	default:
		return m.updateSSHInputs(msg)
	}
	return m, nil
}

func sshInputIndex(focusIdx int) int {
	if focusIdx >= 0 && focusIdx <= 6 {
		return focusIdx
	}
	return -1
}

func (m *Model) blurSSHField() {
	idx := sshInputIndex(m.SSHFocusIdx)
	if idx >= 0 && idx < len(m.SSHInputs) {
		m.SSHInputs[idx].Blur()
	}
}

func (m *Model) focusSSHField() {
	idx := sshInputIndex(m.SSHFocusIdx)
	if idx >= 0 && idx < len(m.SSHInputs) {
		m.SSHInputs[idx].Focus()
	}
}

func (m Model) updateSSHInputs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	idx := sshInputIndex(m.SSHFocusIdx)
	if idx >= 0 && idx < len(m.SSHInputs) {
		var inputCmd tea.Cmd
		m.SSHInputs[idx], inputCmd = m.SSHInputs[idx].Update(msg)
		return m, inputCmd
	}
	return m, nil
}
