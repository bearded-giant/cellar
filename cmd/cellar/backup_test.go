package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bearded-giant/cellar/internal/tui/config"
)

func seedConfig(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "cellar")
	if err := os.MkdirAll(filepath.Join(dir, "saved_queries"), 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"config.toml":                 "[[database]]\nname = \"prod\"\n",
		"saved_queries/prod.toml":     "[[query]]\nname = \"users\"\nsql = \"select 1\"\n",
		filepath.Join("state", "prod.json"): `{"tabs":[{"sql":"select 1"}]}`,
	}
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestBackupRoundTrip(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	dir := seedConfig(t, root)

	out := filepath.Join(t.TempDir(), "backup.tar.gz")
	got, err := exportBackup(out)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if got != out {
		t.Fatalf("export path = %q, want %q", got, out)
	}

	// clobber the live config, then restore
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("ruined"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "saved_queries")); err != nil {
		t.Fatal(err)
	}

	aside, err := importBackup(out)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if aside == "" {
		t.Error("import should set the previous config aside")
	} else if b, err := os.ReadFile(filepath.Join(aside, "config.toml")); err != nil || string(b) != "ruined" {
		t.Errorf("aside dir should hold the pre-import config, got %q err %v", b, err)
	}

	for rel, want := range map[string]string{
		"config.toml":             "[[database]]\nname = \"prod\"\n",
		"saved_queries/prod.toml": "[[query]]\nname = \"users\"\nsql = \"select 1\"\n",
		"state/prod.json":         `{"tabs":[{"sql":"select 1"}]}`,
	} {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("restored %s: %v", rel, err)
		}
		if string(b) != want {
			t.Errorf("restored %s = %q, want %q", rel, b, want)
		}
	}
}

func TestImportRejectsTraversal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	hostile := filepath.Join(t.TempDir(), "evil.tar.gz")
	f, err := os.Create(hostile)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := []byte("pwned")
	if err := tw.WriteHeader(&tar.Header{Name: "../evil.txt", Mode: 0o600, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	f.Close()

	if _, err := importBackup(hostile); err == nil || !strings.Contains(err.Error(), "suspicious") {
		t.Errorf("hostile path must be rejected, got err=%v", err)
	}
}

func TestConfigSetGetAndBackupDirExport(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	seedConfig(t, root)
	cfgPath := filepath.Join(root, "cellar", "config.toml")

	bd := filepath.Join(root, "backups")
	if _, err := config.SetAppSetting(cfgPath, "backupdir", bd); err != nil {
		t.Fatalf("set: %v", err)
	}
	name, v, err := config.GetAppSetting(cfgPath, "BackupDir")
	if err != nil || name != "BackupDir" || v != bd {
		t.Fatalf("get = %q %q %v, want BackupDir %q", name, v, err, bd)
	}
	// connections survive the settings round-trip
	if b, _ := os.ReadFile(cfgPath); !strings.Contains(string(b), "prod") {
		t.Fatalf("config lost connections on settings write:\n%s", b)
	}

	out, err := exportBackup("")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if filepath.Dir(out) != bd {
		t.Errorf("default export landed in %s, want BackupDir %s", filepath.Dir(out), bd)
	}
}

func TestExportNothingToBackUp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := exportBackup(""); err == nil {
		t.Error("export with no config dir must error")
	}
}
