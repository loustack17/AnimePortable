//go:build darwin || linux

// SPDX-License-Identifier: MPL-2.0

package mpv

import (
	"os"
	"syscall"
)

func gracefulStop(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}

func forceStop(process *os.Process) error {
	return process.Kill()
}
