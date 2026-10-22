//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import "golang.org/x/sys/windows/registry"

// readAppsUseLightTheme reads the Windows app theme (0 dark, 1 light). Tests replace it so
// they do not depend on the registry of the machine that runs them.
var readAppsUseLightTheme = func() (uint64, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return 0, err
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue("AppsUseLightTheme")
	return value, err
}

// systemUsesDarkTheme follows the Windows app theme. A missing or unreadable value means
// light, which is also what older Windows versions without a dark mode use.
func systemUsesDarkTheme() bool {
	value, err := readAppsUseLightTheme()
	return err == nil && value == 0
}
