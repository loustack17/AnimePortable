//go:build !windows && !darwin && !linux

// SPDX-License-Identifier: MPL-2.0

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
