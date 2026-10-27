// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"errors"
	"fmt"
	"testing"

	"animeportable/apps/desktop/backend"
)

func TestDetailModelKeepsEpisodeOrderAndBoundsPages(t *testing.T) {
	model := detailModel{}
	model.open("anime")
	episodes := make([]backend.Episode, detailEpisodePageSize*2+1)
	for index := range episodes {
		episodes[index] = backend.Episode{ID: fmt.Sprintf("opaque-%02d", index), Number: fmt.Sprintf("%02d", 50-index)}
	}
	model.setEpisodes(episodes)
	if got := len(model.visibleEpisodes()); got != detailEpisodePageSize {
		t.Fatalf("first page has %d episodes", got)
	}
	for range len(episodes) - 1 {
		model.moveSelection(1)
	}
	if got := model.selectedEpisodeID(); got != "opaque-16" {
		t.Fatalf("selection followed displayed number instead of source order: %q", got)
	}
	if visible := model.visibleEpisodes(); len(visible) != 1 || visible[0].ID != "opaque-16" {
		t.Fatalf("last page = %#v", visible)
	}
	model.movePage(-1)
	if model.selectedEpisodeID() != "opaque-08" || model.page != 1 {
		t.Fatalf("previous page selection = %q at page %d", model.selectedEpisodeID(), model.page)
	}
}

func TestDetailModelPlayUsesExactSelectedIdentity(t *testing.T) {
	model := detailModel{}
	model.open("anime-id")
	model.setEpisodes([]backend.Episode{{ID: "opaque-a", AnimeID: "anime-id"}, {ID: "opaque-b", AnimeID: "anime-id"}})
	model.selectEpisode(1)
	request, ok := model.playRequest()
	if !ok || request.AnimeID != "anime-id" || request.EpisodeID != "opaque-b" || request.StartAt != 0 {
		t.Fatalf("selected playback request = %#v, accepted %v", request, ok)
	}
}

func TestDetailModelRejectsEpisodeFromAnotherAnime(t *testing.T) {
	model := detailModel{}
	model.open("anime-id")
	model.setEpisodes([]backend.Episode{{ID: "opaque", AnimeID: "other"}})
	if _, ok := model.playRequest(); ok {
		t.Fatal("episode identity from another anime was played")
	}
	model.setHistory([]backend.History{{AnimeID: "anime-id", EpisodeID: "opaque", Position: 1000}})
	if _, ok := model.continueRequest(); ok {
		t.Fatal("episode history from another anime was resumed")
	}
}

func TestDetailModelContinueMatchesAnimeAndEpisodeAndRejectsCompleted(t *testing.T) {
	model := detailModel{}
	model.open("wanted")
	model.setEpisodes([]backend.Episode{{ID: "completed"}, {ID: "resume"}, {ID: "missing"}})
	model.setHistory([]backend.History{
		{AnimeID: "other", EpisodeID: "resume", Position: 12000, UpdatedAt: "2026-10-03T10:00:00Z"},
		{AnimeID: "wanted", EpisodeID: "completed", Position: 24000, UpdatedAt: "2026-10-04T10:00:00Z", Completed: true},
		{AnimeID: "wanted", EpisodeID: "not-listed", Position: 90000, UpdatedAt: "2026-10-06T10:00:00Z"},
		{AnimeID: "wanted", EpisodeID: "resume", Position: 42750, UpdatedAt: "2026-10-05T10:00:00Z"},
	})
	request, ok := model.continueRequest()
	if !ok || request.AnimeID != "wanted" || request.EpisodeID != "resume" || request.StartAt != 42750 {
		t.Fatalf("continue request = %#v, accepted %v", request, ok)
	}
	model.setHistory([]backend.History{{AnimeID: "wanted", EpisodeID: "completed", Position: 24000, Completed: true}})
	if _, ok := model.continueRequest(); ok {
		t.Fatal("completed history was treated as resumable")
	}
}

func TestDetailModelContinueUsesTheHomeResumableRule(t *testing.T) {
	model := detailModel{}
	model.open("wanted")
	model.setEpisodes([]backend.Episode{{ID: "past-end"}, {ID: "no-duration"}})
	// A position at or past a known duration is not resumable on Home, so the detail page does
	// not offer to continue it either; a position without a known duration still is.
	model.setHistory([]backend.History{
		{AnimeID: "wanted", EpisodeID: "past-end", Position: 24000, Duration: 24000, UpdatedAt: "2026-10-05T10:00:00Z"},
		{AnimeID: "wanted", EpisodeID: "no-duration", Position: 12000, UpdatedAt: "2026-10-04T10:00:00Z"},
	})
	request, ok := model.continueRequest()
	if !ok || request.EpisodeID != "no-duration" || request.StartAt != 12000 {
		t.Fatalf("continue request = %#v, accepted %v", request, ok)
	}
	if resumable(backend.History{Position: 24000, Duration: 24000}) {
		t.Fatal("Home rule changed: a position at the duration is resumable")
	}
}

func TestDetailModelReordersNoEpisodesAndClearsOnClose(t *testing.T) {
	model := detailModel{}
	model.open("anime")
	model.setEpisodes([]backend.Episode{{ID: "a"}, {ID: "b"}})
	model.selectEpisode(1)
	model.setEpisodes([]backend.Episode{{ID: "b"}, {ID: "a"}})
	if model.selectedEpisodeID() != "b" {
		t.Fatalf("selection lost identity after refresh: %q", model.selectedEpisodeID())
	}
	model.close()
	if model.animeID != "" || len(model.visibleEpisodes()) != 0 || len(model.history) != 0 {
		t.Fatalf("closed detail retained state: %#v", model)
	}
	if _, ok := model.playRequest(); ok {
		t.Fatal("closed detail produced playback")
	}
}

func TestDetailModelShowsTheStoredListUntilAnime1Answers(t *testing.T) {
	stored := []backend.Episode{{ID: "episode:1", Number: "01"}, {ID: "episode:2", Number: "02"}}
	fresh := []backend.Episode{{ID: "episode:1", Number: "01"}, {ID: "episode:2", Number: "02"}, {ID: "episode:3", Number: "03"}}
	failure := errors.New("anime1 unreachable")

	model := detailModel{}
	model.open("anime")
	if !model.applyStored(stored) || len(model.episodes) != 2 {
		t.Fatalf("stored list not shown: %#v", model.episodes)
	}
	model.selected = 1
	if failed := model.applyRefresh(fresh, nil); failed || len(model.episodes) != 3 || model.episodes[2].ID != "episode:3" || model.selected != 1 {
		t.Fatalf("refresh: failed=%t episodes=%#v selected=%d", failed, model.episodes, model.selected)
	}
	if model.applyStored(stored) || len(model.episodes) != 3 {
		t.Fatal("a late stored list replaced Anime1's answer")
	}

	model.open("anime")
	model.applyStored(stored)
	if failed := model.applyRefresh(nil, failure); failed || len(model.episodes) != 2 {
		t.Fatalf("failed refresh over a stored list: failed=%t episodes=%d, want the stored list kept", failed, len(model.episodes))
	}

	model.open("never-loaded")
	if model.applyStored(nil) {
		t.Fatal("an empty stored list counted as shown")
	}
	if failed := model.applyRefresh(nil, failure); !failed {
		t.Fatal("failed refresh without a stored list must report the failure")
	}
	model.open("next")
	if model.refreshed || !model.applyStored(stored) {
		t.Fatal("opening another anime kept the previous refresh state")
	}
}
