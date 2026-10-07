// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"errors"
	"testing"

	"animeportable/apps/desktop/backend"
)

func readyPlaybackModel(t *testing.T) *searchModel {
	t.Helper()
	model := &searchModel{}
	item := backend.Anime{ID: "local-anime", Title: "作品"}
	request, _, ok := model.beginDetail(item)
	if !ok || !model.finishDetail(request, backend.Detail{Anime: item}, nil) {
		t.Fatal("preview did not load")
	}
	return model
}

func TestPreviewPlaybackRequiresExplicitRequestAndUsesFirstEpisode(t *testing.T) {
	model := readyPlaybackModel(t)
	if model.requestActive {
		t.Fatal("opening preview started playback")
	}
	request, _, ok := model.beginPlayback()
	if !ok {
		t.Fatal("explicit playback rejected")
	}
	if _, _, duplicate := model.beginPlayback(); duplicate {
		t.Fatal("overlapping episode fetch accepted")
	}
	play, accepted := model.finishPlayback(request, []backend.Episode{{ID: "first"}, {ID: "second"}}, nil)
	if !accepted || play.AnimeID != "local-anime" || play.EpisodeID != "first" || play.StartAt != 0 {
		t.Fatalf("playback request = %#v, accepted %v", play, accepted)
	}
}

func TestPreviewPlaybackRejectsLateCompletionAfterNavigationOrClose(t *testing.T) {
	for _, closeModel := range []bool{false, true} {
		model := readyPlaybackModel(t)
		request, ctx, _ := model.beginPlayback()
		if closeModel {
			model.close()
		} else {
			model.cancel()
		}
		if ctx.Err() == nil {
			t.Fatal("episode fetch was not canceled")
		}
		if _, accepted := model.finishPlayback(request, []backend.Episode{{ID: "first"}}, nil); accepted {
			t.Fatal("late completion started playback")
		}
		if model.requestActive {
			t.Fatal("canceled request acknowledgment did not release physical slot")
		}
	}
}

func TestPreviewPlaybackDoesNotPlayMissingOrFailedEpisodes(t *testing.T) {
	for _, scenario := range []struct {
		episodes []backend.Episode
		err      error
	}{{}, {episodes: []backend.Episode{{}}}, {err: errors.New("source failed")}} {
		model := readyPlaybackModel(t)
		request, _, _ := model.beginPlayback()
		if _, accepted := model.finishPlayback(request, scenario.episodes, scenario.err); accepted {
			t.Fatal("missing or failed episode list started playback")
		}
		if !model.playbackFailed || model.phase != searchPhaseFailed {
			t.Fatal("failure was not visible")
		}
		if _, _, retry := model.beginPlayback(); !retry {
			t.Fatal("explicit retry was blocked")
		}
		model.close()
	}
}
