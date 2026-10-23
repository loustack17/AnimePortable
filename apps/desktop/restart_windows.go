// SPDX-License-Identifier: MPL-2.0

package main

import "golang.org/x/sys/windows"

// showNativeError uses the native message box, for errors when no FLTK window exists: FLTK
// has not started or is already shut down. The app has no console to print to.
func showNativeError(message string) {
	text, textErr := windows.UTF16PtrFromString(message)
	title, titleErr := windows.UTF16PtrFromString("AnimePortable")
	if textErr != nil || titleErr != nil {
		return
	}
	_, _ = windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONERROR)
}
