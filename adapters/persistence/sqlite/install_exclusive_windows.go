//go:build windows

// SPDX-License-Identifier: MPL-2.0

package sqlite

import "golang.org/x/sys/windows"

func installPortableSnapshot(stage, target string) error {
	from, err := windows.UTF16PtrFromString(stage)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFile(from, to)
}
