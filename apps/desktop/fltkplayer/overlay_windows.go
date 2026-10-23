//go:build windows && cgo

package fltkplayer

/*
#include <stdlib.h>
#cgo LDFLAGS: -lopengl32 -lgdi32
void ap_overlay_draw(int width, int height, int visible, int menu, int episodes, int volume_open, int playing, int paused, int fullscreen, int focus, int menu_cursor, int volume, int episode, int episode_count, int episode_start, int episode_cursor, int thumb_top, int thumb_height, int resolution, int loading, int progress_hover, int scrubbing, int follow_known, int following, int speed_open, int speed_index, int speed_cursor, int speed_count, int skip, int skip_width, double position, double duration, double scrub_position, const char* episode_labels, const char* menu_labels, const char* speed_labels, const char* fixed_labels);
*/
import "C"

import (
	"unsafe"

	"animeportable/apps/desktop/uitext"
	"animeportable/core"
)

func drawPlayerOverlay(width, height int, view *View) {
	state := view.state
	thumbTop, thumbHeight := episodeThumb(len(view.episodes), view.episodeStart, episodeTrackHeight)
	C.ap_overlay_draw(
		C.int(width), C.int(height), C.int(boolInt(view.visible)), C.int(boolInt(view.menuVisible())),
		C.int(boolInt(view.episodesOpen)), C.int(boolInt(view.volumeOpen)), C.int(boolInt(state.Playing)),
		C.int(boolInt(state.Paused)), C.int(boolInt(state.Fullscreen)), C.int(state.Focus), C.int(view.menuCursor),
		C.int(state.Volume), C.int(state.Episode), C.int(len(view.episodes)), C.int(view.episodeStart), C.int(view.episodeCursor), C.int(thumbTop), C.int(thumbHeight), C.int(state.Resolution),
		C.int(boolInt(state.Loading)), C.int(boolInt(view.progressHover)), C.int(boolInt(view.scrubbing)),
		C.int(boolInt(state.FollowKnown)), C.int(boolInt(state.Following)), C.int(boolInt(view.speedOpen)), C.int(speedIndex(view.speedPercentOrDefault())), C.int(view.speedCursor), C.int(len(core.PlaybackSpeedSteps)), C.int(boolInt(state.skipShown())), C.int(view.skipWidth),
		C.double(state.Position), C.double(state.Duration), C.double(view.scrubPosition), (*C.char)(view.episodeText), (*C.char)(view.menuText), (*C.char)(view.speedText), (*C.char)(view.fixedText),
	)
}

func (view *View) updateLabelBuffers() {
	view.freeLabelBuffers()
	view.episodeText = unsafe.Pointer(C.CString(joinLines(view.episodes)))
	view.menuText = unsafe.Pointer(C.CString(joinLines(view.menuItems)))
	view.speedText = unsafe.Pointer(C.CString(joinLines(speedLabels())))
	text := uitext.T()
	view.fixedText = unsafe.Pointer(C.CString(joinLines([]string{text.PlayerEpisodes, text.PlayerLoading, text.PlayerSkipIntro})))
	view.skipWidth = skipButtonWidth(text.PlayerSkipIntro)
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
	if view.speedText != nil {
		C.free(view.speedText)
		view.speedText = nil
	}
	if view.fixedText != nil {
		C.free(view.fixedText)
		view.fixedText = nil
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
