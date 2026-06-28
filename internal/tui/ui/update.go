package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		if m.Screen == types.ScreenEditor {
			bw, bh := editorSize(msg.Width, msg.Height)
			m.EditorArea.SetWidth(bw)
			m.EditorArea.SetHeight(bh)
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case types.ConnectionsLoadedMsg:
		return m.handleConnectionsLoadedMsg(msg)
	case types.ConnectionSavedMsg:
		return m.handleConnectionSavedMsg(msg)
	case types.ConnectionDeletedMsg:
		return m.handleConnectionDeletedMsg(msg)
	case types.ConnectedMsg:
		return m.handleConnectedMsg(msg)
	case types.TestResultMsg:
		return m.handleTestResultMsg(msg)
	case types.SSHTestMsg:
		return m.handleSSHTestMsg(msg)
	case types.LazysqlExitedMsg:
		return m.handleLazysqlExitedMsg(msg)
	case types.DatabasesLoadedMsg:
		return m.handleDatabasesLoadedMsg(msg)
	case types.TablesLoadedMsg:
		return m.handleTablesLoadedMsg(msg)
	case types.RecordsLoadedMsg:
		return m.handleRecordsLoadedMsg(msg)
	case types.QueryExecutedMsg:
		return m.handleQueryExecutedMsg(msg)
	case types.ExportDoneMsg:
		return m.handleExportDoneMsg(msg)
	case types.PrimaryKeyLoadedMsg:
		return m.handlePrimaryKeyLoadedMsg(msg)
	case types.ChangesCommittedMsg:
		return m.handleChangesCommittedMsg(msg)
	case types.HistoryLoadedMsg:
		return m.handleHistoryLoadedMsg(msg)
	case types.MetaLoadedMsg:
		return m.handleMetaLoadedMsg(msg)
	case types.ForeignKeysLoadedMsg:
		return m.handleForeignKeysLoadedMsg(msg)
	case types.SavedQuerySavedMsg:
		return m.handleSavedQuerySavedMsg(msg)
	case types.SavedQueriesLoadedMsg:
		return m.handleSavedQueriesLoadedMsg(msg)
	}

	// forward unhandled msgs (e.g. the textarea cursor-blink tick) to the editor
	if m.Screen == types.ScreenEditor {
		return m.forwardToEditor(msg)
	}
	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		if m.Screen == types.ScreenConnections {
			return m, tea.Quit
		}
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
	case types.ScreenCellEdit:
		return m.handleCellEditScreen(msg)
	case types.ScreenHistory:
		return m.handleHistoryScreen(msg)
	case types.ScreenFilter:
		return m.handleFilterScreen(msg)
	case types.ScreenSetValue:
		return m.handleSetValueScreen(msg)
	case types.ScreenSaveQuery:
		return m.handleSaveQueryScreen(msg)
	case types.ScreenSavedQueries:
		return m.handleSavedQueriesScreen(msg)
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

	if msg.Browse {
		m.initBrowse(msg.Driver)
		m.Screen = types.ScreenBrowse
		m.Focus = types.FocusTree
		m.StatusMsg = "Connected — " + msg.Connection.Name
		return m, m.Cmds.LoadDatabases(msg.Driver)
	}

	m.StatusMsg = "Connected — opening lazysql..."
	return m, handoffToLazysql(msg.Connection, msg.URL)
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
