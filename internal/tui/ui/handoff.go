package ui

import (
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jorgerojas26/lazysql/internal/tui/types"
	"github.com/jorgerojas26/lazysql/models"
)

// lazysqlLookPath resolves the lazysql browser binary. Seam for tests.
var lazysqlLookPath = func() (string, error) { return exec.LookPath("lazysql") }

// handoffToLazysql suspends lazytea and runs `lazysql <url>` attached to the
// terminal. lazytea (and the live SSH tunnel behind url) stays alive for the
// child's lifetime; the tunnel is torn down when lazysql exits.
func handoffToLazysql(conn models.Connection, url string) tea.Cmd {
	bin, err := lazysqlLookPath()
	if err != nil {
		return func() tea.Msg {
			return types.LazysqlExitedMsg{Err: fmt.Errorf("lazysql not found on PATH (run: make install-lazysql): %w", err)}
		}
	}
	var args []string
	if conn.ReadOnly {
		args = append(args, "--read-only")
	}
	args = append(args, url)
	return tea.ExecProcess(exec.Command(bin, args...), func(err error) tea.Msg {
		return types.LazysqlExitedMsg{Err: err}
	})
}

func (m Model) handleLazysqlExitedMsg(msg types.LazysqlExitedMsg) (tea.Model, tea.Cmd) {
	if m.ActiveTunnel != nil {
		_ = m.ActiveTunnel.Close()
		m.ActiveTunnel = nil
	}
	m.CurrentConn = nil
	m.Loading = false
	m.Screen = types.ScreenConnections
	if msg.Err != nil {
		m.ConnectionError = msg.Err.Error()
		m.StatusMsg = "Error: " + msg.Err.Error()
	} else {
		m.StatusMsg = "Returned from lazysql"
	}
	return m, nil
}
