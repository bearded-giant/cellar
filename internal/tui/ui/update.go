package ui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/types"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.sizeFormInputs()
		if m.Screen == types.ScreenEditor {
			ew, _, _ := m.queryLayout()
			m.EditorArea.SetWidth(ew)
			m.syncEditorHeight()
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case tea.PasteMsg:
		return m.handlePaste(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case spinner.TickMsg:
		if !m.Connecting && !m.QueryRunning {
			return m, nil // stop ticking once the connection/query resolves
		}
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd

	case types.ConnectionsLoadedMsg:
		return m.handleConnectionsLoadedMsg(msg)
	case types.ConnectionSavedMsg:
		return m.handleConnectionSavedMsg(msg)
	case types.ConnectionDeletedMsg:
		return m.handleConnectionDeletedMsg(msg)
	case types.ConnectionsReorderedMsg:
		return m.handleConnectionsReorderedMsg(msg)
	case types.ConnectedMsg:
		return m.handleConnectedMsg(msg)
	case types.TestResultMsg:
		return m.handleTestResultMsg(msg)
	case types.SSHTestMsg:
		return m.handleSSHTestMsg(msg)
	case types.DatabasesLoadedMsg:
		return m.handleDatabasesLoadedMsg(msg)
	case types.TablesLoadedMsg:
		return m.handleTablesLoadedMsg(msg)
	case types.ViewsLoadedMsg:
		return m.handleViewsLoadedMsg(msg)
	case types.RecordsLoadedMsg:
		return m.handleRecordsLoadedMsg(msg)
	case types.QueryExecutedMsg:
		return m.handleQueryExecutedMsg(msg)
	case types.ColumnsLoadedMsg:
		return m.handleColumnsLoadedMsg(msg)
	case types.ExportDoneMsg:
		return m.handleExportDoneMsg(msg)
	case types.PrimaryKeyLoadedMsg:
		return m.handlePrimaryKeyLoadedMsg(msg)
	case types.HistoryLoadedMsg:
		return m.handleHistoryLoadedMsg(msg)
	case types.MetaLoadedMsg:
		return m.handleMetaLoadedMsg(msg)
	case types.TableDDLLoadedMsg:
		return m.handleTableDDLLoadedMsg(msg)
	case types.ViewDefinitionLoadedMsg:
		return m.handleViewDefinitionLoadedMsg(msg)
	case types.ForeignKeysLoadedMsg:
		return m.handleForeignKeysLoadedMsg(msg)
	case types.SavedQuerySavedMsg:
		return m.handleSavedQuerySavedMsg(msg)
	case types.SavedQueriesLoadedMsg:
		return m.handleSavedQueriesLoadedMsg(msg)
	case types.QueryStateLoadedMsg:
		return m.handleQueryStateLoadedMsg(msg)
	case types.QueryStateSavedMsg:
		return m.handleQueryStateSavedMsg(msg)
	case types.BackupDoneMsg:
		if msg.Err != nil {
			m.StatusMsg = "Backup failed: " + msg.Err.Error()
		} else {
			m.StatusMsg = "Backup written: " + msg.Path + " (contains credentials)"
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit // exit the program
	case "ctrl+q":
		// disconnect the live connection from anywhere (confirmed); no-op at the list
		if m.ActiveDriver != nil || m.ActiveTunnel != nil {
			return m.confirmDisconnect()
		}
		return m, nil
	case "ctrl+g":
		return m.openHelp() // help is reachable everywhere, incl. the editor text box
	}

	switch m.Screen {
	case types.ScreenConnections:
		return m.handleConnectionsScreen(msg)
	case types.ScreenAddConnection:
		return m.handleAddConnectionScreen(msg)
	case types.ScreenEditConnection:
		return m.handleEditConnectionScreen(msg)
	case types.ScreenSSHTunnel:
		return m.handleSSHTunnelScreen(msg)
	case types.ScreenTestConnection:
		return m.handleTestConnectionScreen(msg)
	case types.ScreenConfirmDelete:
		return m.handleConfirmDeleteScreen(msg)
	case types.ScreenBrowse:
		return m.handleBrowseScreen(msg)
	case types.ScreenEditor:
		return m.handleEditorScreen(msg)
	case types.ScreenExport:
		return m.handleExportScreen(msg)
	case types.ScreenHistory:
		return m.handleHistoryScreen(msg)
	case types.ScreenFilter:
		return m.handleFilterScreen(msg)
	case types.ScreenSaveQuery:
		return m.handleSaveQueryScreen(msg)
	case types.ScreenSavedQueries:
		return m.handleSavedQueriesScreen(msg)
	case types.ScreenYank:
		return m.handleYankScreen(msg)
	case types.ScreenCellView:
		return m.handleCellViewScreen(msg)
	case types.ScreenTreeFilter:
		return m.handleTreeFilterScreen(msg)
	case types.ScreenConnFilter:
		return m.handleConnFilterScreen(msg)
	case types.ScreenHelp:
		return m.handleHelpScreen(msg)
	case types.ScreenSettings:
		return m.handleSettingsScreen(msg)
	}
	return m, nil
}

// handlePaste routes bracketed-paste content to whatever is accepting text on
// the current screen: the SQL editor, or the focused textinput.
func (m Model) handlePaste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	switch m.Screen {
	case types.ScreenEditor:
		if m.Focus != types.FocusEditor {
			return m, nil
		}
		m.EditorArea.Paste(msg.Content)
		m.syncEditorHeight()
		m.refreshCompletions()
		return m, m.ensureRefColumns()
	case types.ScreenAddConnection, types.ScreenEditConnection:
		return m.updateConnInputs(msg)
	case types.ScreenSSHTunnel:
		return m.updateSSHInputs(msg)
	case types.ScreenExport:
		var cmd tea.Cmd
		m.ExportInput, cmd = m.ExportInput.Update(msg)
		return m, cmd
	case types.ScreenFilter:
		var cmd tea.Cmd
		m.FilterInput, cmd = m.FilterInput.Update(msg)
		return m, cmd
	case types.ScreenTreeFilter:
		var cmd tea.Cmd
		m.TreeFilterInput, cmd = m.TreeFilterInput.Update(msg)
		return m, cmd
	case types.ScreenConnFilter:
		var cmd tea.Cmd
		m.ConnFilterInput, cmd = m.ConnFilterInput.Update(msg)
		m.ConnFilter = strings.TrimSpace(m.ConnFilterInput.Value())
		m.SelectedConnIdx = clampIndex(m.SelectedConnIdx, len(m.visibleConnIndices()))
		return m, cmd
	case types.ScreenSaveQuery:
		var cmd tea.Cmd
		m.SaveNameInput, cmd = m.SaveNameInput.Update(msg)
		return m, cmd
	case types.ScreenSettings:
		if m.SettingsEditing {
			var cmd tea.Cmd
			m.SettingsInput, cmd = m.SettingsInput.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) handleConnectionsLoadedMsg(msg types.ConnectionsLoadedMsg) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.Err != nil {
		m.Err = msg.Err
		m.StatusMsg = "Error: " + msg.Err.Error()
		return m, nil
	}
	m.Connections = msg.Connections
	m.StatusMsg = ""
	if m.SelectedConnIdx >= len(m.Connections) {
		m.SelectedConnIdx = len(m.Connections) - 1
	}
	if m.SelectedConnIdx < 0 {
		m.SelectedConnIdx = 0
	}
	return m, nil
}

func (m Model) handleConnectionSavedMsg(msg types.ConnectionSavedMsg) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.Err != nil {
		m.StatusMsg = "Error: " + msg.Err.Error()
		return m, nil
	}

	if msg.IsEdit {
		original := msg.Connection.Name
		if m.EditingConnection != nil {
			original = m.EditingConnection.Name
		}
		for i := range m.Connections {
			if m.Connections[i].Name == original {
				m.Connections[i] = msg.Connection
				break
			}
		}
		m.StatusMsg = "Connection updated"
	} else {
		m.Connections = append(m.Connections, msg.Connection)
		m.SelectedConnIdx = len(m.Connections) - 1
		m.StatusMsg = "Connection added"
	}

	m.Screen = types.ScreenConnections
	m.EditingConnection = nil
	m.DuplicatingFrom = nil
	m.resetConnInputs()
	return m, nil
}

func (m Model) handleConnectionsReorderedMsg(msg types.ConnectionsReorderedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.StatusMsg = "Reorder failed: " + msg.Err.Error()
	}
	return m, nil
}

func (m Model) handleConnectionDeletedMsg(msg types.ConnectionDeletedMsg) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.Err != nil {
		m.StatusMsg = "Error: " + msg.Err.Error()
		m.Screen = types.ScreenConnections
		return m, nil
	}
	for i := range m.Connections {
		if m.Connections[i].Name == msg.Name {
			m.Connections = append(m.Connections[:i], m.Connections[i+1:]...)
			break
		}
	}
	if m.SelectedConnIdx >= len(m.Connections) && m.SelectedConnIdx > 0 {
		m.SelectedConnIdx--
	}
	m.StatusMsg = "Connection deleted"
	m.Screen = types.ScreenConnections
	return m, nil
}

func (m Model) handleConnectedMsg(msg types.ConnectedMsg) (tea.Model, tea.Cmd) {
	m.Loading = false
	m.Connecting = false
	if msg.Err != nil {
		m.ConnectionError = formatConnectError(msg.Err)
		m.StatusMsg = "Error: connection failed"
		return m, nil
	}
	m.ConnectionError = ""
	if m.ActiveTunnel != nil {
		_ = m.ActiveTunnel.Close()
	}
	m.ActiveTunnel = msg.Tunnel
	m.ActiveDriver = msg.Driver
	stored := msg.Connection
	m.CurrentConn = &stored

	m.initBrowse(msg.Driver)
	// config default; the connection's persisted query state overrides it when
	// QueryStateLoadedMsg lands
	if ac := m.Cmds.AppConfig(); ac != nil {
		m.SidebarHidden = ac.DisableSidebar
	}
	m.Screen = types.ScreenBrowse
	m.Focus = types.FocusTree
	m.StatusMsg = "Connected — " + msg.Connection.Name
	return m, tea.Batch(
		m.Cmds.LoadDatabases(msg.Driver),
		m.Cmds.LoadQueryState(msg.Connection.Name),
	)
}

func (m Model) handleTestResultMsg(msg types.TestResultMsg) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.Err != nil {
		m.TestResult = "Failed: " + formatConnectError(msg.Err)
		return m, nil
	}
	m.TestResult = "Connected in " + msg.Latency.Round(time.Millisecond).String()
	return m, nil
}

func (m Model) handleSSHTestMsg(msg types.SSHTestMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.SSHTunnelStatus = "SSH failed: " + formatConnectError(msg.Err)
		return m, nil
	}
	m.SSHTunnelStatus = "SSH OK"
	return m, nil
}
