package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/helpers"
	"github.com/bearded-giant/cellar/internal/tui/commands"
	"github.com/bearded-giant/cellar/internal/tui/sqlmeta"
	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/models"
)

type Model struct {
	Cmds    *commands.Commands
	Version string
	Screen  types.Screen

	// TestReturnScreen is the screen to restore when leaving the test-result
	// screen (the list, or the add/edit form the test was launched from).
	TestReturnScreen types.Screen

	Connections     []models.Connection
	SelectedConnIdx int

	ConnInputs        []textinput.Model
	ConnFocusIdx      int
	ConnReadOnly      bool
	EditingConnection *models.Connection
	DuplicatingFrom   *models.Connection
	CurrentConn       *models.Connection
	ActiveTunnel      *helpers.Tunnel
	ActiveDriver      drivers.Driver

	SSHInputs       []textinput.Model
	SSHFocusIdx     int
	SSHEnabled      bool
	PendingSSH      *models.Connection
	SSHTunnelStatus string

	ConfirmType string
	ConfirmData any
	// ConfirmReturnScreen is where the confirm dialog returns on cancel/finish.
	ConfirmReturnScreen types.Screen
	// HelpReturnScreen is the screen to restore when closing the help overlay.
	HelpReturnScreen types.Screen

	Focus  types.Focus
	Browse browseState // live state of the active browse tab

	// Tabs are the per-tab browse snapshots; m.Browse mirrors Tabs[TabActive].
	Tabs      []browseState
	TabActive int

	// SQL editor + autocomplete. EditorContent persists the last query across
	// opens.
	EditorArea    sqlEditor
	EditorContent string
	Completer     *sqlmeta.Autocompleter
	Completions   []sqlmeta.CompletionItem
	CompCursor    int
	CompVisible   bool

	ExportInput     textinput.Model
	CellInput       textinput.Model
	FilterInput     textinput.Model
	TreeFilterInput textinput.Model
	ExportAll       bool // export modal scope: all rows vs current page

	HistoryItems  []models.QueryHistoryItem
	HistoryCursor int

	SaveNameInput textinput.Model
	SavedItems    []models.SavedQuery
	SavedCursor   int

	PreviewChanges []models.DBDMLChange
	PreviewSQL     []string
	PreviewCursor  int

	CellViewLines  []string
	CellViewScroll int
	CellViewCol    string

	Width  int
	Height int

	Err             error
	StatusMsg       string
	Loading         bool
	ConnectionError string
	TestResult      string

	SendFunc *func(tea.Msg)
}

type ActionType string

const (
	ActionAdd  ActionType = "add"
	ActionEdit ActionType = "edit"
	ActionTest ActionType = "test"
)

func New(cmds *commands.Commands) Model {
	return Model{
		Cmds:        cmds,
		Screen:      types.ScreenConnections,
		Connections: []models.Connection{},
		ConnInputs:  createConnectionInputs(),
		SSHInputs:   createSSHInputs(),
	}
}

func createConnectionInputs() []textinput.Model {
	inputs := make([]textinput.Model, 3)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Connection Name"
	inputs[0].Focus()
	inputs[0].Width = 50

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "URL (e.g. mysql://user:pass@host:3306/db)"
	inputs[1].Width = 50

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "Provider (mysql/postgres/sqlite3/sqlserver)"
	inputs[2].Width = 50

	return inputs
}

// SSH form inputs:
// 0 host, 1 port (22), 2 user, 3 key path, 4 passphrase, 5 password, 6 proxy command.
func createSSHInputs() []textinput.Model {
	inputs := make([]textinput.Model, 7)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Bastion Host"
	inputs[0].Width = 50
	inputs[0].Focus()

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "Bastion Port"
	inputs[1].Width = 50
	inputs[1].SetValue("22")

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "SSH User"
	inputs[2].Width = 50

	inputs[3] = textinput.New()
	inputs[3].Placeholder = "Private Key Path (optional)"
	inputs[3].Width = 50

	inputs[4] = textinput.New()
	inputs[4].Placeholder = "Passphrase (optional)"
	inputs[4].Width = 50
	inputs[4].EchoMode = textinput.EchoPassword

	inputs[5] = textinput.New()
	inputs[5].Placeholder = "SSH Password (optional)"
	inputs[5].Width = 50
	inputs[5].EchoMode = textinput.EchoPassword

	inputs[6] = textinput.New()
	inputs[6].Placeholder = `Proxy command, e.g. sh -c "aws ssm start-session ..." (optional)`
	inputs[6].Width = 50

	return inputs
}

func (m Model) Init() tea.Cmd {
	return m.Cmds.LoadConnections()
}

// connFieldCount returns the number of focusable fields in the connection form:
// Name, URL, Provider, ReadOnly toggle.
func (m Model) connFieldCount() int {
	return len(m.ConnInputs) + 1
}

// connReadOnlyFocusIdx is the focus index of the ReadOnly toggle (last field).
func (m Model) connReadOnlyFocusIdx() int {
	return len(m.ConnInputs)
}

func (m *Model) resetConnInputs() {
	for i := range m.ConnInputs {
		m.ConnInputs[i].SetValue("")
		m.ConnInputs[i].Blur()
	}
	m.ConnInputs[0].Focus()
	m.ConnFocusIdx = 0
	m.ConnReadOnly = false
	m.SSHEnabled = false
	m.PendingSSH = nil
	m.resetSSHInputs()
}

func (m *Model) populateConnInputs(conn models.Connection) {
	m.ConnInputs[0].SetValue(conn.Name)
	m.ConnInputs[1].SetValue(conn.URL)
	m.ConnInputs[2].SetValue(conn.Provider)
	m.ConnFocusIdx = 0
	m.ConnReadOnly = conn.ReadOnly
	m.SSHEnabled = conn.UseSSH
	if conn.UseSSH || conn.SSHHost != "" {
		staged := conn
		m.PendingSSH = &staged
	} else {
		m.PendingSSH = nil
	}
	for i := range m.ConnInputs {
		m.ConnInputs[i].Blur()
	}
	m.ConnInputs[0].Focus()
}

// convertCurrentInputsToConnection builds a models.Connection from the current
// form inputs. The edit path keeps immutable fields off the source via
// EditingConnection; the duplicate path carries them forward from DuplicatingFrom.
func (m *Model) convertCurrentInputsToConnection(inputs []textinput.Model, action ActionType) models.Connection {
	url := strings.TrimSpace(inputs[1].Value())
	provider := strings.TrimSpace(inputs[2].Value())
	dbName := ""
	if parsed, err := helpers.ParseConnectionString(url); err == nil {
		if provider == "" {
			provider = parsed.Driver
		}
		dbName = strings.Split(parsed.Normalize(",", "NULL", 0), ",")[3]
		if dbName == "NULL" {
			dbName = ""
		}
	}

	conn := models.Connection{
		Name:     inputs[0].Value(),
		URL:      url,
		Provider: provider,
		DBName:   dbName,
		ReadOnly: m.ConnReadOnly,
		UseSSH:   m.SSHEnabled,
	}

	if m.PendingSSH != nil {
		conn.SSHHost = m.PendingSSH.SSHHost
		conn.SSHPort = m.PendingSSH.SSHPort
		conn.SSHUser = m.PendingSSH.SSHUser
		conn.SSHKeyFile = m.PendingSSH.SSHKeyFile
		conn.SSHPassphrase = m.PendingSSH.SSHPassphrase
		conn.SSHPassword = m.PendingSSH.SSHPassword
		conn.SSHProxyCommand = m.PendingSSH.SSHProxyCommand
	}
	// Match the tview app: never persist the display default "22" (the tunnel
	// defaults to 22 when unset), and drop the port when SSH is off, so the
	// shared config.toml stays byte-identical across both writers.
	if !conn.UseSSH || conn.SSHPort == "22" {
		conn.SSHPort = ""
	}

	// Carry forward fields with no form UI.
	switch {
	case action == ActionEdit && m.EditingConnection != nil:
		src := m.EditingConnection
		conn.Username = src.Username
		conn.Password = src.Password
		conn.Hostname = src.Hostname
		conn.Port = src.Port
		conn.URLParams = src.URLParams
		conn.Schemas = src.Schemas
		conn.Commands = src.Commands
	case action == ActionAdd && m.DuplicatingFrom != nil:
		src := m.DuplicatingFrom
		conn.Username = src.Username
		conn.Password = src.Password
		conn.Hostname = src.Hostname
		conn.Port = src.Port
		conn.URLParams = src.URLParams
		conn.Schemas = src.Schemas
		conn.Commands = src.Commands
	}

	return conn
}

func (m *Model) resetSSHInputs() {
	for i := range m.SSHInputs {
		m.SSHInputs[i].SetValue("")
		m.SSHInputs[i].Blur()
	}
	m.SSHInputs[1].SetValue("22")
	m.SSHInputs[0].Focus()
	m.SSHFocusIdx = 0
	m.SSHTunnelStatus = ""
}

// populateSSHInputs loads the staged SSH fields (flat on models.Connection) into
// the SSH form inputs. A nil staged connection clears the form.
func (m *Model) populateSSHInputs(staged *models.Connection) {
	m.resetSSHInputs()
	if staged == nil {
		return
	}
	m.SSHInputs[0].SetValue(staged.SSHHost)
	if staged.SSHPort != "" {
		m.SSHInputs[1].SetValue(staged.SSHPort)
	}
	m.SSHInputs[2].SetValue(staged.SSHUser)
	m.SSHInputs[3].SetValue(staged.SSHKeyFile)
	m.SSHInputs[4].SetValue(staged.SSHPassphrase)
	m.SSHInputs[5].SetValue(staged.SSHPassword)
	m.SSHInputs[6].SetValue(staged.SSHProxyCommand)
}

// convertSSHInputs stages the SSH form into a models.Connection holding only the
// flat SSH fields. Returns nil if Host is empty (treated as "not configured").
func (m *Model) convertSSHInputs() *models.Connection {
	host := m.SSHInputs[0].Value()
	if host == "" {
		return nil
	}
	port := m.SSHInputs[1].Value()
	if port == "" {
		port = "22"
	}
	return &models.Connection{
		SSHHost:         host,
		SSHPort:         port,
		SSHUser:         m.SSHInputs[2].Value(),
		SSHKeyFile:      m.SSHInputs[3].Value(),
		SSHPassphrase:   m.SSHInputs[4].Value(),
		SSHPassword:     m.SSHInputs[5].Value(),
		SSHProxyCommand: m.SSHInputs[6].Value(),
	}
}
