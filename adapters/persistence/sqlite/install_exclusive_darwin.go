//go:build darwin

// SPDX-License-Identifier: MPL-2.0

package sqlite

import "golang.org/x/sys/unix"

func installPortableSnapshot(stage, target string) error {
	return unix.RenamexNp(stage, target, unix.RENAME_EXCL)
}
