//go:build windows

package mpv

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func newIPCEndpoint() (*ipcEndpoint, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return nil, ErrIPC
	}
	name := `\\.\pipe\animeportable-` + hex.EncodeToString(bytes[:])
	return &ipcEndpoint{name: name}, nil
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
		if restrictPipeToCurrentUser(endpoint.name) == nil {
			attemptCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			conn, err := winio.DialPipeContext(attemptCtx, endpoint.name)
			cancel()
			if err == nil {
				return conn, nil
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

func restrictPipeToCurrentUser(name string) error {
	securityDescriptor, err := currentUserSecurityDescriptor()
	if err != nil {
		return err
	}
	dacl, _, err := securityDescriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(name, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

func currentUserSecurityDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	if user == nil || user.User.Sid == nil {
		return nil, ErrIPC
	}
	return windows.SecurityDescriptorFromString("D:P(A;;GA;;;" + user.User.Sid.String() + ")")
}
