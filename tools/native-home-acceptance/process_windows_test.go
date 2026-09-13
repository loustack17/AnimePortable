// SPDX-License-Identifier: MPL-2.0
//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsProcessLiveness(t *testing.T) {
	alive, err := processAlive(os.Getpid())
	if err != nil || !alive {
		t.Fatalf("current process: alive=%v err=%v", alive, err)
	}
	if _, err := processAlive(0); err == nil {
		t.Fatal("invalid PID accepted")
	}
}

func TestWindowsInventoryWaitsForSharingLock(t *testing.T) {
	p, err := newPaths(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(p.appdata, "animeportable.exe", "EBWebView")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "closing.tmp")
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inventory(p, true); err == nil {
		windows.CloseHandle(handle)
		t.Fatal("expected real Windows sharing violation")
	}
	closed := make(chan error, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		closed <- windows.CloseHandle(handle)
	}()
	_, scanErr := collectRuntimeInventory(p)
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if scanErr != nil {
		t.Fatal(scanErr)
	}
}
