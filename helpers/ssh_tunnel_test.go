package helpers

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// ---------------------------------------------------------------------------
// In-process SSH server: accepts a single user/password OR public key; for
// "direct-tcpip" channel requests, dials the requested target and bridges the
// streams. Exercises the full tunnel path against a real ssh handshake.
// ---------------------------------------------------------------------------

type testSSHServer struct {
	listener net.Listener
	addr     string
	hostKey  ssh.Signer
	wg       sync.WaitGroup
	stop     chan struct{}

	allowedUser     string
	allowedPassword string
	allowedKey      ssh.PublicKey
}

func newTestSSHServer(t *testing.T) *testSSHServer {
	t.Helper()
	hostSigner := mustGenerateHostKey(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("test ssh server listen: %v", err)
	}
	srv := &testSSHServer{
		listener: listener,
		addr:     listener.Addr().String(),
		hostKey:  hostSigner,
		stop:     make(chan struct{}),
	}
	srv.wg.Add(1)
	go srv.acceptLoop(t)
	return srv
}

func (s *testSSHServer) Close() {
	close(s.stop)
	_ = s.listener.Close()
	s.wg.Wait()
}

func (s *testSSHServer) HostPublicKey() ssh.PublicKey { return s.hostKey.PublicKey() }

func (s *testSSHServer) acceptLoop(t *testing.T) {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go s.handleConn(t, conn)
	}
}

func (s *testSSHServer) handleConn(t *testing.T, conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if s.allowedUser != "" && c.User() != s.allowedUser {
				return nil, errors.New("user not allowed")
			}
			if s.allowedPassword == "" {
				return nil, errors.New("password auth disabled")
			}
			if string(password) != s.allowedPassword {
				return nil, errors.New("password mismatch")
			}
			return nil, nil
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if s.allowedKey == nil {
				return nil, errors.New("pubkey auth disabled")
			}
			if string(key.Marshal()) != string(s.allowedKey.Marshal()) {
				return nil, errors.New("pubkey mismatch")
			}
			return nil, nil
		},
	}
	cfg.AddHostKey(s.hostKey)

	srvConn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	defer srvConn.Close()

	for nc := range chans {
		if nc.ChannelType() != "direct-tcpip" {
			_ = nc.Reject(ssh.UnknownChannelType, "only direct-tcpip supported")
			continue
		}
		target := parseDirectTCPIP(nc.ExtraData())
		ch, _, err := nc.Accept()
		if err != nil {
			continue
		}
		go bridge(ch, target)
	}
}

// parseDirectTCPIP extracts host:port from the direct-tcpip channel extra data.
// Format: string host, uint32 port, string origin host, uint32 origin port.
func parseDirectTCPIP(data []byte) string {
	if len(data) < 4 {
		return ""
	}
	hostLen := int(uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3]))
	if len(data) < 4+hostLen+4 {
		return ""
	}
	host := string(data[4 : 4+hostLen])
	portOff := 4 + hostLen
	port := uint32(data[portOff])<<24 | uint32(data[portOff+1])<<16 | uint32(data[portOff+2])<<8 | uint32(data[portOff+3])
	return net.JoinHostPort(host, itoa(int(port)))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [11]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func bridge(ch ssh.Channel, target string) {
	defer ch.Close()
	if target == "" {
		return
	}
	remote, err := net.DialTimeout("tcp", target, 2*time.Second)
	if err != nil {
		return
	}
	defer remote.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(remote, ch); done <- struct{}{} }()
	go func() { _, _ = io.Copy(ch, remote); done <- struct{}{} }()
	<-done
}

func mustGenerateHostKey(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	return signer
}

// withTempKnownHosts redirects knownHostsPath to a file trusting host:port+key.
func withTempKnownHosts(t *testing.T, hostPort string, key ssh.PublicKey) func() {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "known_hosts")
	entry := knownhostsLine(t, hostPort, key)
	if err := os.WriteFile(file, []byte(entry), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	orig := knownHostsPath
	knownHostsPath = func() (string, error) { return file, nil }
	return func() { knownHostsPath = orig }
}

func knownhostsLine(t *testing.T, hostPort string, key ssh.PublicKey) string {
	t.Helper()
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	addr := host
	if port != "22" {
		addr = "[" + host + "]:" + port
	}
	return addr + " " + key.Type() + " " + base64Encode(key.Marshal()) + "\n"
}

func base64Encode(b []byte) string {
	const enc = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var out strings.Builder
	for i := 0; i < len(b); i += 3 {
		var n uint32
		switch {
		case i+2 < len(b):
			n = uint32(b[i])<<16 | uint32(b[i+1])<<8 | uint32(b[i+2])
		case i+1 < len(b):
			n = uint32(b[i])<<16 | uint32(b[i+1])<<8
		default:
			n = uint32(b[i]) << 16
		}
		out.WriteByte(enc[(n>>18)&0x3f])
		out.WriteByte(enc[(n>>12)&0x3f])
		if i+1 < len(b) {
			out.WriteByte(enc[(n>>6)&0x3f])
		} else {
			out.WriteByte('=')
		}
		if i+2 < len(b) {
			out.WriteByte(enc[n&0x3f])
		} else {
			out.WriteByte('=')
		}
	}
	return out.String()
}

// ---------------------------------------------------------------------------
// dialSSH validation
// ---------------------------------------------------------------------------

func TestDialSSH_NilConfig(t *testing.T) {
	_, err := dialSSH(nil)
	if err == nil || !strings.Contains(err.Error(), "ssh config is nil") {
		t.Errorf("dialSSH(nil) err = %v", err)
	}
}

func TestDialSSH_MissingHost(t *testing.T) {
	_, err := dialSSH(&SSHConfig{User: "u"})
	if err == nil || !strings.Contains(err.Error(), "host is required") {
		t.Errorf("dialSSH missing host err = %v", err)
	}
}

func TestDialSSH_MissingUser(t *testing.T) {
	_, err := dialSSH(&SSHConfig{Host: "h"})
	if err == nil || !strings.Contains(err.Error(), "user is required") {
		t.Errorf("dialSSH missing user err = %v", err)
	}
}

func TestDialSSH_NoAuthAvailable(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	cleanup := withTempKnownHosts(t, "127.0.0.1:22", mustGenerateHostKey(t).PublicKey())
	defer cleanup()
	_, err := dialSSH(&SSHConfig{Host: "127.0.0.1", User: "u"})
	if err == nil || !strings.Contains(err.Error(), "no SSH auth method available") {
		t.Errorf("dialSSH no auth err = %v", err)
	}
}

func TestDialSSH_KnownHostsLoadFail(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	orig := knownHostsPath
	knownHostsPath = func() (string, error) { return "/nonexistent/known_hosts", nil }
	defer func() { knownHostsPath = orig }()

	_, err := dialSSH(&SSHConfig{Host: "127.0.0.1", User: "u", Password: "p"})
	if err == nil || !strings.Contains(err.Error(), "known_hosts") {
		t.Errorf("dialSSH known_hosts err = %v", err)
	}
}

func TestDialSSH_DefaultPort(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	cleanup := withTempKnownHosts(t, "127.0.0.1:22", mustGenerateHostKey(t).PublicKey())
	defer cleanup()

	var capturedAddr string
	origDial := sshDialFunc
	sshDialFunc = func(network, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
		capturedAddr = addr
		return nil, errors.New("captured")
	}
	defer func() { sshDialFunc = origDial }()

	_, _ = dialSSH(&SSHConfig{Host: "127.0.0.1", User: "u", Password: "p"})
	if capturedAddr != "127.0.0.1:22" {
		t.Errorf("default port addr = %q, want 127.0.0.1:22", capturedAddr)
	}
}

// ---------------------------------------------------------------------------
// ProxyCommand bastion (SSM / jump host)
// ---------------------------------------------------------------------------

func TestExpandProxyTokens(t *testing.T) {
	got := expandProxyTokens(`ssh -W %h:%p as %r 100%%`, "db.internal", 5432, "bryan")
	want := `ssh -W db.internal:5432 as bryan 100%`
	if got != want {
		t.Errorf("expandProxyTokens = %q, want %q", got, want)
	}
}

// TestSSHProxyHelperProcess is not a real test: TestDialSSH_ViaProxyCommand
// re-execs it as the ProxyCommand subprocess, bridging stdio to the TCP addr in
// CELLAR_PROXY_HELPER_ADDR (avoids depending on nc, which is GNU netcat here).
func TestSSHProxyHelperProcess(t *testing.T) {
	addr := os.Getenv("CELLAR_PROXY_HELPER_ADDR")
	if addr == "" {
		return
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		os.Exit(3)
	}
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(conn, os.Stdin); done <- struct{}{} }()
	go func() { _, _ = io.Copy(os.Stdout, conn); done <- struct{}{} }()
	<-done
	_ = conn.Close()
	os.Exit(0)
}

func TestDialSSH_ViaProxyCommand(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	srv := newTestSSHServer(t)
	defer srv.Close()
	srv.allowedUser = "bryan"
	srv.allowedPassword = "secret"

	// echo server stands in for the DB behind the bastion.
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo listen: %v", err)
	}
	defer echo.Close()
	go func() {
		c, err := echo.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = io.Copy(c, c)
	}()

	// "bastion.invalid" has no TCP route; reachable ONLY via the ProxyCommand,
	// which re-execs the helper to dial the in-process sshd. Proves the proxy
	// path, with host-key verification still enforced against known_hosts.
	cleanup := withTempKnownHosts(t, "bastion.invalid:22", srv.HostPublicKey())
	defer cleanup()
	t.Setenv("CELLAR_PROXY_HELPER_ADDR", srv.addr)

	cfg := &SSHConfig{
		Host:         "bastion.invalid",
		User:         "bryan",
		Password:     "secret",
		ProxyCommand: os.Args[0] + " -test.run=TestSSHProxyHelperProcess",
	}

	tun, err := OpenSSHTunnel(context.Background(), cfg, echo.Addr().String())
	if err != nil {
		t.Fatalf("OpenSSHTunnel via proxy command: %v", err)
	}
	defer tun.Close()

	conn, err := net.Dial("tcp", tun.LocalAddr())
	if err != nil {
		t.Fatalf("dial local forward: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "ping" {
		t.Errorf("echo via proxy tunnel = %q, want ping", string(buf))
	}
}

func TestDialSSH_ProxyCommandTimeout(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	orig := proxyHandshakeTimeout
	proxyHandshakeTimeout = 200 * time.Millisecond
	defer func() { proxyHandshakeTimeout = orig }()

	cleanup := withTempKnownHosts(t, "bastion.invalid:22", mustGenerateHostKey(t).PublicKey())
	defer cleanup()

	cfg := &SSHConfig{
		Host:         "bastion.invalid",
		User:         "bryan",
		Password:     "secret",
		ProxyCommand: "sleep 5",
	}
	start := time.Now()
	_, err := dialSSH(cfg)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("dialSSH proxy timeout err = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("proxy timeout took %s, expected ~200ms", elapsed)
	}
}

// ---------------------------------------------------------------------------
// loadPrivateKey
// ---------------------------------------------------------------------------

func TestLoadPrivateKey_FileMissing(t *testing.T) {
	_, err := loadPrivateKey("/nonexistent/key", "")
	if err == nil || !strings.Contains(err.Error(), "read private key") {
		t.Errorf("loadPrivateKey missing file err = %v", err)
	}
}

func TestLoadPrivateKey_BadPEM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad")
	if err := os.WriteFile(path, []byte("not a key"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := loadPrivateKey(path, "")
	if err == nil || !strings.Contains(err.Error(), "parse private key") {
		t.Errorf("loadPrivateKey bad PEM err = %v", err)
	}
}

func TestLoadPrivateKey_Valid(t *testing.T) {
	path := writeTempPrivateKey(t, "")
	signer, err := loadPrivateKey(path, "")
	if err != nil || signer == nil {
		t.Fatalf("loadPrivateKey: signer=%v err=%v", signer, err)
	}
}

func TestLoadPrivateKey_ValidPassphrase(t *testing.T) {
	path := writeTempPrivateKey(t, "supersecret")
	signer, err := loadPrivateKey(path, "supersecret")
	if err != nil || signer == nil {
		t.Fatalf("loadPrivateKey w/ passphrase: signer=%v err=%v", signer, err)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}
	cases := map[string]string{
		"":              "",
		"/abs/path":     "/abs/path",
		"relative":      "relative",
		"~":             home,
		"~/":            home,
		"~/.ssh/id_rsa": filepath.Join(home, ".ssh", "id_rsa"),
		"~alice/x":      "~alice/x",
	}
	for in, want := range cases {
		got, err := expandHome(in)
		if err != nil {
			t.Errorf("expandHome(%q) err = %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("expandHome(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// dialSSH end-to-end against the in-process server
// ---------------------------------------------------------------------------

func TestDialSSH_PasswordAuthSuccess(t *testing.T) {
	srv := newTestSSHServer(t)
	defer srv.Close()
	srv.allowedUser = "alice"
	srv.allowedPassword = "secret"

	cleanup := withTempKnownHosts(t, srv.addr, srv.HostPublicKey())
	defer cleanup()

	host, port := splitForTest(t, srv.addr)
	client, err := dialSSH(&SSHConfig{Host: host, Port: port, User: "alice", Password: "secret"})
	if err != nil {
		t.Fatalf("dialSSH: %v", err)
	}
	defer client.Close()
}

func TestDialSSH_PasswordAuthRejected(t *testing.T) {
	srv := newTestSSHServer(t)
	defer srv.Close()
	srv.allowedUser = "alice"
	srv.allowedPassword = "secret"

	cleanup := withTempKnownHosts(t, srv.addr, srv.HostPublicKey())
	defer cleanup()

	host, port := splitForTest(t, srv.addr)
	if _, err := dialSSH(&SSHConfig{Host: host, Port: port, User: "alice", Password: "wrong"}); err == nil {
		t.Fatal("dialSSH expected auth failure")
	}
}

func TestDialSSH_HostKeyMismatch(t *testing.T) {
	srv := newTestSSHServer(t)
	defer srv.Close()
	srv.allowedUser = "alice"
	srv.allowedPassword = "secret"

	otherKey := mustGenerateHostKey(t).PublicKey()
	cleanup := withTempKnownHosts(t, srv.addr, otherKey)
	defer cleanup()

	host, port := splitForTest(t, srv.addr)
	if _, err := dialSSH(&SSHConfig{Host: host, Port: port, User: "alice", Password: "secret"}); err == nil {
		t.Fatal("dialSSH expected host key mismatch error")
	}
}

func TestDialSSH_PrivateKeyAuthSuccess(t *testing.T) {
	srv := newTestSSHServer(t)
	defer srv.Close()
	srv.allowedUser = "alice"

	keyPath := writeTempPrivateKey(t, "")
	srv.allowedKey = derivePubKey(t, keyPath, "")

	cleanup := withTempKnownHosts(t, srv.addr, srv.HostPublicKey())
	defer cleanup()

	host, port := splitForTest(t, srv.addr)
	client, err := dialSSH(&SSHConfig{Host: host, Port: port, User: "alice", PrivateKeyPath: keyPath})
	if err != nil {
		t.Fatalf("dialSSH: %v", err)
	}
	defer client.Close()
}

// ---------------------------------------------------------------------------
// Tunnel
// ---------------------------------------------------------------------------

func TestStartTunnel_NilSSHClient(t *testing.T) {
	_, err := startTunnel(context.Background(), nil, "127.0.0.1:1", 0)
	if err == nil || !strings.Contains(err.Error(), "ssh client is nil") {
		t.Errorf("startTunnel nil client err = %v", err)
	}
}

func TestTunnel_EndToEnd_Bridge(t *testing.T) {
	echo := newEchoServer(t)
	defer echo.Close()

	srv := newTestSSHServer(t)
	defer srv.Close()
	srv.allowedUser = "alice"
	srv.allowedPassword = "secret"

	cleanup := withTempKnownHosts(t, srv.addr, srv.HostPublicKey())
	defer cleanup()

	host, port := splitForTest(t, srv.addr)
	tunnel, err := OpenSSHTunnel(context.Background(), &SSHConfig{
		Host: host, Port: port, User: "alice", Password: "secret",
	}, echo.addr)
	if err != nil {
		t.Fatalf("OpenSSHTunnel: %v", err)
	}
	defer tunnel.Close()

	if tunnel.LocalPort() == 0 || tunnel.LocalAddr() == "" {
		t.Fatalf("tunnel local addr=%q port=%d", tunnel.LocalAddr(), tunnel.LocalPort())
	}

	conn, err := net.Dial("tcp", tunnel.LocalAddr())
	if err != nil {
		t.Fatalf("dial tunnel: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "ping" {
		t.Errorf("echoed = %q, want ping", buf)
	}
}

func TestTunnel_CloseIdempotent(t *testing.T) {
	srv := newTestSSHServer(t)
	defer srv.Close()
	srv.allowedUser = "alice"
	srv.allowedPassword = "secret"
	cleanup := withTempKnownHosts(t, srv.addr, srv.HostPublicKey())
	defer cleanup()

	host, port := splitForTest(t, srv.addr)
	tunnel, err := OpenSSHTunnel(context.Background(), &SSHConfig{
		Host: host, Port: port, User: "alice", Password: "secret",
	}, "127.0.0.1:1")
	if err != nil {
		t.Fatalf("OpenSSHTunnel: %v", err)
	}
	if err := tunnel.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := tunnel.Close(); err != nil {
		t.Errorf("second close should be no-op, got: %v", err)
	}
}

func TestOpenSSHTunnel_NilConfig(t *testing.T) {
	_, err := OpenSSHTunnel(context.Background(), nil, "127.0.0.1:1")
	if err == nil || !strings.Contains(err.Error(), "SSH configuration is missing") {
		t.Errorf("OpenSSHTunnel nil cfg err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// URL rewrite / orchestration
// ---------------------------------------------------------------------------

func TestRemoteAddrFromURL(t *testing.T) {
	cases := []struct {
		url, def, want string
	}{
		{"mysql://u:p@db.example.com:3306/foo", "3306", "db.example.com:3306"},
		{"postgres://u@db.example.com/foo", "5432", "db.example.com:5432"},
		{"sqlserver://u:p@host/db", "1433", "host:1433"},
	}
	for _, c := range cases {
		got, err := remoteAddrFromURL(c.url, c.def)
		if err != nil {
			t.Errorf("remoteAddrFromURL(%q) err = %v", c.url, err)
			continue
		}
		if got != c.want {
			t.Errorf("remoteAddrFromURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestRemoteAddrFromURL_NoHost(t *testing.T) {
	if _, err := remoteAddrFromURL("sqlite:///tmp/foo.db", "0"); err == nil {
		t.Error("expected error for url with no host")
	}
}

func TestRewriteURLHostPort(t *testing.T) {
	got, err := rewriteURLHostPort("mysql://u:p@db.example.com:3306/foo?x=1", "127.0.0.1", "54321")
	if err != nil {
		t.Fatalf("rewriteURLHostPort: %v", err)
	}
	want := "mysql://u:p@127.0.0.1:54321/foo?x=1"
	if got != want {
		t.Errorf("rewriteURLHostPort = %q, want %q", got, want)
	}
}

func TestOpenTunnelForURL_RewriteAndForward(t *testing.T) {
	echo := newEchoServer(t)
	defer echo.Close()

	srv := newTestSSHServer(t)
	defer srv.Close()
	srv.allowedUser = "alice"
	srv.allowedPassword = "secret"
	cleanup := withTempKnownHosts(t, srv.addr, srv.HostPublicKey())
	defer cleanup()

	echoHost, echoPort := splitForTest(t, echo.addr)
	dbURL := "mysql://user:pass@" + echoHost + ":" + itoa(echoPort) + "/mydb?parseTime=true"

	host, port := splitForTest(t, srv.addr)
	rewritten, tunnel, err := OpenTunnelForURL(context.Background(), &SSHConfig{
		Host: host, Port: port, User: "alice", Password: "secret",
	}, dbURL, "3306")
	if err != nil {
		t.Fatalf("OpenTunnelForURL: %v", err)
	}
	defer tunnel.Close()

	wantPrefix := "mysql://user:pass@127.0.0.1:" + itoa(tunnel.LocalPort())
	if !strings.HasPrefix(rewritten, wantPrefix) {
		t.Errorf("rewritten = %q, want prefix %q", rewritten, wantPrefix)
	}
	if !strings.Contains(rewritten, "/mydb?parseTime=true") {
		t.Errorf("rewritten lost path/query: %q", rewritten)
	}

	// Data path: dial the rewritten host:port, echo round-trips through the tunnel.
	conn, err := net.Dial("tcp", "127.0.0.1:"+itoa(tunnel.LocalPort()))
	if err != nil {
		t.Fatalf("dial rewritten: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("hey!")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "hey!" {
		t.Errorf("echoed = %q, want hey!", buf)
	}
}

// ---------------------------------------------------------------------------
// agent
// ---------------------------------------------------------------------------

func TestAgentDialFunc_NoSocket(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	ag, closer, err := agentDialFunc()
	if err != nil {
		t.Fatalf("agentDialFunc no socket err = %v", err)
	}
	if ag != nil || closer != nil {
		t.Errorf("agentDialFunc no socket = (%v, %v), want (nil, nil)", ag, closer)
	}
}

func TestAgentSignersError(t *testing.T) {
	origAgent := agentDialFunc
	agentDialFunc = func() (agent.ExtendedAgent, io.Closer, error) {
		return &fakeAgent{signersErr: errors.New("signers boom")}, nopCloser{}, nil
	}
	defer func() { agentDialFunc = origAgent }()
	cleanup := withTempKnownHosts(t, "127.0.0.1:22", mustGenerateHostKey(t).PublicKey())
	defer cleanup()

	_, err := dialSSH(&SSHConfig{Host: "127.0.0.1", User: "u"})
	if err == nil || !strings.Contains(err.Error(), "signers boom") {
		t.Errorf("dialSSH agent signers err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func splitForTest(t *testing.T, hostPort string) (string, int) {
	t.Helper()
	host, p, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("split %q: %v", hostPort, err)
	}
	port := 0
	for _, c := range p {
		port = port*10 + int(c-'0')
	}
	return host, port
}

func writeTempPrivateKey(t *testing.T, passphrase string) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	var block *pem
	if passphrase == "" {
		b, err := ssh.MarshalPrivateKey(priv, "")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		block = &pem{Type: b.Type, Bytes: b.Bytes}
	} else {
		b, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
		if err != nil {
			t.Fatalf("marshal w/ passphrase: %v", err)
		}
		block = &pem{Type: b.Type, Bytes: b.Bytes}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(path, encodePEM(block.Type, block.Bytes), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return path
}

type pem struct {
	Type  string
	Bytes []byte
}

func encodePEM(typ string, data []byte) []byte {
	var b strings.Builder
	b.WriteString("-----BEGIN ")
	b.WriteString(typ)
	b.WriteString("-----\n")
	enc := base64Encode(data)
	for i := 0; i < len(enc); i += 64 {
		end := i + 64
		if end > len(enc) {
			end = len(enc)
		}
		b.WriteString(enc[i:end])
		b.WriteString("\n")
	}
	b.WriteString("-----END ")
	b.WriteString(typ)
	b.WriteString("-----\n")
	return []byte(b.String())
}

func derivePubKey(t *testing.T, keyPath, passphrase string) ssh.PublicKey {
	t.Helper()
	signer, err := loadPrivateKey(keyPath, passphrase)
	if err != nil {
		t.Fatalf("loadPrivateKey: %v", err)
	}
	return signer.PublicKey()
}

type echoServer struct {
	listener net.Listener
	addr     string
	wg       sync.WaitGroup
}

func newEchoServer(t *testing.T) *echoServer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo listen: %v", err)
	}
	e := &echoServer{listener: l, addr: l.Addr().String()}
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			e.wg.Add(1)
			go func() {
				defer e.wg.Done()
				defer c.Close()
				_, _ = io.Copy(c, c)
			}()
		}
	}()
	return e
}

func (e *echoServer) Close() {
	_ = e.listener.Close()
	e.wg.Wait()
}

type fakeAgent struct {
	signers    []ssh.Signer
	signersErr error
}

func (f *fakeAgent) List() ([]*agent.Key, error)                        { return nil, nil }
func (f *fakeAgent) Sign(ssh.PublicKey, []byte) (*ssh.Signature, error) { return nil, nil }
func (f *fakeAgent) Add(agent.AddedKey) error                           { return nil }
func (f *fakeAgent) Remove(ssh.PublicKey) error                         { return nil }
func (f *fakeAgent) RemoveAll() error                                   { return nil }
func (f *fakeAgent) Lock([]byte) error                                  { return nil }
func (f *fakeAgent) Unlock([]byte) error                                { return nil }
func (f *fakeAgent) Signers() ([]ssh.Signer, error)                     { return f.signers, f.signersErr }
func (f *fakeAgent) SignWithFlags(ssh.PublicKey, []byte, agent.SignatureFlags) (*ssh.Signature, error) {
	return nil, nil
}
func (f *fakeAgent) Extension(string, []byte) ([]byte, error) { return nil, nil }

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
