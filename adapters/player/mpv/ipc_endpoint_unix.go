//go:build darwin || linux

package mpv

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const maxUnixSocketPath = 100

func newIPCEndpoint() (*ipcEndpoint, error) {
	for _, base := range runtimeBases() {
		endpoint, err := newUnixEndpoint(base)
		if err == nil {
			return endpoint, nil
		}
	}
	return nil, ErrIPC
}

func newUnixEndpoint(base string) (*ipcEndpoint, error) {
	directory, err := os.MkdirTemp(base, ".animeportable-mpv-")
	if err != nil {
		return nil, ErrIPC
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		_ = os.Remove(directory)
		return nil, ErrIPC
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		_ = os.Remove(directory)
		return nil, ErrIPC
	}
	path := filepath.Join(directory, "ipc.sock")
	if len(path) > maxUnixSocketPath {
		_ = os.Remove(directory)
		return nil, ErrIPC
	}
	return &ipcEndpoint{
		name: path,
		cleanupFn: func() error {
			var cleanupErr error
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				cleanupErr = ErrIPC
			}
			if err := os.Remove(directory); err != nil && !errors.Is(err, os.ErrNotExist) {
				cleanupErr = ErrIPC
			}
			return cleanupErr
		},
	}, nil
}

func runtimeBases() []string {
	bases := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)
	for _, base := range []string{os.Getenv("XDG_RUNTIME_DIR"), os.TempDir()} {
		if base == "" || len(filepath.Join(base, ".animeportable-mpv-0000000000", "ipc.sock")) > maxUnixSocketPath {
			continue
		}
		if _, duplicate := seen[base]; duplicate {
			continue
		}
		info, err := os.Stat(base)
		if err == nil && trustedRuntimeBase(info) {
			seen[base] = struct{}{}
			bases = append(bases, base)
		}
	}
	return bases
}

func trustedRuntimeBase(info os.FileInfo) bool {
	if info == nil || !info.IsDir() {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || (stat.Uid != uint32(os.Geteuid()) && stat.Uid != 0) {
		return false
	}
	return info.Mode().Perm()&0o022 == 0 || info.Mode()&os.ModeSticky != 0
}

func dialIPC(ctx context.Context, endpoint *ipcEndpoint, done <-chan struct{}) (net.Conn, error) {
	if endpoint == nil || endpoint.name == "" || ctx == nil {
		return nil, ErrIPC
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		select {
		case <-done:
			return nil, ErrExited
		default:
		}
		if info, err := os.Lstat(endpoint.name); err == nil && info.Mode()&os.ModeSocket != 0 {
			if os.Chmod(endpoint.name, 0o600) == nil {
				conn, dialErr := (&net.Dialer{}).DialContext(ctx, "unix", endpoint.name)
				if dialErr == nil {
					return conn, nil
				}
			}
		}
		select {
		case <-done:
			return nil, ErrExited
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
