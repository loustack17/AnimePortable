// SPDX-License-Identifier: MPL-2.0
//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func lockGuard(file *os.File) (func() error, error) {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, err
	}
	return func() error { return syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }, nil
}

func processAlive(pid int) (bool, error) {
	if pid <= 0 {
		return false, errors.New("invalid process id")
	}
	if err := syscall.Kill(pid, 0); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return false, nil
		}
		return false, fmt.Errorf("query process %d: %w", pid, err)
	}
	return true, nil
}

func nativeProcessesStopped() error {
	return errors.New("native process recovery is supported only on Windows")
}
