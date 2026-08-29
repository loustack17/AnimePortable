//go:build windows

package mpv

import "os"

func gracefulStop(process *os.Process) error {
	return process.Kill()
}

func forceStop(process *os.Process) error {
	return process.Kill()
}
