//go:build !windows

// SPDX-License-Identifier: MPL-2.0

package forwardproxy

// readSystem: only the Windows system proxy is supported; elsewhere system mode is direct.
func readSystem() (SystemProxy, error) { return SystemProxy{}, nil }
