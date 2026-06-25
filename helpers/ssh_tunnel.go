package helpers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	defaultSSHPort        = 22
	defaultSSHTimeout     = 10 * time.Second
	defaultTunnelLoopback = "127.0.0.1"
)

// SSHConfig describes a bastion to tunnel a DB connection through.
type SSHConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	PrivateKeyPath string
	Passphrase     string
	LocalPort      int
}

// sshDialFunc is the seam for ssh.Dial, swapped in tests.
var sshDialFunc = ssh.Dial

// agentDialFunc returns an SSH agent client. Returns nil agent + nil error
// when SSH_AUTH_SOCK is unset (treated as "agent unavailable").
var agentDialFunc = func() (agent.ExtendedAgent, io.Closer, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, nil, nil
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, nil, fmt.Errorf("ssh agent dial: %w", err)
	}
	return agent.NewClient(conn), conn, nil
}

var knownHostsPath = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

// expandHome resolves a leading "~/" or bare "~" to the user's home dir.
func expandHome(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	if path != "~" && path[1] != '/' {
		// Other-user expansion ("~alice/...") is not supported.
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

// dialSSH establishes an SSH connection using auth precedence:
// private key (with optional passphrase) -> password -> SSH agent.
func dialSSH(cfg *SSHConfig) (*ssh.Client, error) {
	if cfg == nil {
		return nil, errors.New("ssh config is nil")
	}
	if cfg.Host == "" {
		return nil, errors.New("ssh host is required")
	}
	if cfg.User == "" {
		return nil, errors.New("ssh user is required")
	}

	port := cfg.Port
	if port == 0 {
		port = defaultSSHPort
	}

	auth, agentCloser, err := buildAuthMethods(cfg)
	if err != nil {
		return nil, err
	}
	if agentCloser != nil {
		defer agentCloser.Close()
	}
	if len(auth) == 0 {
		return nil, errors.New("no SSH auth method available: provide private key, password, or run ssh-agent")
	}

	hostKeyCallback, err := buildHostKeyCallback()
	if err != nil {
		return nil, err
	}

	clientCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         defaultSSHTimeout,
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))
	client, err := sshDialFunc("tcp", addr, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	return client, nil
}

func buildAuthMethods(cfg *SSHConfig) ([]ssh.AuthMethod, io.Closer, error) {
	var methods []ssh.AuthMethod

	if cfg.PrivateKeyPath != "" {
		signer, err := loadPrivateKey(cfg.PrivateKeyPath, cfg.Passphrase)
		if err != nil {
			return nil, nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))
		return methods, nil, nil
	}

	if cfg.Password != "" {
		methods = append(methods, ssh.Password(cfg.Password))
		return methods, nil, nil
	}

	ag, closer, err := agentDialFunc()
	if err != nil {
		return nil, nil, err
	}
	if ag == nil {
		return methods, nil, nil
	}
	signers, err := ag.Signers()
	if err != nil {
		_ = closer.Close()
		return nil, nil, fmt.Errorf("ssh agent signers: %w", err)
	}
	if len(signers) == 0 {
		_ = closer.Close()
		return methods, nil, nil
	}
	methods = append(methods, ssh.PublicKeys(signers...))
	return methods, closer, nil
}

func loadPrivateKey(path, passphrase string) (ssh.Signer, error) {
	expanded, err := expandHome(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(expanded) // #nosec G304 -- path is user-configured
	if err != nil {
		return nil, fmt.Errorf("read private key %s: %w", expanded, err)
	}
	if passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("parse private key with passphrase: %w", err)
		}
		return signer, nil
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return signer, nil
}

func buildHostKeyCallback() (ssh.HostKeyCallback, error) {
	path, err := knownHostsPath()
	if err != nil {
		return nil, err
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("known_hosts %s not found; add the bastion first (ssh-keyscan -H <host> >> %s, or connect once with plain ssh): %w", path, path, err)
		}
		return nil, fmt.Errorf("load known_hosts %s: %w", path, err)
	}
	return cb, nil
}

// Tunnel is a local-listener SSH port forward. It accepts connections on a
// loopback port and forwards each through an SSH client to a remote address.
type Tunnel struct {
	listener   net.Listener
	sshClient  *ssh.Client
	remoteAddr string
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	mu     sync.Mutex
	closed bool
}

func (t *Tunnel) LocalAddr() string {
	return t.listener.Addr().String()
}

func (t *Tunnel) LocalPort() int {
	_, p, err := net.SplitHostPort(t.listener.Addr().String())
	if err != nil {
		return 0
	}
	port, _ := strconv.Atoi(p)
	return port
}

// Close stops the accept loop, closes the listener, and tears down the SSH
// client. Idempotent.
func (t *Tunnel) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	t.mu.Unlock()

	t.cancel()
	var errs []error
	if err := t.listener.Close(); err != nil {
		errs = append(errs, err)
	}
	t.wg.Wait()
	if t.sshClient != nil {
		if err := t.sshClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func startTunnel(parentCtx context.Context, sshClient *ssh.Client, remoteAddr string, localPort int) (*Tunnel, error) {
	if sshClient == nil {
		return nil, errors.New("ssh client is nil")
	}
	bind := net.JoinHostPort(defaultTunnelLoopback, strconv.Itoa(localPort))
	listener, err := net.Listen("tcp", bind)
	if err != nil {
		return nil, fmt.Errorf("tunnel listen %s: %w", bind, err)
	}

	ctx, cancel := context.WithCancel(parentCtx)
	t := &Tunnel{
		listener:   listener,
		sshClient:  sshClient,
		remoteAddr: remoteAddr,
		cancel:     cancel,
	}

	t.wg.Add(1)
	go t.acceptLoop(ctx)

	return t, nil
}

func (t *Tunnel) acceptLoop(ctx context.Context) {
	defer t.wg.Done()
	for {
		local, err := t.listener.Accept()
		if err != nil {
			return
		}
		t.wg.Add(1)
		go t.handleConn(ctx, local)
	}
}

func (t *Tunnel) handleConn(ctx context.Context, local net.Conn) {
	defer t.wg.Done()
	defer local.Close()

	remote, err := t.sshClient.Dial("tcp", t.remoteAddr)
	if err != nil {
		return
	}
	defer remote.Close()

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(remote, local)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(local, remote)
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}

// OpenSSHTunnel dials the bastion and starts a local forward to remoteAddr.
// The caller owns the returned Tunnel and must Close it.
func OpenSSHTunnel(ctx context.Context, cfg *SSHConfig, remoteAddr string) (*Tunnel, error) {
	if cfg == nil {
		return nil, errors.New("SSH configuration is missing")
	}
	client, err := dialSSH(cfg)
	if err != nil {
		return nil, err
	}
	t, err := startTunnel(ctx, client, remoteAddr, cfg.LocalPort)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return t, nil
}

// rewriteURLHostPort returns dbURL with its host:port replaced by newHost:newPort.
func rewriteURLHostPort(dbURL, newHost, newPort string) (string, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "", fmt.Errorf("parse connection url: %w", err)
	}
	if u.Host == "" {
		return "", errors.New("connection url has no host to tunnel")
	}
	u.Host = net.JoinHostPort(newHost, newPort)
	return u.String(), nil
}

// remoteAddrFromURL extracts host:port from dbURL, falling back to defaultPort
// when the URL omits the port.
func remoteAddrFromURL(dbURL, defaultPort string) (string, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "", fmt.Errorf("parse connection url: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return "", errors.New("connection url has no host to tunnel")
	}
	port := u.Port()
	if port == "" {
		port = defaultPort
	}
	return net.JoinHostPort(host, port), nil
}

// OpenTunnelForURL opens an SSH tunnel to the DB host:port encoded in dbURL and
// returns a rewritten URL pointing at the local forward plus the live Tunnel.
// defaultPort is used when dbURL omits an explicit port.
func OpenTunnelForURL(ctx context.Context, cfg *SSHConfig, dbURL, defaultPort string) (string, *Tunnel, error) {
	remoteAddr, err := remoteAddrFromURL(dbURL, defaultPort)
	if err != nil {
		return "", nil, err
	}
	tunnel, err := OpenSSHTunnel(ctx, cfg, remoteAddr)
	if err != nil {
		return "", nil, err
	}
	rewritten, err := rewriteURLHostPort(dbURL, defaultTunnelLoopback, strconv.Itoa(tunnel.LocalPort()))
	if err != nil {
		_ = tunnel.Close()
		return "", nil, err
	}
	return rewritten, tunnel, nil
}
