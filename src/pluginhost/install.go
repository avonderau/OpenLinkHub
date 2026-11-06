package pluginhost

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"OpenLinkHub/src/deckplugin"
)

// Archive limits for .streamDeckPlugin / .zip installs.
const (
	maxArchiveEntries   = 4096
	maxArchiveFileBytes = 64 << 20
	maxArchiveBytes     = 256 << 20
)

// PluginsDir is where the agent looks for installed plugins:
// $XDG_DATA_HOME/OpenLinkHub/deck-plugins (default ~/.local/share/...).
func PluginsDir() (string, error) {
	return xdgDir("XDG_DATA_HOME", ".local/share", "deck-plugins")
}

func xdgDir(env, fallback, leaf string) (string, error) {
	base := os.Getenv(env)
	if base == "" || !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, fallback)
	}
	return filepath.Join(base, "OpenLinkHub", leaf), nil
}

// Install copies a plugin into the plugins folder. The source is a
// .streamDeckPlugin (zip) archive or an unpacked *.sdPlugin folder.
// Hardware plugins and plugins without a usable manifest are refused.
func Install(source, pluginsDir string) (*Plugin, error) {
	if err := os.MkdirAll(pluginsDir, 0o700); err != nil {
		return nil, err
	}
	staging, err := os.MkdirTemp(pluginsDir, ".install-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)

	info, err := os.Stat(source)
	if err != nil {
		return nil, err
	}
	var folder string
	if info.IsDir() {
		if !strings.HasSuffix(filepath.Base(source), ".sdPlugin") {
			return nil, errors.New("plugin folder name must end in .sdPlugin")
		}
		folder = filepath.Join(staging, filepath.Base(source))
		if err := copyTree(source, folder); err != nil {
			return nil, err
		}
	} else {
		if folder, err = extractArchive(source, staging); err != nil {
			return nil, err
		}
	}
	plugin, err := LoadPlugin(folder)
	if err != nil {
		return nil, err
	}
	if errors.Is(plugin.Err, ErrDevicePlugin) {
		return nil, plugin.Err
	}
	target := filepath.Join(pluginsDir, plugin.UUID+".sdPlugin")
	if err := os.RemoveAll(target); err != nil {
		return nil, err
	}
	if err := os.Rename(folder, target); err != nil {
		return nil, err
	}
	return LoadPlugin(target)
}

// InstallArchive installs a plugin archive received from the service.
func InstallArchive(data []byte, pluginsDir string) (*Plugin, error) {
	if len(data) == 0 || len(data) > deckplugin.MaxUploadBytes {
		return nil, errors.New("plugin archive is empty or too large")
	}
	if err := os.MkdirAll(pluginsDir, 0o700); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(pluginsDir, ".upload-*.zip")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	return Install(tmp.Name(), pluginsDir)
}

// Remove uninstalls the plugin with this UUID from pluginsDir.
func Remove(uuid, pluginsDir string) error {
	if !deckplugin.ValidID(uuid) {
		return errors.New("invalid plugin ID")
	}
	for _, p := range ScanPlugins(pluginsDir) {
		if p.UUID != uuid {
			continue
		}
		dir := filepath.Clean(p.Dir)
		if filepath.Dir(dir) != filepath.Clean(pluginsDir) {
			return errors.New("plugin is outside the plugins folder")
		}
		info, err := os.Lstat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return errors.New("plugin folder is not a directory")
		}
		return os.RemoveAll(dir)
	}
	return errors.New("plugin is not installed")
}

// extractArchive unpacks exactly one top-level *.sdPlugin folder, refusing
// absolute paths, parent references, links and oversized content.
func extractArchive(path, dest string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open plugin archive: %w", err)
	}
	defer r.Close()
	if len(r.File) > maxArchiveEntries {
		return "", errors.New("plugin archive has too many files")
	}
	top := ""
	var total uint64
	for _, f := range r.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if name == "" || !filepath.IsLocal(name) || strings.Contains(name, "\x00") {
			return "", fmt.Errorf("unsafe archive entry %q", f.Name)
		}
		parts := strings.Split(strings.TrimSuffix(name, "/"), "/")
		for _, part := range parts {
			if part == ".." || part == "" {
				return "", fmt.Errorf("unsafe archive entry %q", f.Name)
			}
		}
		if parts[0] == "__MACOSX" {
			continue
		}
		if !strings.HasSuffix(parts[0], ".sdPlugin") {
			return "", errors.New("archive must contain one *.sdPlugin folder")
		}
		if top == "" {
			top = parts[0]
		} else if top != parts[0] {
			return "", errors.New("archive contains more than one plugin folder")
		}
		mode := f.Mode()
		if mode&os.ModeSymlink != 0 || (!mode.IsDir() && !mode.IsRegular()) {
			return "", fmt.Errorf("archive entry %q is not a regular file", f.Name)
		}
		target := filepath.Join(dest, filepath.FromSlash(strings.TrimSuffix(name, "/")))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return "", fmt.Errorf("unsafe archive entry %q", f.Name)
		}
		if mode.IsDir() || strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return "", err
			}
			continue
		}
		if f.UncompressedSize64 > maxArchiveFileBytes {
			return "", fmt.Errorf("archive entry %q is too large", f.Name)
		}
		total += f.UncompressedSize64
		if total > maxArchiveBytes {
			return "", errors.New("plugin archive is too large")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return "", err
		}
		perm := os.FileMode(0o600)
		if mode.Perm()&0o100 != 0 {
			perm = 0o700
		}
		if err := writeArchiveFile(f, target, perm); err != nil {
			return "", err
		}
	}
	if top == "" {
		return "", errors.New("archive contains no *.sdPlugin folder")
	}
	return filepath.Join(dest, top), nil
}

func writeArchiveFile(f *zip.File, target string, perm os.FileMode) error {
	in, err := f.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	// Never trust the declared size: stop one byte past the limit.
	n, err := io.Copy(out, io.LimitReader(in, maxArchiveFileBytes+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxArchiveFileBytes {
		err = fmt.Errorf("archive entry %q is too large", f.Name)
	}
	return err
}

// copyTree copies regular files and folders only; links are refused.
func copyTree(src, dst string) error {
	var total int64
	count := 0
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		count++
		if count > maxArchiveEntries {
			return errors.New("plugin folder has too many files")
		}
		switch {
		case info.IsDir():
			return os.MkdirAll(target, 0o700)
		case info.Mode().IsRegular():
			total += info.Size()
			if info.Size() > maxArchiveFileBytes || total > maxArchiveBytes {
				return errors.New("plugin folder is too large")
			}
			perm := os.FileMode(0o600)
			if info.Mode().Perm()&0o100 != 0 {
				perm = 0o700
			}
			in, err := os.Open(path)
			if err != nil {
				return err
			}
			defer in.Close()
			out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, in); err != nil {
				out.Close()
				return err
			}
			return out.Close()
		}
		return fmt.Errorf("plugin folder entry %q is not a regular file", rel)
	})
}
