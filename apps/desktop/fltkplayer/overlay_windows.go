//go:build windows && cgo

package fltkplayer

/*
#include <stdlib.h>
#cgo LDFLAGS: -lopengl32
void ap_overlay_draw(int width, int height, int visible, int menu, int episodes, int volume_open, int playing, int paused, int fullscreen, int focus, int menu_cursor, int volume, int episode, int episode_count, int episode_start, int episode_cursor, int resolution, double position, double duration, const char* episode_labels, const char* menu_labels);
*/
import "C"

import "unsafe"

func drawPlayerOverlay(width, height int, view *View) {
	state := view.state
	C.ap_overlay_draw(
		C.int(width), C.int(height), C.int(boolInt(view.visible)), C.int(boolInt(view.menuVisible())),
		C.int(boolInt(view.episodesOpen)), C.int(boolInt(view.volumeOpen)), C.int(boolInt(state.Playing)),
		C.int(boolInt(state.Paused)), C.int(boolInt(state.Fullscreen)), C.int(state.Focus), C.int(view.menuCursor),
		C.int(state.Volume), C.int(state.Episode), C.int(len(view.episodes)), C.int(view.episodeStart), C.int(view.episodeCursor), C.int(state.Resolution),
		C.double(state.Position), C.double(state.Duration), (*C.char)(view.episodeText), (*C.char)(view.menuText),
	)
}

func (view *View) updateLabelBuffers() {
	view.freeLabelBuffers()
	view.episodeText = unsafe.Pointer(C.CString(joinLines(view.episodes)))
	view.menuText = unsafe.Pointer(C.CString(joinLines(view.menuItems)))
}

func (view *View) freeLabelBuffers() {
	if view.episodeText != nil {
		C.free(view.episodeText)
		view.episodeText = nil
	}
	if view.menuText != nil {
		C.free(view.menuText)
		view.menuText = nil
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
