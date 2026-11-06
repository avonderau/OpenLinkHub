package backup

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Backup archive headers can specify permissive modes. Imported settings
// must never inherit them, including for implicit parent directories.
func TestRestoreArchiveNeverInstallsPermissiveModes(t *testing.T) {
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	write := func(name string, mode os.FileMode, contents string) {
		t.Helper()
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(mode)
		entry, err := archive.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	write("database/", os.ModeDir|0o777, "")
	write("database/public/", os.ModeDir|0o777, "")
	write("database/public/secret.json", 0o777, "private settings")
	write("database/implicit/secret.txt", 0o666, "more private settings")
	write("config.json", 0o666, "{}")
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	zipPath := filepath.Join(root, "restore.zip")
	if err := os.WriteFile(zipPath, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(root, "staging")
	if err := os.Mkdir(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := unzipFile(zipPath, staging); err != nil {
		t.Fatal(err)
	}
	assertPrivate := func(name string, want os.FileMode) {
		t.Helper()
		info, err := os.Stat(filepath.Join(staging, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s permissions = %#o, want %#o", name, got, want)
		}
	}
	assertPrivate("database", 0o700)
	assertPrivate("database/public", 0o700)
	assertPrivate("database/public/secret.json", 0o600)
	assertPrivate("database/implicit", 0o700)
	assertPrivate("database/implicit/secret.txt", 0o600)
	assertPrivate("config.json", 0o600)

	// The staged files are renamed into the live config folder during the
	// actual restore, so private permissions must survive installation.
	installed := filepath.Join(root, "installed")
	if err := os.Mkdir(installed, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := performRestore(staging, installed); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]os.FileMode{
		"config.json": 0o600, "database": 0o700,
		"database/public": 0o700, "database/public/secret.json": 0o600,
		"database/implicit": 0o700, "database/implicit/secret.txt": 0o600,
	} {
		info, err := os.Stat(filepath.Join(installed, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("installed %s permissions = %#o, want %#o", name, got, want)
		}
	}
}
