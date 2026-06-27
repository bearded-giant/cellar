package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jorgerojas26/lazysql/models"
)

func TestSaveConnections_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := &Config{
		ConfigFile: path,
		AppConfig: &models.AppConfig{
			DefaultPageSize:              300,
			MaxQueryHistoryPerConnection: 100,
			TreeWidth:                    30,
		},
	}

	conns := []models.Connection{
		{
			Name:     "plain-mysql",
			URL:      "mysql://user:pass@host:3306/db",
			Provider: "mysql",
			ReadOnly: true,
		},
		{
			Name:          "ssh-postgres",
			URL:           "postgres://user:pass@db.internal:5432/app",
			Provider:      "postgres",
			UseSSH:        true,
			SSHHost:       "bastion.example.com",
			SSHPort:       "2222",
			SSHUser:       "deploy",
			SSHKeyFile:    "~/.ssh/id_ed25519",
			SSHPassphrase: "should-not-persist",
			SSHPassword:   "also-should-not-persist",
		},
	}

	if err := cfg.SaveConnections(conns); err != nil {
		t.Fatalf("SaveConnections failed: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(loaded.Connections) != len(conns) {
		t.Fatalf("loaded %d connections, want %d", len(loaded.Connections), len(conns))
	}

	plain := loaded.Connections[0]
	if plain.Name != "plain-mysql" {
		t.Errorf("Name = %q, want plain-mysql", plain.Name)
	}
	if plain.URL != "mysql://user:pass@host:3306/db" {
		t.Errorf("URL = %q, want mysql URL", plain.URL)
	}
	if plain.Provider != "mysql" {
		t.Errorf("Provider = %q, want mysql", plain.Provider)
	}
	if !plain.ReadOnly {
		t.Error("ReadOnly = false, want true")
	}

	ssh := loaded.Connections[1]
	if !ssh.UseSSH {
		t.Error("UseSSH = false, want true")
	}
	if ssh.SSHHost != "bastion.example.com" {
		t.Errorf("SSHHost = %q, want bastion.example.com", ssh.SSHHost)
	}
	if ssh.SSHPort != "2222" {
		t.Errorf("SSHPort = %q, want 2222", ssh.SSHPort)
	}
	if ssh.SSHUser != "deploy" {
		t.Errorf("SSHUser = %q, want deploy", ssh.SSHUser)
	}
	if ssh.SSHKeyFile != "~/.ssh/id_ed25519" {
		t.Errorf("SSHKeyFile = %q, want ~/.ssh/id_ed25519", ssh.SSHKeyFile)
	}
}

func TestSaveConnections_BlanksSecretsAndSetsPerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := &Config{ConfigFile: path, AppConfig: &models.AppConfig{}}
	conns := []models.Connection{{
		Name:          "ssh-conn",
		URL:           "postgres://u:p@h:5432/d",
		Provider:      "postgres",
		UseSSH:        true,
		SSHHost:       "bastion",
		SSHUser:       "deploy",
		SSHPassphrase: "secret-phrase",
		SSHPassword:   "secret-pw",
	}}

	if err := cfg.SaveConnections(conns); err != nil {
		t.Fatalf("SaveConnections failed: %v", err)
	}

	// In-memory copy keeps the secrets for same-session reconnects.
	if conns[0].SSHPassphrase != "secret-phrase" {
		t.Error("caller's in-memory SSHPassphrase was mutated, should be untouched")
	}
	if conns[0].SSHPassword != "secret-pw" {
		t.Error("caller's in-memory SSHPassword was mutated, should be untouched")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	disk := string(raw)
	if contains(disk, "secret-phrase") {
		t.Errorf("on-disk config leaked SSHPassphrase:\n%s", disk)
	}
	if contains(disk, "secret-pw") {
		t.Errorf("on-disk config leaked SSHPassword:\n%s", disk)
	}

	// Reload to confirm the blanked secrets are gone on the round-trip too.
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if loaded.Connections[0].SSHPassphrase != "" {
		t.Errorf("loaded SSHPassphrase = %q, want empty", loaded.Connections[0].SSHPassphrase)
	}
	if loaded.Connections[0].SSHPassword != "" {
		t.Errorf("loaded SSHPassword = %q, want empty", loaded.Connections[0].SSHPassword)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("file perms = %o, want 600", perm)
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
