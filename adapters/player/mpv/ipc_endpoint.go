package mpv

import (
	"context"
	"errors"
	"net"
	"sync"
)

var ErrIPCUnsupported = errors.New("mpv: IPC is unsupported on this platform")

type ipcEndpoint struct {
	name        string
	cleanupFn   func() error
	cleanupOnce sync.Once
	cleanupErr  error
}

func (endpoint *ipcEndpoint) String() string {
	return "mpv.IPCEndpoint{redacted}"
}

func (endpoint *ipcEndpoint) GoString() string {
	return "mpv.IPCEndpoint{redacted}"
}

func (endpoint *ipcEndpoint) cleanup() error {
	if endpoint == nil {
		return nil
	}
	endpoint.cleanupOnce.Do(func() {
		if endpoint.cleanupFn != nil {
			endpoint.cleanupErr = endpoint.cleanupFn()
		}
	})
	return endpoint.cleanupErr
}

func newEndpoint() (*ipcEndpoint, error) {
	return newIPCEndpoint()
}

func dialEndpoint(ctx context.Context, endpoint *ipcEndpoint, done <-chan struct{}) (net.Conn, error) {
	return dialIPC(ctx, endpoint, done)
}
