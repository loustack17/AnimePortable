package sqlite

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func databasePath(raw string) (string, error) {
	if raw == "" || strings.IndexByte(raw, 0) >= 0 || !filepath.IsAbs(raw) || isRemoteOrDevicePath(raw) {
		return "", ErrInvalidInput
	}
	path := filepath.Clean(raw)
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", ErrInvalidInput
		}
		return path, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", ErrStorage
	}
	return path, nil
}

func isRemoteOrDevicePath(path string) bool {
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`) {
		return true
	}
	if strings.HasPrefix(path, `\??\`) || strings.HasPrefix(path, `\\.\`) || strings.HasPrefix(path, `\\?\`) {
		return true
	}
	if isWindowsDrivePath(path) {
		return false
	}
	parsed, err := url.Parse(path)
	return err != nil || parsed.Scheme != ""
}

func isWindowsDrivePath(path string) bool {
	return len(path) > 2 && isASCIIAlpha(path[0]) && path[1] == ':' && (path[2] == '/' || path[2] == '\\')
}

func prepareDatabaseFile(path string) (bool, error) {
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err == nil {
		if !info.IsDir() {
			return false, ErrInvalidInput
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return false, ErrStorage
		}
	} else {
		return false, ErrStorage
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		closeErr := file.Close()
		if closeErr != nil {
			return true, ErrStorage
		}
		if chmodErr := os.Chmod(path, 0o600); chmodErr != nil && runtime.GOOS != "windows" {
			return true, ErrStorage
		}
		return true, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return false, ErrStorage
	}

	info, err = os.Lstat(path)
	if err != nil {
		return false, ErrStorage
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, ErrInvalidInput
	}
	return false, nil
}

func secureDatabaseArtifacts(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	for _, artifact := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(artifact, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
			return ErrStorage
		}
	}
	return nil
}

func databaseDSN(path string) string {
	slashPath := filepath.ToSlash(path)
	if isWindowsDrivePath(slashPath) {
		slashPath = "/" + slashPath
	}
	uri := url.URL{Scheme: "file", Path: slashPath}
	query := uri.Query()
	query.Set("_busy_timeout", "5000")
	query.Set("_foreign_keys", "on")
	query.Set("_journal_mode", "WAL")
	query.Set("nofollow", "1")
	uri.RawQuery = query.Encode()
	return uri.String()
}

func isASCIIAlpha(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z')
}
