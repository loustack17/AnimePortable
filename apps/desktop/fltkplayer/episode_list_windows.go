//go:build windows && cgo

package fltkplayer

// The episode list is drawn by overlay_windows.cc with the same numbers; keep both in step.
const (
	episodeRows      = 8
	episodeRowHeight = 34
	episodeListTop   = 58 // panel top; the first row starts 4 px lower
	episodeListWidth = 200
	episodeListRight = 81 // distance from the window's right edge to the panel's right edge
	episodeBarWidth  = 14 // right strip of the panel kept for the scroll bar
	episodeThumbMin  = 24
	// A list long enough for a bar always shows a full window, so the track is a constant height.
	episodeTrackHeight = episodeRowHeight * episodeRows
	episodeRowsOffset  = 4
)

func episodeListLeft(width int) int { return width - episodeListRight - episodeListWidth }

// episodeStartFor places the playing episode in the middle of the visible window so the viewer
// sees both earlier and later episodes around it.
func episodeStartFor(playing, count int) int {
	return clampEpisodeWindow(playing-episodeRows/2+1, count)
}

func clampEpisodeWindow(start, count int) int {
	return max(0, min(start, max(0, count-episodeRows)))
}

// episodeRowAt maps a click to an episode index; ok is false outside the rows and the scroll bar strip.
func episodeRowAt(x, y, width, start, count int) (index int, ok bool) {
	rows := min(count-start, episodeRows)
	left := episodeListLeft(width)
	top := episodeListTop + episodeRowsOffset
	if rows <= 0 || x < left || x >= left+episodeListWidth-barStrip(count) || y < top || y >= top+episodeRowHeight*rows {
		return 0, false
	}
	return start + (y-top)/episodeRowHeight, true
}

// episodeInBar reports whether a click falls in the scroll bar strip of the open list.
func episodeInBar(x, y, width, count int) bool {
	left := episodeListLeft(width)
	return barStrip(count) > 0 && x >= left+episodeListWidth-episodeBarWidth && x < left+episodeListWidth &&
		y >= episodeListTop && y < episodeListTop+episodeTrackHeight+2*episodeRowsOffset
}

// barStrip is the width reserved for the scroll bar; zero when every episode fits.
func barStrip(count int) int {
	if count > episodeRows {
		return episodeBarWidth
	}
	return 0
}

// episodeThumb returns the scroll bar thumb top (relative to the track) and height; height 0 means no bar.
func episodeThumb(count, start, track int) (top, height int) {
	if count <= episodeRows || track <= 0 {
		return 0, 0
	}
	height = max(episodeThumbMin, track*episodeRows/count)
	height = min(height, track)
	return (track - height) * clampEpisodeWindow(start, count) / (count - episodeRows), height
}

// episodeStartAtTrack turns a click on the scroll bar into the window start that centres the thumb there.
func episodeStartAtTrack(y, count int) int {
	track := episodeTrackHeight
	_, height := episodeThumb(count, 0, track)
	if height == 0 || track == height {
		return 0
	}
	return clampEpisodeWindow(((y-episodeListTop-episodeRowsOffset-height/2)*(count-episodeRows)+(track-height)/2)/(track-height), count)
}

// openEpisodes shows the list on the playing episode, including after autoplay moved on.
func (view *View) openEpisodes() {
	view.episodesOpen = true
	view.episodeCursor = view.state.Episode
	view.episodeStart = episodeStartFor(view.state.Episode, len(view.episodes))
}
