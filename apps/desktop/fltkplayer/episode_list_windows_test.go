//go:build windows && cgo

package fltkplayer

import (
	"fmt"
	"testing"
)

func numberedEpisodes(count int) []string {
	labels := make([]string, count)
	for index := range labels {
		labels[index] = fmt.Sprintf("第 %02d 集", index)
	}
	return labels
}

func TestOpeningEpisodeListShowsPlayingEpisode(t *testing.T) {
	view := &View{episodes: numberedEpisodes(13), state: State{Episode: 11}}
	view.activate(focusEpisodes)
	if !view.episodesOpen || view.episodeCursor != 11 {
		t.Fatalf("open=%v cursor=%d, want open on 11", view.episodesOpen, view.episodeCursor)
	}
	if view.episodeStart > 11 || view.episodeStart+episodeRows <= 11 {
		t.Fatalf("window %d..%d hides episode 11", view.episodeStart, view.episodeStart+episodeRows-1)
	}
	if view.episodeStart != 5 {
		t.Fatalf("window starts at %d, want 5 (clamped so the last rows fill the list)", view.episodeStart)
	}
	view.activate(focusEpisodes)
	if view.episodesOpen {
		t.Fatal("second activation did not close the list")
	}
}

func TestEpisodeListOpensOnEpisodeAutoplayMovedTo(t *testing.T) {
	view := &View{episodes: numberedEpisodes(30), state: State{Episode: 2}}
	view.SetState(State{Episode: 20})
	view.activate(focusEpisodes)
	if view.episodeCursor != 20 || view.episodeStart > 20 || view.episodeStart+episodeRows <= 20 {
		t.Fatalf("cursor=%d window=%d, want episode 20 visible", view.episodeCursor, view.episodeStart)
	}
	view.activate(focusEpisodes)
	view.SetEpisodes(numberedEpisodes(30), 25)
	view.activate(focusEpisodes)
	if view.episodeCursor != 25 || view.episodeStart > 25 || view.episodeStart+episodeRows <= 25 {
		t.Fatalf("after SetEpisodes cursor=%d window=%d, want episode 25 visible", view.episodeCursor, view.episodeStart)
	}
}

func TestEpisodeStartForClampsAtBothEnds(t *testing.T) {
	for _, test := range []struct{ playing, count, want int }{
		{0, 30, 0},
		{2, 30, 0},
		{15, 30, 12},
		{29, 30, 22},
		{3, 5, 0},
		{0, 0, 0},
	} {
		if got := episodeStartFor(test.playing, test.count); got != test.want {
			t.Errorf("episodeStartFor(%d, %d) = %d, want %d", test.playing, test.count, got, test.want)
		}
	}
}

func TestEpisodeThumbFollowsWindow(t *testing.T) {
	if _, height := episodeThumb(8, 0, episodeTrackHeight); height != 0 {
		t.Fatalf("8 episodes drew a scroll bar of height %d", height)
	}
	top, height := episodeThumb(32, 0, episodeTrackHeight)
	if top != 0 || height != episodeTrackHeight/4 {
		t.Fatalf("thumb at start = %d,%d, want 0,%d", top, height, episodeTrackHeight/4)
	}
	top, height = episodeThumb(32, 24, episodeTrackHeight)
	if top+height != episodeTrackHeight {
		t.Fatalf("thumb at end = %d,%d, want it to touch the track bottom %d", top, height, episodeTrackHeight)
	}
	if _, height = episodeThumb(500, 0, episodeTrackHeight); height != episodeThumbMin {
		t.Fatalf("long list thumb height = %d, want minimum %d", height, episodeThumbMin)
	}
}

func TestEpisodeBarClickCentresWindow(t *testing.T) {
	if got := episodeStartAtTrack(episodeListTop, 32); got != 0 {
		t.Fatalf("click at track top = %d, want 0", got)
	}
	if got := episodeStartAtTrack(episodeListTop+episodeTrackHeight+episodeRowsOffset, 32); got != 24 {
		t.Fatalf("click at track bottom = %d, want 24", got)
	}
	mid := episodeStartAtTrack(episodeListTop+episodeRowsOffset+episodeTrackHeight/2, 32)
	if mid < 11 || mid > 13 {
		t.Fatalf("click at track middle = %d, want about 12", mid)
	}
}

func TestEpisodeClickMapsToRowsWithAndWithoutBar(t *testing.T) {
	const width = 1000
	left := episodeListLeft(width)
	firstRowY := episodeListTop + episodeRowsOffset
	for _, test := range []struct {
		name         string
		x, y         int
		start, count int
		want         int
		ok           bool
	}{
		{"first row", left + 20, firstRowY, 5, 13, 5, true},
		{"row boundary", left + 20, firstRowY + episodeRowHeight, 5, 13, 6, true},
		{"last row", left + 20, firstRowY + episodeRowHeight*8 - 1, 5, 13, 12, true},
		{"below rows", left + 20, firstRowY + episodeRowHeight*8, 5, 13, 0, false},
		{"panel padding above rows", left + 20, episodeListTop, 5, 13, 0, false},
		{"scroll bar strip", left + episodeListWidth - 5, firstRowY + 40, 5, 13, 0, false},
		{"left of panel", left - 1, firstRowY, 5, 13, 0, false},
		{"short list uses full width", left + episodeListWidth - 5, firstRowY, 0, 5, 0, true},
		{"short list ends after its rows", left + 20, firstRowY + episodeRowHeight*5, 0, 5, 0, false},
	} {
		got, ok := episodeRowAt(test.x, test.y, width, test.start, test.count)
		if ok != test.ok || (ok && got != test.want) {
			t.Errorf("%s: episodeRowAt = %d,%v, want %d,%v", test.name, got, ok, test.want, test.ok)
		}
	}
	if !episodeInBar(left+episodeListWidth-5, firstRowY, width, 13) || episodeInBar(left+20, firstRowY, width, 13) || episodeInBar(left+episodeListWidth-5, firstRowY, width, 5) {
		t.Fatal("episodeInBar must hit only the bar strip of a list long enough to scroll")
	}
}
