//go:build windows && cgo

// SPDX-License-Identifier: MPL-2.0

// Package appicon is the app's icon (owner-supplied, 2026-10-22). The windows get it here; the
// executable gets icon.ico through ../rsrc_windows_amd64.syso, which make_syso.ps1 rebuilds.
package appicon

import (
	"bytes"
	"embed"
	"image/png"
	"sync"

	fltk "github.com/pwiecz/go-fltk"
)

// The sizes Windows asks for: title bar and taskbar at 100 to 150 percent scaling.
//
//go:embed icon-16.png icon-32.png icon-48.png
var files embed.FS

var (
	loadOnce sync.Once
	images   []*fltk.RgbImage
)

// Set gives window the app icon. FLTK copies the images, so every window can share them.
func Set(window *fltk.Window) {
	loadOnce.Do(load)
	if len(images) > 0 {
		window.SetIcons(images)
	}
}

// load decodes the embedded sizes; one that cannot be read is left out, so the worst case is
// Windows' default icon.
func load() {
	for _, name := range []string{"icon-16.png", "icon-32.png", "icon-48.png"} {
		data, err := files.ReadFile(name)
		if err != nil {
			continue
		}
		decoded, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		image, err := fltk.NewRgbImageFromImage(decoded)
		if err != nil {
			continue
		}
		images = append(images, image)
	}
}
