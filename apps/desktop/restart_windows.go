// SPDX-License-Identifier: MPL-2.0

package main

import "golang.org/x/sys/windows"

// showRestartFailure uses the native message box because FLTK is already shut down.
func showRestartFailure(message string) {
	text, textErr := windows.UTF16PtrFromString(message)
	title, titleErr := windows.UTF16PtrFromString("AnimePortable")
	if textErr != nil || titleErr != nil {
		return
	}
	_, _ = windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONERROR)
}
