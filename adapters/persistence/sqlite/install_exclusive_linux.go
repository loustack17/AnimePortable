//go:build linux

// SPDX-License-Identifier: MPL-2.0

package sqlite

import "golang.org/x/sys/unix"

func installPortableSnapshot(stage, target string) error {
	return unix.Renameat2(unix.AT_FDCWD, stage, unix.AT_FDCWD, target, unix.RENAME_NOREPLACE)
}
