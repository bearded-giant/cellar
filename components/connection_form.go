package components

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/models"
)

type ConnectionForm struct {
	*tview.Flex
	*tview.Form
	StatusText *tview.TextView
	Action     string
}

func NewConnectionForm(connectionPages *models.ConnectionPages) *ConnectionForm {
	wrapper := tview.NewFlex()

	wrapper.SetDirection(tview.FlexColumnCSS)

	addForm := tview.NewForm().SetFieldBackgroundColor(app.Styles.InverseTextColor).SetButtonBackgroundColor(tview.Styles.InverseTextColor).SetLabelColor(tview.Styles.PrimaryTextColor).SetFieldTextColor(tview.Styles.ContrastSecondaryTextColor)
	addForm.AddInputField("Name", "", 0, nil, nil)
	addForm.AddInputField("URL", "", 0, nil, nil)
	addForm.AddCheckbox("Read-Only", false, nil)
	addForm.AddCheckbox("Use SSH Tunnel", false, nil)
	addForm.AddInputField("SSH Host", "", 0, nil, nil)
	addForm.AddInputField("SSH Port", "22", 0, nil, nil)
	addForm.AddInputField("SSH User", "", 0, nil, nil)
	addForm.AddInputField("SSH Key File", "", 0, nil, nil)
	addForm.AddPasswordField("SSH Passphrase", "", 0, '*', nil)
	addForm.AddPasswordField("SSH Password", "", 0, '*', nil)

	buttonsWrapper := tview.NewFlex().SetDirection(tview.FlexColumn)

	saveButton := tview.NewButton("[yellow]F1 [dark]Save")
	saveButton.SetStyle(tcell.StyleDefault.Background(app.Styles.PrimaryTextColor))
	saveButton.SetBorder(true)

	buttonsWrapper.AddItem(saveButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	testButton := tview.NewButton("[yellow]F2 [dark]Test")
	testButton.SetStyle(tcell.StyleDefault.Background(app.Styles.PrimaryTextColor))
	testButton.SetBorder(true)

	buttonsWrapper.AddItem(testButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	connectButton := tview.NewButton("[yellow]F3 [dark]Connect")
	connectButton.SetStyle(tcell.StyleDefault.Background(app.Styles.PrimaryTextColor))
	connectButton.SetBorder(true)

	buttonsWrapper.AddItem(connectButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	cancelButton := tview.NewButton("[yellow]Esc [dark]Cancel")
	cancelButton.SetStyle(tcell.StyleDefault.Background(tcell.Color(app.Styles.PrimaryTextColor)))
	cancelButton.SetBorder(true)

	buttonsWrapper.AddItem(cancelButton, 0, 1, false)

	statusText := tview.NewTextView()
	statusText.SetBorderPadding(1, 1, 0, 0)

	wrapper.AddItem(addForm, 0, 1, true)
	wrapper.AddItem(statusText, 4, 0, false)
	wrapper.AddItem(buttonsWrapper, 3, 0, false)

	form := &ConnectionForm{
		Flex:       wrapper,
		Form:       addForm,
		StatusText: statusText,
	}

	wrapper.SetInputCapture(form.inputCapture(connectionPages))

	return form
}

func (form *ConnectionForm) inputCapture(connectionPages *models.ConnectionPages) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			connectionPages.SwitchToPage(pageNameConnectionSelection)
		} else if event.Key() == tcell.KeyF1 || event.Key() == tcell.KeyEnter {
			parsedDatabaseData := form.readConnection()

			if parsedDatabaseData.Name == "" {
				form.StatusText.SetText("Connection name is required").SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
				return event
			}

			parsed, err := helpers.ParseConnectionString(parsedDatabaseData.URL)
			if err != nil {
				form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
				return event
			}

			databases := app.App.Connections()
			newDatabases := make([]models.Connection, len(databases))

			DBName := strings.Split(parsed.Normalize(",", "NULL", 0), ",")[3]

			if DBName == "NULL" {
				DBName = ""
			}

			parsedDatabaseData.Provider = parsed.Driver
			parsedDatabaseData.DBName = DBName

			switch form.Action {
			case actionNewConnection:

				newDatabases = append(databases, parsedDatabaseData)
				err := app.App.SaveConnections(newDatabases)
				if err != nil {
					form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
					return event
				}

			case actionEditConnection:
				newDatabases = make([]models.Connection, len(databases))
				row, _ := connectionsTable.GetSelection()

				for i, database := range databases {
					if i == row {
						newDatabases[i] = parsedDatabaseData

						// newDatabases[i].Name = connectionName
						// newDatabases[i].Provider = database.Provider
						// newDatabases[i].User = parsed.User.Username()
						// newDatabases[i].Password, _ = parsed.User.Password()
						// newDatabases[i].Host = parsed.Hostname()
						// newDatabases[i].Port = parsed.Port()
						// newDatabases[i].Query = parsed.Query().Encode()
						// newDatabases[i].DBName = helpers.ParsedDBName(parsed.Path)
						// newDatabases[i].DSN = parsed.DSN
					} else {
						newDatabases[i] = database
					}
				}

				err := app.App.SaveConnections(newDatabases)
				if err != nil {
					form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
					return event

				}
			}

			connectionsTable.SetConnections(newDatabases)
			connectionPages.SwitchToPage(pageNameConnectionSelection)

		} else if event.Key() == tcell.KeyF2 {
			go form.testConnection(form.readConnection())
		}
		return event
	}
}

func (form *ConnectionForm) testConnection(connection models.Connection) {
	parsed, err := helpers.ParseConnectionString(connection.URL)
	if err != nil {
		form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
		return
	}

	form.StatusText.SetText("Connecting...").SetTextColor(app.Styles.TertiaryTextColor)

	urlstr := connection.URL
	if connection.UseSSH {
		sshCfg, err := sshConfigFromConnection(connection)
		if err != nil {
			form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
			return
		}
		rewritten, tunnel, err := helpers.OpenTunnelForURL(app.App.Context(), sshCfg, connection.URL, defaultDBPort(parsed.Driver))
		if err != nil {
			form.StatusText.SetText("SSH tunnel: " + err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
			return
		}
		defer tunnel.Close()
		urlstr = rewritten
	}

	var db drivers.Driver

	switch parsed.Driver {
	case drivers.DriverMySQL:
		db = &drivers.MySQL{}
	case drivers.DriverPostgres:
		db = &drivers.Postgres{}
	case drivers.DriverSqlite:
		db = &drivers.SQLite{}
	case drivers.DriverMSSQL:
		db = &drivers.MSSQL{}
	default:
		form.StatusText.SetText("Unsupported database provider: " + parsed.Driver).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
		return
	}

	err = db.TestConnection(urlstr)

	if err != nil {
		form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
	} else {
		form.StatusText.SetText("Connection success").SetTextColor(app.Styles.TertiaryTextColor)
	}
	App.ForceDraw()
}

func (form *ConnectionForm) SetAction(action string) {
	form.Action = action
}

// readConnection pulls every form field into a models.Connection. Name/URL/
// Read-Only are read positionally (legacy), SSH fields by label.
func (form *ConnectionForm) readConnection() models.Connection {
	useSSH := form.GetFormItemByLabel("Use SSH Tunnel").(*tview.Checkbox).IsChecked()

	// Don't persist the display default "22" (tunnel defaults to 22 when unset),
	// and drop the port entirely when SSH is off, so non-SSH configs stay clean.
	sshPort := form.GetFormItemByLabel("SSH Port").(*tview.InputField).GetText()
	if !useSSH || sshPort == "22" {
		sshPort = ""
	}

	return models.Connection{
		Name:          form.GetFormItem(0).(*tview.InputField).GetText(),
		URL:           form.GetFormItem(1).(*tview.InputField).GetText(),
		ReadOnly:      form.GetFormItem(2).(*tview.Checkbox).IsChecked(),
		UseSSH:        useSSH,
		SSHHost:       form.GetFormItemByLabel("SSH Host").(*tview.InputField).GetText(),
		SSHPort:       sshPort,
		SSHUser:       form.GetFormItemByLabel("SSH User").(*tview.InputField).GetText(),
		SSHKeyFile:    form.GetFormItemByLabel("SSH Key File").(*tview.InputField).GetText(),
		SSHPassphrase: form.GetFormItemByLabel("SSH Passphrase").(*tview.InputField).GetText(),
		SSHPassword:   form.GetFormItemByLabel("SSH Password").(*tview.InputField).GetText(),
	}
}

// ResetFields clears every field to its default (used by the New action).
func (form *ConnectionForm) ResetFields() {
	form.GetFormItem(0).(*tview.InputField).SetText("")
	form.GetFormItem(1).(*tview.InputField).SetText("")
	form.GetFormItem(2).(*tview.Checkbox).SetChecked(false)
	form.GetFormItemByLabel("Use SSH Tunnel").(*tview.Checkbox).SetChecked(false)
	form.GetFormItemByLabel("SSH Host").(*tview.InputField).SetText("")
	form.GetFormItemByLabel("SSH Port").(*tview.InputField).SetText("22")
	form.GetFormItemByLabel("SSH User").(*tview.InputField).SetText("")
	form.GetFormItemByLabel("SSH Key File").(*tview.InputField).SetText("")
	form.GetFormItemByLabel("SSH Passphrase").(*tview.InputField).SetText("")
	form.GetFormItemByLabel("SSH Password").(*tview.InputField).SetText("")
}

func (form *ConnectionForm) SetConnectionData(conn models.Connection) {
	form.GetFormItem(0).(*tview.InputField).SetText(conn.Name)
	form.GetFormItem(1).(*tview.InputField).SetText(conn.URL)
	form.GetFormItem(2).(*tview.Checkbox).SetChecked(conn.ReadOnly)
	form.GetFormItemByLabel("Use SSH Tunnel").(*tview.Checkbox).SetChecked(conn.UseSSH)
	form.GetFormItemByLabel("SSH Host").(*tview.InputField).SetText(conn.SSHHost)
	sshPort := conn.SSHPort
	if sshPort == "" {
		sshPort = "22"
	}
	form.GetFormItemByLabel("SSH Port").(*tview.InputField).SetText(sshPort)
	form.GetFormItemByLabel("SSH User").(*tview.InputField).SetText(conn.SSHUser)
	form.GetFormItemByLabel("SSH Key File").(*tview.InputField).SetText(conn.SSHKeyFile)
	form.GetFormItemByLabel("SSH Passphrase").(*tview.InputField).SetText(conn.SSHPassphrase)
	form.GetFormItemByLabel("SSH Password").(*tview.InputField).SetText(conn.SSHPassword)
}
