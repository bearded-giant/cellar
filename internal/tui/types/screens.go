package types

type Screen int

const (
	ScreenConnections Screen = iota
	ScreenAddConnection
	ScreenEditConnection
	ScreenSSHTunnel
	ScreenTestConnection
	ScreenConfirmDelete
	ScreenBrowse
)

func (s Screen) String() string {
	names := map[Screen]string{
		ScreenConnections:    "Connections",
		ScreenAddConnection:  "Add Connection",
		ScreenEditConnection: "Edit Connection",
		ScreenSSHTunnel:      "SSH Tunnel",
		ScreenTestConnection: "Test Connection",
		ScreenConfirmDelete:  "Confirm Delete",
		ScreenBrowse:         "Browse",
	}
	if name, ok := names[s]; ok {
		return name
	}
	return "Unknown"
}
