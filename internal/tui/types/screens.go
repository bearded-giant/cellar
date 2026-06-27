package types

type Screen int

const (
	ScreenConnections Screen = iota
	ScreenAddConnection
	ScreenEditConnection
	ScreenSSHTunnel
	ScreenTestConnection
	ScreenConfirmDelete
)

func (s Screen) String() string {
	names := map[Screen]string{
		ScreenConnections:    "Connections",
		ScreenAddConnection:  "Add Connection",
		ScreenEditConnection: "Edit Connection",
		ScreenSSHTunnel:      "SSH Tunnel",
		ScreenTestConnection: "Test Connection",
		ScreenConfirmDelete:  "Confirm Delete",
	}
	if name, ok := names[s]; ok {
		return name
	}
	return "Unknown"
}
