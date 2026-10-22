//go:build windows

// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// localDriveType accepts fixed and removable drives only: SQLite WAL is unsafe on network file systems.
func localDriveType(path string) bool {
	volume := filepath.VolumeName(path)
	if volume == "" {
		return false
	}
	root, err := windows.UTF16PtrFromString(volume + `\`)
	if err != nil {
		return false
	}
	switch windows.GetDriveType(root) {
	case windows.DRIVE_FIXED, windows.DRIVE_REMOVABLE:
		return true
	}
	return false
}
