package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/internal/tui/commands"
	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/models"
)

func newTestModel() Model {
	return New(commands.New(nil))
}

func TestMoveConnection(t *testing.T) {
	m := newTestModel()
	m.Connections = []models.Connection{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	// move "a" (idx 0) down -> [b a c], selection follows to idx 1
	m.SelectedConnIdx = 0
	res, cmd := m.moveConnection(+1)
	m = res.(Model)
	if m.Connections[0].Name != "b" || m.Connections[1].Name != "a" {
		t.Fatalf("after move down: %v", names(m.Connections))
	}
	if m.SelectedConnIdx != 1 {
		t.Errorf("selection should follow the moved item to idx 1, got %d", m.SelectedConnIdx)
	}
	if cmd == nil {
		t.Error("a move should fire a persist command")
	}

	// move up at the top edge is a no-op (no reorder, no command)
	m.SelectedConnIdx = 0
	res, cmd = m.moveConnection(-1)
	if cmd != nil {
		t.Error("move up at the top edge should be a no-op")
	}
	if names(res.(Model).Connections) != "b,a,c" {
		t.Errorf("edge move should not reorder, got %v", names(res.(Model).Connections))
	}
}

func names(cs []models.Connection) string {
	out := ""
	for i, c := range cs {
		if i > 0 {
			out += ","
		}
		out += c.Name
	}
	return out
}

func TestTestConnectionScreen_ReturnsToOrigin(t *testing.T) {
	t.Run("from add form returns to add form, not the list", func(t *testing.T) {
		m := newTestModel()
		m.Screen = types.ScreenAddConnection
		res, _ := m.handleConnFormScreen(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
		m = res.(Model)
		if m.Screen != types.ScreenTestConnection {
			t.Fatalf("after ctrl+t Screen = %v, want TestConnection", m.Screen)
		}
		if m.TestReturnScreen != types.ScreenAddConnection {
			t.Fatalf("TestReturnScreen = %v, want AddConnection", m.TestReturnScreen)
		}
		res, _ = m.handleTestConnectionScreen(tea.KeyPressMsg{Code: tea.KeyEsc})
		if got := res.(Model).Screen; got != types.ScreenAddConnection {
			t.Errorf("after esc Screen = %v, want AddConnection (form, not list)", got)
		}
	})

	t.Run("from edit form returns to edit form", func(t *testing.T) {
		m := newTestModel()
		conn := models.Connection{Name: "a", URL: "postgres://x"}
		m.EditingConnection = &conn
		m.Screen = types.ScreenEditConnection
		res, _ := m.handleConnFormScreen(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
		res, _ = res.(Model).handleTestConnectionScreen(tea.KeyPressMsg{Code: tea.KeyEsc})
		if got := res.(Model).Screen; got != types.ScreenEditConnection {
			t.Errorf("after esc Screen = %v, want EditConnection", got)
		}
	})

	t.Run("from list returns to list", func(t *testing.T) {
		m := newTestModel()
		m.Connections = []models.Connection{{Name: "a", URL: "postgres://x"}}
		m.Screen = types.ScreenConnections
		res, _ := m.handleConnectionsScreen(keyMsg('t'))
		m = res.(Model)
		if m.Screen != types.ScreenTestConnection {
			t.Fatalf("after t Screen = %v, want TestConnection", m.Screen)
		}
		res, _ = m.handleTestConnectionScreen(tea.KeyPressMsg{Code: tea.KeyEsc})
		if got := res.(Model).Screen; got != types.ScreenConnections {
			t.Errorf("after esc Screen = %v, want Connections", got)
		}
	})
}

func TestConnInputIndex(t *testing.T) {
	const inputCount = 3
	tests := []struct {
		focus    int
		expected int
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, -1}, // ReadOnly toggle, no backing input
	}
	for _, tt := range tests {
		if got := connInputIndex(tt.focus, inputCount); got != tt.expected {
			t.Errorf("connInputIndex(%d) = %d, want %d", tt.focus, got, tt.expected)
		}
	}
}

func TestHandleAddConnectionScreen_FocusCycling(t *testing.T) {
	// Connection form has 4 focusable fields: Name, URL, Provider, ReadOnly toggle.
	t.Run("tab advances focus", func(t *testing.T) {
		m := newTestModel()
		result, _ := m.handleAddConnectionScreen(tea.KeyPressMsg{Code: tea.KeyTab})
		model := result.(Model)
		if model.ConnFocusIdx != 1 {
			t.Errorf("ConnFocusIdx = %d, want 1", model.ConnFocusIdx)
		}
	})
	t.Run("tab wraps past last field", func(t *testing.T) {
		m := newTestModel()
		m.ConnFocusIdx = m.connFieldCount() - 1 // ReadOnly toggle (idx 3)
		result, _ := m.handleAddConnectionScreen(tea.KeyPressMsg{Code: tea.KeyTab})
		model := result.(Model)
		if model.ConnFocusIdx != 0 {
			t.Errorf("ConnFocusIdx = %d, want 0 (wrap)", model.ConnFocusIdx)
		}
	})
	t.Run("shift+tab from first wraps to last", func(t *testing.T) {
		m := newTestModel()
		result, _ := m.handleAddConnectionScreen(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		model := result.(Model)
		if want := m.connFieldCount() - 1; model.ConnFocusIdx != want {
			t.Errorf("ConnFocusIdx = %d, want %d (wrap to ReadOnly toggle)", model.ConnFocusIdx, want)
		}
	})
	t.Run("focus maps to the right input being edited", func(t *testing.T) {
		m := newTestModel()
		// advance to URL field (focus idx 1)
		result, _ := m.handleAddConnectionScreen(tea.KeyPressMsg{Code: tea.KeyTab})
		m = result.(Model)
		if m.ConnFocusIdx != 1 {
			t.Fatalf("expected focus idx 1, got %d", m.ConnFocusIdx)
		}
		// a rune keystroke routes to the focused input (URL), not Name/Provider
		result, _ = m.handleAddConnectionScreen(keyMsg('x'))
		m = result.(Model)
		if m.ConnInputs[0].Value() != "" {
			t.Errorf("Name input mutated, value = %q", m.ConnInputs[0].Value())
		}
		if m.ConnInputs[1].Value() != "x" {
			t.Errorf("URL input = %q, want x", m.ConnInputs[1].Value())
		}
		if m.ConnInputs[2].Value() != "" {
			t.Errorf("Provider input mutated, value = %q", m.ConnInputs[2].Value())
		}
	})
	t.Run("space toggles ReadOnly when focused on toggle", func(t *testing.T) {
		m := newTestModel()
		m.ConnFocusIdx = m.connReadOnlyFocusIdx()
		result, _ := m.handleAddConnectionScreen(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
		model := result.(Model)
		if !model.ConnReadOnly {
			t.Error("ReadOnly = false, want true after space on toggle")
		}
	})
}

func TestConvertCurrentInputsToConnection_Add(t *testing.T) {
	m := newTestModel()
	m.ConnInputs[0].SetValue("local-mysql")
	m.ConnInputs[1].SetValue("mysql://root:pw@127.0.0.1:3306/app")
	m.ConnInputs[2].SetValue("") // blank provider -> inferred from URL
	m.ConnReadOnly = true

	got := m.convertCurrentInputsToConnection(m.ConnInputs, ActionAdd)

	if got.Name != "local-mysql" {
		t.Errorf("Name = %q, want local-mysql", got.Name)
	}
	if got.URL != "mysql://root:pw@127.0.0.1:3306/app" {
		t.Errorf("URL = %q, want the mysql URL", got.URL)
	}
	if got.Provider != "mysql" {
		t.Errorf("Provider = %q, want mysql (inferred from URL)", got.Provider)
	}
	if !got.ReadOnly {
		t.Error("ReadOnly = false, want true")
	}
	if got.UseSSH {
		t.Error("UseSSH = true, want false (SSH not enabled)")
	}
}

func TestConvertCurrentInputsToConnection_ExplicitProviderWins(t *testing.T) {
	m := newTestModel()
	m.ConnInputs[0].SetValue("pg")
	m.ConnInputs[1].SetValue("postgres://u:p@h:5432/d")
	m.ConnInputs[2].SetValue("postgres")

	got := m.convertCurrentInputsToConnection(m.ConnInputs, ActionAdd)
	if got.Provider != "postgres" {
		t.Errorf("Provider = %q, want postgres", got.Provider)
	}
}

func TestConvertCurrentInputsToConnection_EditCarriesNonFormFields(t *testing.T) {
	m := newTestModel()
	src := models.Connection{
		Name:      "prod",
		Username:  "admin",
		Password:  "hunter2",
		Hostname:  "db.internal",
		Port:      "5432",
		DBName:    "app",
		URLParams: "?sslmode=require",
		Schemas:   []string{"public", "audit"},
	}
	m.EditingConnection = &src
	m.ConnInputs[0].SetValue("prod")
	m.ConnInputs[1].SetValue("postgres://admin:hunter2@db.internal:5432/app")
	m.ConnInputs[2].SetValue("postgres")

	got := m.convertCurrentInputsToConnection(m.ConnInputs, ActionEdit)
	if got.Username != "admin" || got.Password != "hunter2" {
		t.Errorf("Username/Password not carried: %q/%q", got.Username, got.Password)
	}
	if got.Hostname != "db.internal" || got.Port != "5432" || got.DBName != "app" {
		t.Errorf("Hostname/Port/DBName not carried: %q/%q/%q", got.Hostname, got.Port, got.DBName)
	}
	if got.URLParams != "?sslmode=require" {
		t.Errorf("URLParams = %q, want carried", got.URLParams)
	}
	if len(got.Schemas) != 2 {
		t.Errorf("Schemas = %v, want carried", got.Schemas)
	}
}

func TestSSHStaging_ConvertSSHInputsAndFold(t *testing.T) {
	m := newTestModel()
	// edit the SSH textinputs (0 host, 1 port, 2 user, 3 key, 4 passphrase, 5 password)
	m.SSHInputs[0].SetValue("bastion.example.com")
	m.SSHInputs[1].SetValue("2200")
	m.SSHInputs[2].SetValue("deploy")
	m.SSHInputs[3].SetValue("~/.ssh/id_ed25519")
	m.SSHInputs[4].SetValue("phrase")
	m.SSHInputs[5].SetValue("pw")
	m.SSHFocusIdx = 0 // on an input field; Enter stages rather than toggling
	m.SSHEnabled = true
	m.EditingConnection = &models.Connection{Name: "prod"}

	// Enter materializes PendingSSH and returns to the edit screen.
	result, _ := m.handleSSHTunnelScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := result.(Model)

	if model.Screen != types.ScreenEditConnection {
		t.Errorf("Screen = %v, want ScreenEditConnection", model.Screen)
	}
	if model.PendingSSH == nil {
		t.Fatal("PendingSSH = nil, want staged connection")
	}
	if model.PendingSSH.SSHHost != "bastion.example.com" {
		t.Errorf("PendingSSH.SSHHost = %q", model.PendingSSH.SSHHost)
	}
	if model.PendingSSH.SSHPort != "2200" {
		t.Errorf("PendingSSH.SSHPort = %q, want 2200", model.PendingSSH.SSHPort)
	}
	if model.PendingSSH.SSHUser != "deploy" {
		t.Errorf("PendingSSH.SSHUser = %q", model.PendingSSH.SSHUser)
	}
	if model.PendingSSH.SSHKeyFile != "~/.ssh/id_ed25519" {
		t.Errorf("PendingSSH.SSHKeyFile = %q", model.PendingSSH.SSHKeyFile)
	}
	if model.PendingSSH.SSHPassphrase != "phrase" {
		t.Errorf("PendingSSH.SSHPassphrase = %q", model.PendingSSH.SSHPassphrase)
	}
	if model.PendingSSH.SSHPassword != "pw" {
		t.Errorf("PendingSSH.SSHPassword = %q", model.PendingSSH.SSHPassword)
	}

	// Folding PendingSSH into the saved connection populates the ssh_* fields.
	model.ConnInputs[0].SetValue("prod")
	model.ConnInputs[1].SetValue("postgres://u:p@db:5432/app")
	model.ConnInputs[2].SetValue("postgres")
	conn := model.convertCurrentInputsToConnection(model.ConnInputs, ActionEdit)
	if !conn.UseSSH {
		t.Error("UseSSH = false, want true (SSHEnabled was set)")
	}
	if conn.SSHHost != "bastion.example.com" {
		t.Errorf("folded SSHHost = %q", conn.SSHHost)
	}
	if conn.SSHPort != "2200" {
		t.Errorf("folded SSHPort = %q, want 2200", conn.SSHPort)
	}
	if conn.SSHUser != "deploy" {
		t.Errorf("folded SSHUser = %q", conn.SSHUser)
	}
	if conn.SSHPassphrase != "phrase" || conn.SSHPassword != "pw" {
		t.Errorf("folded secrets = %q/%q", conn.SSHPassphrase, conn.SSHPassword)
	}
}

func TestSSHStaging_EmptyHostClearsPending(t *testing.T) {
	m := newTestModel()
	m.SSHEnabled = true
	m.PendingSSH = &models.Connection{SSHHost: "old"}
	// no host set in inputs (reset leaves host blank)
	result, _ := m.handleSSHTunnelScreen(tea.KeyPressMsg{Code: tea.KeyEnter})
	model := result.(Model)
	if model.PendingSSH != nil {
		t.Errorf("PendingSSH = %v, want nil when host empty", model.PendingSSH)
	}
	if model.SSHEnabled {
		t.Error("SSHEnabled = true, want false when host cleared")
	}
}

func TestSSHStaging_DefaultPortWhenBlank(t *testing.T) {
	m := newTestModel()
	m.SSHInputs[0].SetValue("bastion")
	m.SSHInputs[1].SetValue("") // blank -> defaults to 22
	staged := m.convertSSHInputs()
	if staged == nil {
		t.Fatal("convertSSHInputs returned nil for non-empty host")
	}
	if staged.SSHPort != "22" {
		t.Errorf("SSHPort = %q, want 22", staged.SSHPort)
	}
}

// keyMsg constructs a key-press message for a single rune.
func keyMsg(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}
