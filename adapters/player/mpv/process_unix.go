//go:build darwin || linux

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
