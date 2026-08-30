//go:build !windows && !darwin && !linux

package mpv

import (
	"context"
	"net"
)

func newIPCEndpoint() (*ipcEndpoint, error) {
	return nil, ErrIPCUnsupported
}

func dialIPC(context.Context, *ipcEndpoint, <-chan struct{}) (net.Conn, error) {
	return nil, ErrIPCUnsupported
}
