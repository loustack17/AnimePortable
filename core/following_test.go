// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func numberedMappings(animeID AnimeID, numbers map[EpisodeID]string) []EpisodeMapping {
	animeRef := SourceRef{Provider: "source", ID: string(animeID)}
	mappings := make([]EpisodeMapping, 0, len(numbers))
	for id, number := range numbers {
		mappings = append(mappings, EpisodeMapping{AnimeID: animeID, EpisodeID: id, Ref: EpisodeRef{Anime: animeRef, ID: "provider-" + string(id)}, Number: number})
	}
	return mappings
}

func played(animeID AnimeID, episodeID EpisodeID, at int64) HistoryEntry {
	return HistoryEntry{Progress: PlaybackProgress{AnimeID: animeID, EpisodeID: episodeID}, LastPlayedAt: time.Unix(at, 0)}
}

func TestAppListFollowingIndicator(t *testing.T) {
	numbers := map[EpisodeID]string{"ep-1": "1", "ep-2": "2", "ep-10": "10", "special": "SP"}
	tests := []struct {
		name    string
		history []HistoryEntry
		want    FollowingEntry
	}{
		{
			name:    "watched latest",
			history: []HistoryEntry{played("anime", "ep-10", 1)},
			want:    FollowingEntry{AnimeID: "anime", LatestAvailable: "ep-10", LatestAvailableNumber: "10", LatestWatched: "ep-10", LatestWatchedNumber: "10", HasAvailable: true, HasWatched: true},
		},
		{
			name:    "watched older",
			history: []HistoryEntry{played("anime", "ep-2", 1)},
			want:    FollowingEntry{AnimeID: "anime", LatestAvailable: "ep-10", LatestAvailableNumber: "10", LatestWatched: "ep-2", LatestWatchedNumber: "2", HasAvailable: true, HasWatched: true, NewEpisode: true},
		},
		{
			name: "never watched",
			want: FollowingEntry{AnimeID: "anime", LatestAvailable: "ep-10", LatestAvailableNumber: "10", HasAvailable: true},
		},
		{
			name:    "rewatching an older episode keeps the highest watched",
			history: []HistoryEntry{played("anime", "ep-10", 1), played("anime", "ep-1", 2)},
			want:    FollowingEntry{AnimeID: "anime", LatestAvailable: "ep-10", LatestAvailableNumber: "10", LatestWatched: "ep-10", LatestWatchedNumber: "10", HasAvailable: true, HasWatched: true},
		},
		{
			name:    "only a special watched falls back to the most recent",
			history: []HistoryEntry{played("anime", "special", 1)},
			want:    FollowingEntry{AnimeID: "anime", LatestAvailable: "ep-10", LatestAvailableNumber: "10", LatestWatched: "special", LatestWatchedNumber: "SP", HasAvailable: true, HasWatched: true, NewEpisode: true},
		},
		{
			name:    "other anime history is ignored",
			history: []HistoryEntry{played("other", "ep-10", 1)},
			want:    FollowingEntry{AnimeID: "anime", LatestAvailable: "ep-10", LatestAvailableNumber: "10", HasAvailable: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeStore{
				following:       []AnimeID{"anime"},
				episodeMappings: map[AnimeID][]EpisodeMapping{"anime": numberedMappings("anime", numbers)},
				history:         test.history,
			}
			// A nil source proves the list never needs the network.
			app := NewApp(nil, &fakePlayer{}, store)

			entries, err := app.ListFollowing(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || !reflect.DeepEqual(entries[0], test.want) {
				t.Fatalf("entries = %#v, want %#v", entries, test.want)
			}
		})
	}
}

func TestAppListFollowingOrdersByNumberNotSourceOrder(t *testing.T) {
	// Anime1 lists some series newest first; the latest episode must come from the number.
	animeRef := SourceRef{Provider: "source", ID: "anime"}
	store := &fakeStore{
		following: []AnimeID{"anime"},
		episodeMappings: map[AnimeID][]EpisodeMapping{"anime": {
			{AnimeID: "anime", EpisodeID: "newest", Ref: EpisodeRef{Anime: animeRef, ID: "a"}, Number: "12"},
			{AnimeID: "anime", EpisodeID: "middle", Ref: EpisodeRef{Anime: animeRef, ID: "b"}, Number: "11.5"},
			{AnimeID: "anime", EpisodeID: "oldest", Ref: EpisodeRef{Anime: animeRef, ID: "c"}, Number: "9"},
		}},
	}
	entries, err := NewApp(nil, &fakePlayer{}, store).ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].LatestAvailable != "newest" || entries[0].LatestAvailableNumber != "12" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestAppListFollowingNewEpisodeComparesNumbersNotIDs(t *testing.T) {
	// The same episode number under two local IDs, history on only one of them.
	for _, watchedID := range []EpisodeID{"a-13", "z-13"} {
		store := &fakeStore{
			following:       []AnimeID{"anime"},
			episodeMappings: map[AnimeID][]EpisodeMapping{"anime": numberedMappings("anime", map[EpisodeID]string{"a-13": "13", "z-13": "13", "ep-12": "12"})},
			history:         []HistoryEntry{played("anime", watchedID, 1)},
		}
		entries, err := NewApp(nil, &fakePlayer{}, store).ListFollowing(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].NewEpisode || entries[0].LatestAvailableNumber != "13" {
			t.Fatalf("watched %s: entries = %#v, want episode 13 seen as watched", watchedID, entries)
		}
	}
}

func TestAppListFollowingWithoutNumbersHasNoLatest(t *testing.T) {
	// Mappings stored before episode numbers were kept, or specials only.
	store := &fakeStore{
		following:       []AnimeID{"anime"},
		episodeMappings: map[AnimeID][]EpisodeMapping{"anime": numberedMappings("anime", map[EpisodeID]string{"legacy": "", "special": "OVA"})},
		history:         []HistoryEntry{played("anime", "legacy", 1)},
	}
	entries, err := NewApp(nil, &fakePlayer{}, store).ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := FollowingEntry{AnimeID: "anime", LatestWatched: "legacy", HasWatched: true}
	if len(entries) != 1 || !reflect.DeepEqual(entries[0], want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
}

func TestAppListFollowingUsesAnyNumberedMappingOfAnEpisode(t *testing.T) {
	animeRef := SourceRef{Provider: "source", ID: "anime"}
	store := &fakeStore{
		following: []AnimeID{"anime"},
		episodeMappings: map[AnimeID][]EpisodeMapping{"anime": {
			{AnimeID: "anime", EpisodeID: "episode", Ref: EpisodeRef{Anime: animeRef, ID: "numbered"}, Number: "3"},
			{AnimeID: "anime", EpisodeID: "episode", Ref: EpisodeRef{Anime: SourceRef{Provider: "other", ID: "anime"}, ID: "legacy"}},
		}},
	}
	entries, err := NewApp(nil, &fakePlayer{}, store).ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].LatestAvailable != "episode" || entries[0].LatestAvailableNumber != "3" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestAppListFollowingSkipsDeletedAnimeAndPreservesStoreOrder(t *testing.T) {
	store := &fakeStore{following: []AnimeID{"first", "second", "third"}}
	entries, err := NewApp(nil, &fakePlayer{}, store).ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := make([]AnimeID, len(entries))
	for index, entry := range entries {
		got[index] = entry.AnimeID
	}
	if !reflect.DeepEqual(got, store.following) {
		t.Fatalf("following order = %#v, want %#v", got, store.following)
	}

	missing := &fakeStore{following: []AnimeID{"gone"}, animeErr: ErrNotFound}
	entries, err = NewApp(nil, &fakePlayer{}, missing).ListFollowing(context.Background())
	if err != nil || len(entries) != 0 {
		t.Fatalf("entries for deleted anime = %#v, %v", entries, err)
	}
}

func TestAppListFollowingPropagatesLocalErrors(t *testing.T) {
	expected := errors.New("local failure")
	tests := []struct {
		name  string
		store *fakeStore
	}{
		{name: "anime", store: &fakeStore{animeErr: expected, following: []AnimeID{"anime"}}},
		{name: "episode mappings", store: &fakeStore{episodeMappingsErr: expected, following: []AnimeID{"anime"}}},
		{name: "history", store: &fakeStore{historyErr: expected, following: []AnimeID{"anime"}}},
		{name: "following", store: &fakeStore{followingErr: expected}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewApp(nil, &fakePlayer{}, test.store).ListFollowing(context.Background()); !errors.Is(err, expected) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestAppListFollowingStopsWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := &fakeStore{following: []AnimeID{"anime"}}
	if _, err := NewApp(nil, &fakePlayer{}, store).ListFollowing(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestEpisodeNumberValue(t *testing.T) {
	for number, want := range map[string]bool{"12": true, " 7 ": true, "12.5": true, "SP": false, "": false, "NaN": false, "Inf": false} {
		if _, ok := EpisodeNumberValue(number); ok != want {
			t.Errorf("EpisodeNumberValue(%q) ok = %v, want %v", number, ok, want)
		}
	}
}
