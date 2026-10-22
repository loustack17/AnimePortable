//go:build !windows

// SPDX-License-Identifier: MPL-2.0

package backend

func localDriveType(string) bool { return true }
