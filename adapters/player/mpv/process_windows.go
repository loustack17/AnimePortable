//go:build windows

// SPDX-License-Identifier: MPL-2.0

package mpv

import "os"

func gracefulStop(process *os.Process) error {
	return process.Kill()
}

func forceStop(process *os.Process) error {
	return process.Kill()
}
