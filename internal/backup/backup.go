package backup

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
)

// maxFileSize bounds a single extracted file (decompression-bomb guard);
// config artifacts are tiny, so 64MB is generous.
const maxFileSize = 64 << 20

// Export archives the whole cellar config dir (config.toml, saved_queries/,
// state/, history/) into out (tar.gz). When out is empty a timestamped name is
// written into dir (or the current directory when dir is empty too).
func Export(out, dir string) (string, error) {
	src, err := saved.GetAppConfigDir()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("nothing to back up: %w", err)
	}
	if out == "" {
		out = fmt.Sprintf("cellar-backup-%s.tar.gz", time.Now().Format("20060102-150405"))
		if dir != "" {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return "", fmt.Errorf("backup dir %s: %w", dir, err)
			}
			out = filepath.Join(dir, out)
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
	walkErr := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, path)
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
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if _, err := io.Copy(tw, in); err != nil {
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
		return "", fmt.Errorf("config dir %s is empty", src)
	}
	return out, f.Close()
}

// Import restores an Export archive. The current config dir is moved aside
// (cellar.pre-import-<ts>) rather than merged, so a restore is always
// reversible. Returns the aside path ("" when there was nothing to move).
func Import(archive string) (string, error) {
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
		n, err := io.CopyN(w, tr, maxFileSize+1)
		w.Close()
		if err != nil && err != io.EOF {
			return aside, err
		}
		if n > maxFileSize {
			return aside, fmt.Errorf("entry %q exceeds %d bytes", hdr.Name, maxFileSize)
		}
	}
	return aside, nil
}

// ExpandHome resolves a leading ~ or ~/ against the user's home directory.
func ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}
