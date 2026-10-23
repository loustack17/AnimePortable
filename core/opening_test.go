// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

type openingStore struct {
	fakeStore
	title    string
	metadata *AnimeMetadata
}

func (store *openingStore) Anime(_ context.Context, id AnimeID) (Anime, error) {
	return Anime{ID: id, Title: store.title}, nil
}

func (store *openingStore) Metadata(context.Context, AnimeID) (AnimeMetadata, error) {
	if store.metadata == nil {
		return AnimeMetadata{}, ErrNotFound
	}
	return *store.metadata, nil
}

type scriptedMyAnimeList struct {
	candidates []MetadataCandidate
	id         int
	idErr      error
	queries    []MetadataQuery
	refs       []MetadataRef
}

func (source *scriptedMyAnimeList) Search(_ context.Context, query MetadataQuery) ([]MetadataCandidate, error) {
	source.queries = append(source.queries, query)
	return source.candidates, nil
}

func (source *scriptedMyAnimeList) MyAnimeListID(_ context.Context, ref MetadataRef) (int, error) {
	source.refs = append(source.refs, ref)
	return source.id, source.idErr
}

func frierenOnAniList() []MetadataCandidate {
	return []MetadataCandidate{
		{Ref: MetadataRef{Provider: "anilist", ID: "154587"}, Title: "Sousou no Frieren", NativeTitle: "葬送のフリーレン", Year: 2023, EpisodeCount: 28},
		{Ref: MetadataRef{Provider: "anilist", ID: "170068"}, Title: "Sousou no Frieren: ●● no Mahou", NativeTitle: "葬送のフリーレン ～●●の魔法～", Year: 2023},
	}
}

func TestMyAnimeListIDSearchesWithTheStoredJapaneseTitle(t *testing.T) {
	store := &openingStore{title: "葬送的芙莉蓮", metadata: &AnimeMetadata{Ref: MetadataRef{Provider: "bangumi", ID: "400602"}, Title: "葬送的芙莉莲", NativeTitle: "葬送のフリーレン", Year: 2023, EpisodeCount: 28}}
	source := &scriptedMyAnimeList{candidates: frierenOnAniList(), id: 52991}
	id, settled, err := NewApp(nil, nil, store).MyAnimeListID(context.Background(), "anime", source)
	if err != nil || id != 52991 || !settled {
		t.Fatal(id, settled, err)
	}
	if want := (MetadataQuery{NativeTitle: "葬送のフリーレン", Year: 2023, EpisodeCount: 28}); len(source.queries) != 1 || source.queries[0] != want {
		t.Fatalf("queries = %+v", source.queries)
	}
	if len(source.refs) != 1 || source.refs[0].ID != "154587" {
		t.Fatalf("refs = %+v", source.refs)
	}
}

func TestMyAnimeListIDWithoutStoredMetadataSearchesTheTitle(t *testing.T) {
	source := &scriptedMyAnimeList{candidates: []MetadataCandidate{{Ref: MetadataRef{Provider: "anilist", ID: "1"}, Title: "Frieren"}}, id: 7}
	id, settled, err := NewApp(nil, nil, &openingStore{title: "Frieren"}).MyAnimeListID(context.Background(), "anime", source)
	if err != nil || id != 7 || !settled || source.queries[0].Title != "Frieren" {
		t.Fatal(id, settled, err, source.queries)
	}
	// A miss on the Anime1 title is not settled: metadata stored later may find the id.
	id, settled, err = NewApp(nil, nil, &openingStore{title: "葬送的芙莉蓮"}).MyAnimeListID(context.Background(), "anime", &scriptedMyAnimeList{candidates: frierenOnAniList(), id: 7})
	if err != nil || id != 0 || settled {
		t.Fatal(id, settled, err)
	}
}

func TestMyAnimeListIDIsZeroWithoutAnAcceptedMatch(t *testing.T) {
	for name, source := range map[string]*scriptedMyAnimeList{
		"no candidate":   {},
		"other work":     {candidates: []MetadataCandidate{{Ref: MetadataRef{Provider: "anilist", ID: "1"}, Title: "Dungeon Meshi", NativeTitle: "ダンジョン飯"}}, id: 9},
		"no idMal":       {candidates: frierenOnAniList(), idErr: ErrNotFound},
		"two same names": {candidates: []MetadataCandidate{{Ref: MetadataRef{Provider: "anilist", ID: "1"}, NativeTitle: "葬送のフリーレン"}, {Ref: MetadataRef{Provider: "anilist", ID: "2"}, NativeTitle: "葬送のフリーレン", Year: 2030}}, id: 9},
	} {
		store := &openingStore{metadata: &AnimeMetadata{NativeTitle: "葬送のフリーレン"}}
		id, settled, err := NewApp(nil, nil, store).MyAnimeListID(context.Background(), "anime", source)
		if err != nil || id != 0 || !settled {
			t.Errorf("%s: %d, settled %t, %v", name, id, settled, err)
		}
	}
	failing := &scriptedMyAnimeList{candidates: frierenOnAniList(), idErr: errors.New("down")}
	store := &openingStore{metadata: &AnimeMetadata{NativeTitle: "葬送のフリーレン", Year: 2023}}
	if _, settled, err := NewApp(nil, nil, store).MyAnimeListID(context.Background(), "anime", failing); err == nil || settled {
		t.Fatal("a failed lookup must be an error, so it is tried again")
	}
}

type scriptedOpenings struct {
	candidates []OpeningCandidate
	calls      []int
}

func (source *scriptedOpenings) Openings(_ context.Context, myAnimeListID, episode int, _ time.Duration) ([]OpeningCandidate, error) {
	source.calls = append(source.calls, myAnimeListID, episode)
	return source.candidates, nil
}

func TestFindOpeningAsksForTheEpisodeNumber(t *testing.T) {
	store := &openingStore{}
	store.episodeMappings = map[AnimeID][]EpisodeMapping{"anime": {
		{EpisodeID: "e1", Number: "1"}, {EpisodeID: "e7", Number: "07"}, {EpisodeID: "e7.5", Number: "7.5"}, {EpisodeID: "sp", Number: "SP"},
	}}
	opening := Opening{Start: 90 * time.Second, End: 180 * time.Second}
	source := &scriptedOpenings{candidates: []OpeningCandidate{{Opening: opening, EpisodeLength: 1420 * time.Second}}}
	app := NewApp(nil, nil, store)
	got, ok, err := app.FindOpening(context.Background(), "anime", "e7", 52991, 1420*time.Second, source)
	if err != nil || !ok || got != opening || len(source.calls) != 2 || source.calls[0] != 52991 || source.calls[1] != 7 {
		t.Fatal(got, ok, err, source.calls)
	}
	for _, episode := range []EpisodeID{"e7.5", "sp", "missing"} {
		if _, ok, err := app.FindOpening(context.Background(), "anime", episode, 52991, 1420*time.Second, source); ok || err != nil {
			t.Errorf("%s: ok %t, %v", episode, ok, err)
		}
	}
	if len(source.calls) != 2 {
		t.Fatalf("episodes without a whole number were looked up: %v", source.calls)
	}
}

func TestChooseOpeningUsesTheClosestValidCut(t *testing.T) {
	length := 1420 * time.Second
	candidate := func(start, end, cut int) OpeningCandidate {
		return OpeningCandidate{Opening: Opening{Start: time.Duration(start) * time.Second, End: time.Duration(end) * time.Second}, EpisodeLength: time.Duration(cut) * time.Second}
	}
	tests := map[string]struct {
		candidates []OpeningCandidate
		want       Opening
		ok         bool
	}{
		"closest":        {[]OpeningCandidate{candidate(10, 100, 1424), candidate(20, 110, 1421), candidate(30, 120, 1418)}, Opening{20 * time.Second, 110 * time.Second}, true},
		"other cut":      {[]OpeningCandidate{candidate(10, 100, 1440)}, Opening{}, false},
		"unknown cut":    {[]OpeningCandidate{candidate(10, 100, 0)}, Opening{}, false},
		"empty":          {nil, Opening{}, false},
		"backwards":      {[]OpeningCandidate{candidate(100, 10, 1420)}, Opening{}, false},
		"past the end":   {[]OpeningCandidate{candidate(1400, 1430, 1420)}, Opening{}, false},
		"too long":       {[]OpeningCandidate{candidate(0, 241, 1420)}, Opening{}, false},
		"invalid first":  {[]OpeningCandidate{candidate(0, 300, 1420), candidate(5, 95, 1423)}, Opening{5 * time.Second, 95 * time.Second}, true},
		"starts at zero": {[]OpeningCandidate{candidate(0, 90, 1420)}, Opening{0, 90 * time.Second}, true},
	}
	for name, test := range tests {
		got, ok := ChooseOpening(test.candidates, length)
		if got != test.want || ok != test.ok {
			t.Errorf("%s: %+v, %t", name, got, ok)
		}
	}
}
