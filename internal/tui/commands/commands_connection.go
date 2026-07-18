package commands

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bearded-giant/cellar/drivers"
	"github.com/bearded-giant/cellar/helpers"
	"github.com/bearded-giant/cellar/internal/tui/types"
	"github.com/bearded-giant/cellar/models"
)

const vaultResolveTimeout = 30 * time.Second

// resolveVaultURL runs conn.VaultCommand and returns its trimmed stdout as the
// URL to dial. Empty command returns conn.URL unchanged. The command's stderr
// rides along on failure so the user sees why the vault fetch broke.
func resolveVaultURL(ctx context.Context, conn models.Connection) (string, error) {
	cmdStr := strings.TrimSpace(conn.VaultCommand)
	if cmdStr == "" {
		return conn.URL, nil
	}

	parts := strings.Fields(cmdStr)

	ctx, cancel := context.WithTimeout(ctx, vaultResolveTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, parts[0], parts[1:]...).Output() // #nosec G204
	if err != nil {
		detail := err.Error()
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return "", fmt.Errorf("vault: %s", detail)
	}

	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", fmt.Errorf("vault: command produced no output")
	}
	return url, nil
}

func sshConfigFromConnection(conn models.Connection) (*helpers.SSHConfig, error) {
	port := 0
	if conn.SSHPort != "" {
		p, err := strconv.Atoi(conn.SSHPort)
		if err != nil {
			return nil, fmt.Errorf("invalid SSH port %q: %w", conn.SSHPort, err)
		}
		port = p
	}
	return &helpers.SSHConfig{
		Host:           conn.SSHHost,
		Port:           port,
		User:           conn.SSHUser,
		Password:       conn.SSHPassword,
		PrivateKeyPath: conn.SSHKeyFile,
		Passphrase:     conn.SSHPassphrase,
		ProxyCommand:   conn.SSHProxyCommand,
	}, nil
}

func defaultDBPort(provider string) string {
	switch provider {
	case drivers.DriverPostgres:
		return "5432"
	default:
		return "3306"
	}
}

func (c *Commands) LoadConnections() tea.Cmd {
	return func() tea.Msg {
		if c.cfg == nil {
			return types.ConnectionsLoadedMsg{}
		}
		return types.ConnectionsLoadedMsg{Connections: c.cfg.Connections}
	}
}

func (c *Commands) SaveConnection(conn models.Connection, existing []models.Connection, isEdit bool, originalName string) tea.Cmd {
	return func() tea.Msg {
		next := make([]models.Connection, 0, len(existing)+1)
		if isEdit {
			for _, e := range existing {
				if e.Name == originalName {
					next = append(next, conn)
				} else {
					next = append(next, e)
				}
			}
		} else {
			next = append(next, existing...)
			next = append(next, conn)
		}
		if err := c.saveConnections(next); err != nil {
			return types.ConnectionSavedMsg{Connection: conn, IsEdit: isEdit, Err: err}
		}
		return types.ConnectionSavedMsg{Connection: conn, IsEdit: isEdit}
	}
}

func (c *Commands) DeleteConnection(name string, existing []models.Connection) tea.Cmd {
	return func() tea.Msg {
		next := make([]models.Connection, 0, len(existing))
		for _, e := range existing {
			if e.Name != name {
				next = append(next, e)
			}
		}
		if err := c.saveConnections(next); err != nil {
			return types.ConnectionDeletedMsg{Name: name, Err: err}
		}
		return types.ConnectionDeletedMsg{Name: name}
	}
}

// ReorderConnections persists connections in the given order (the list is
// loaded and displayed in file order).
func (c *Commands) ReorderConnections(conns []models.Connection) tea.Cmd {
	return func() tea.Msg {
		return types.ConnectionsReorderedMsg{Err: c.saveConnections(conns)}
	}
}

func (c *Commands) Connect(conn models.Connection) tea.Cmd {
	return func() tea.Msg {
		url, tunnel, err := c.openDial(context.Background(), conn)
		if err != nil {
			return types.ConnectedMsg{Connection: conn, Err: err}
		}

		driver := c.DriverFor(conn.Provider)
		if driver == nil {
			if tunnel != nil {
				_ = tunnel.Close()
			}
			return types.ConnectedMsg{
				Connection: conn,
				Err:        fmt.Errorf("unsupported database provider %q (valid: mysql, postgres, sqlite3)", conn.Provider),
			}
		}

		if err := driver.Connect(url); err != nil {
			if tunnel != nil {
				_ = tunnel.Close()
			}
			return types.ConnectedMsg{Connection: conn, Err: err}
		}

		// keep the tunnel + driver open for the connection's lifetime; the UI closes
		// the tunnel on disconnect/quit. Closing it here would kill the live pool.
		return types.ConnectedMsg{Connection: conn, URL: url, Tunnel: tunnel, Driver: driver}
	}
}

func (c *Commands) TestConnection(conn models.Connection) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		url, tunnel, err := c.openDial(context.Background(), conn)
		if err != nil {
			return types.TestResultMsg{Err: err}
		}
		defer func() {
			if tunnel != nil {
				_ = tunnel.Close()
			}
		}()

		driver := c.DriverFor(conn.Provider)
		if driver == nil {
			return types.TestResultMsg{
				Err: fmt.Errorf("unsupported database provider %q (valid: mysql, postgres, sqlite3)", conn.Provider),
			}
		}

		if err := driver.TestConnection(url); err != nil {
			return types.TestResultMsg{Err: err}
		}
		return types.TestResultMsg{Success: true, Latency: time.Since(start)}
	}
}

func (c *Commands) TestSSH(conn models.Connection) tea.Cmd {
	return func() tea.Msg {
		sshCfg, err := sshConfigFromConnection(conn)
		if err != nil {
			return types.SSHTestMsg{Err: err}
		}
		remoteAddr := conn.SSHHost
		if remoteAddr == "" {
			return types.SSHTestMsg{Err: fmt.Errorf("SSH host is required")}
		}
		// Dial the bastion and forward to itself to prove reachability + auth,
		// then close immediately.
		tunnel, err := helpers.OpenSSHTunnel(context.Background(), sshCfg, sshCfg.Host+":"+defaultDBPort(conn.Provider))
		if err != nil {
			return types.SSHTestMsg{Err: err}
		}
		_ = tunnel.Close()
		return types.SSHTestMsg{}
	}
}

// openDial opens the SSH tunnel when the connection uses SSH (never for SQLite,
// which has no network host) and returns the URL the driver should dial plus
// the tunnel handle the caller must Close.
func (c *Commands) openDial(ctx context.Context, conn models.Connection) (string, *helpers.Tunnel, error) {
	url, err := resolveVaultURL(ctx, conn)
	if err != nil {
		return "", nil, err
	}

	if !conn.UseSSH || conn.Provider == drivers.DriverSqlite {
		return url, nil, nil
	}

	sshCfg, err := sshConfigFromConnection(conn)
	if err != nil {
		return "", nil, err
	}

	rewritten, tunnel, err := helpers.OpenTunnelForURL(ctx, sshCfg, url, defaultDBPort(conn.Provider))
	if err != nil {
		return "", nil, err
	}
	return rewritten, tunnel, nil
}
