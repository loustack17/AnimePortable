//go:build !linux && !darwin && !windows

// SPDX-License-Identifier: MPL-2.0

package sqlite

import "os"

func installPortableSnapshot(stage, target string) error {
	if err := os.Link(stage, target); err != nil {
		return err
	}
	return os.Remove(stage)
}
