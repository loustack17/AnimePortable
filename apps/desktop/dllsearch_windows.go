// SPDX-License-Identifier: MPL-2.0

package main

import "golang.org/x/sys/windows"

// restrictDLLSearch makes every later load of a library by bare name search only the
// program's folder and System32 (docs/03_SECURITY.md, libmpv DLL loading). libmpv and FFmpeg
// load some libraries by name while playing (GPU and decoder runtimes); without this Windows
// also searches the current folder and PATH, where another program may have put a DLL.
func restrictDLLSearch() error {
	return windows.SetDefaultDllDirectories(windows.LOAD_LIBRARY_SEARCH_APPLICATION_DIR | windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
}
