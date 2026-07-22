package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bearded-giant/cellar/internal/saved"
	"github.com/bearded-giant/cellar/internal/tui/config"
)

// maxBackupFileSize bounds a single extracted file (decompression-bomb guard);
// config artifacts are tiny, so 64MB is generous.
const maxBackupFileSize = 64 << 20

// exportBackup archives the whole cellar config dir (config.toml,
// saved_queries/, state/, history/) into out (tar.gz).
func exportBackup(out string) (string, error) {
	dir, err := saved.GetAppConfigDir()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("nothing to back up: %w", err)
	}
	if out == "" {
		out = fmt.Sprintf("cellar-backup-%s.tar.gz", time.Now().Format("20060102-150405"))
		if bd := configuredBackupDir(); bd != "" {
			if err := os.MkdirAll(bd, 0o700); err != nil {
				return "", fmt.Errorf("backup dir %s: %w", bd, err)
			}
			out = filepath.Join(bd, out)
		}
	}

	f, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) // holds credentials
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	count := 0
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr := &tar.Header{
			Name:    filepath.ToSlash(rel),
			Mode:    int64(info.Mode().Perm()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		if _, err := io.Copy(tw, src); err != nil {
			return err
		}
		count++
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}
	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	if count == 0 {
		return "", fmt.Errorf("config dir %s is empty", dir)
	}
	return out, f.Close()
}

// importBackup restores an exportBackup archive. The current config dir is
// moved aside (cellar.pre-import-<ts>) rather than merged, so a restore is
// always reversible.
func importBackup(archive string) (string, error) {
	dir, err := saved.GetAppConfigDir()
	if err != nil {
		return "", err
	}

	f, err := os.Open(archive)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("not a cellar backup (gzip): %w", err)
	}
	tr := tar.NewReader(gz)

	aside := ""
	if _, err := os.Stat(dir); err == nil {
		aside = dir + ".pre-import-" + time.Now().Format("20060102-150405")
		if err := os.Rename(dir, aside); err != nil {
			return "", fmt.Errorf("set aside current config: %w", err)
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return aside, err
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return aside, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.FromSlash(hdr.Name)
		if strings.Contains(name, "..") || filepath.IsAbs(name) {
			return aside, fmt.Errorf("refusing suspicious archive path %q", hdr.Name)
		}
		dst := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return aside, err
		}
		w, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode).Perm())
		if err != nil {
			return aside, err
		}
		n, err := io.CopyN(w, tr, maxBackupFileSize+1)
		w.Close()
		if err != nil && err != io.EOF {
			return aside, err
		}
		if n > maxBackupFileSize {
			return aside, fmt.Errorf("entry %q exceeds %d bytes", hdr.Name, maxBackupFileSize)
		}
	}
	return aside, nil
}

// configuredBackupDir reads the BackupDir setting; "" when unset/unreadable
// (export then falls back to the current directory).
func configuredBackupDir() string {
	path, err := config.DefaultConfigFile()
	if err != nil {
		return ""
	}
	_, v, err := config.GetAppSetting(path, "BackupDir")
	if err != nil {
		return ""
	}
	return expandHome(v)
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func runConfigCommand(args []string) {
	path, err := config.DefaultConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cellar config: %v\n", err)
		os.Exit(1)
	}
	usage := func() {
		fmt.Fprintln(os.Stderr, "usage: cellar config list | get <key> | set <key> <value>")
		os.Exit(2)
	}
	if len(args) < 2 {
		usage()
	}
	switch args[1] {
	case "list":
		rows, err := config.ListAppSettings(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cellar config: %v\n", err)
			os.Exit(1)
		}
		for _, r := range rows {
			fmt.Printf("%-32s %s\n", r[0], r[1])
		}
	case "get":
		if len(args) < 3 {
			usage()
		}
		name, v, err := config.GetAppSetting(path, args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "cellar config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s = %s\n", name, v)
	case "set":
		if len(args) < 4 {
			usage()
		}
		name, err := config.SetAppSetting(path, args[2], args[3])
		if err != nil {
			fmt.Fprintf(os.Stderr, "cellar config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("cellar: %s = %s saved to %s\n", name, args[3], path)
	default:
		usage()
	}
}

func runBackupCommand(args []string) {
	switch args[0] {
	case "export":
		out := ""
		if len(args) > 1 {
			out = args[1]
		}
		path, err := exportBackup(out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cellar export: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("cellar: backed up connections, saved queries, buffers and history to %s\n", path)
		fmt.Println("cellar: the archive contains connection credentials — store it somewhere safe")
	case "import":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: cellar import <backup.tar.gz>")
			os.Exit(2)
		}
		aside, err := importBackup(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "cellar import: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("cellar: restored from %s\n", args[1])
		if aside != "" {
			fmt.Printf("cellar: previous config kept at %s (delete it once you're happy)\n", aside)
		}
	}
}
