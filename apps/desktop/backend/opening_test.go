// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"testing"
	"time"

	"animeportable/core"
)

type countingMyAnimeList struct {
	id       int
	err      error
	searches int
}

func (source *countingMyAnimeList) Search(context.Context, core.MetadataQuery) ([]core.MetadataCandidate, error) {
	source.searches++
	if source.err != nil {
		return nil, source.err
	}
	return []core.MetadataCandidate{{Ref: core.MetadataRef{Provider: "anilist", ID: "1"}, NativeTitle: "葬送のフリーレン"}}, nil
}

func (source *countingMyAnimeList) MyAnimeListID(context.Context, core.MetadataRef) (int, error) {
	return source.id, nil
}

type recordedOpenings struct {
	lengths []time.Duration
	episode []int
}

func (source *recordedOpenings) Openings(_ context.Context, _ int, episode int, length time.Duration) ([]core.OpeningCandidate, error) {
	source.lengths = append(source.lengths, length)
	source.episode = append(source.episode, episode)
	return []core.OpeningCandidate{{Opening: core.Opening{Start: 90 * time.Second, End: 180 * time.Second}, EpisodeLength: length}}, nil
}

func openingService(myAnimeList *countingMyAnimeList, openings *recordedOpenings) *Service {
	store := newFakeStore()
	store.anime["anime"] = core.Anime{ID: "anime", Title: "葬送的芙莉蓮"}
	store.metadata["anime"] = core.AnimeMetadata{Ref: core.MetadataRef{Provider: "bangumi", ID: "400602"}, NativeTitle: "葬送のフリーレン"}
	store.mappings["anime"] = []core.EpisodeMapping{{EpisodeID: "e1", Number: "1"}, {EpisodeID: "e2", Number: "2"}}
	return newWithDependencies(dependencies{store: store, source: &fakeSource{}, myAnimeList: myAnimeList, openings: openings})
}

func TestOpeningTimesLooksUpTheMyAnimeListIDOncePerRun(t *testing.T) {
	myAnimeList, openings := &countingMyAnimeList{id: 52991}, &recordedOpenings{}
	service := openingService(myAnimeList, openings)
	for _, episode := range []string{"e1", "e2"} {
		got, err := service.OpeningTimes(context.Background(), "anime", episode, 1420500)
		if err != nil || got != (Opening{Found: true, Start: 90000, End: 180000}) {
			t.Fatal(episode, got, err)
		}
	}
	if myAnimeList.searches != 1 || len(openings.episode) != 2 || openings.episode[1] != 2 || openings.lengths[0] != 1420500*time.Millisecond {
		t.Fatalf("searches %d, openings %v %v", myAnimeList.searches, openings.episode, openings.lengths)
	}
}

func TestOpeningTimesRemembersAnAnimeWithoutMyAnimeListID(t *testing.T) {
	myAnimeList, openings := &countingMyAnimeList{}, &recordedOpenings{}
	service := openingService(myAnimeList, openings)
	for range 2 {
		if got, err := service.OpeningTimes(context.Background(), "anime", "e1", 1420000); err != nil || got.Found {
			t.Fatal(got, err)
		}
	}
	if myAnimeList.searches != 1 || len(openings.episode) != 0 {
		t.Fatalf("searches %d, openings %v", myAnimeList.searches, openings.episode)
	}
}

func TestOpeningTimesTriesAgainAfterAFailedLookup(t *testing.T) {
	myAnimeList := &countingMyAnimeList{id: 52991, err: errors.New("offline")}
	service := openingService(myAnimeList, &recordedOpenings{})
	if _, err := service.OpeningTimes(context.Background(), "anime", "e1", 1420000); err == nil {
		t.Fatal("a failed lookup was not reported")
	}
	myAnimeList.err = nil
	if got, err := service.OpeningTimes(context.Background(), "anime", "e1", 1420000); err != nil || !got.Found || myAnimeList.searches != 2 {
		t.Fatal(got, err, myAnimeList.searches)
	}
}

func TestOpeningTimesRefusesInvalidInput(t *testing.T) {
	myAnimeList := &countingMyAnimeList{id: 52991}
	service := openingService(myAnimeList, &recordedOpenings{})
	for _, input := range []struct {
		anime, episode string
		length         int64
	}{{"", "e1", 1000}, {"anime", "", 1000}, {"anime", "e1", 0}, {"anime", "e1", -1}} {
		if _, err := service.OpeningTimes(context.Background(), input.anime, input.episode, input.length); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%+v: %v", input, err)
		}
	}
	if myAnimeList.searches != 0 {
		t.Fatal("invalid input reached a provider")
	}
}

func TestOpeningTimesAsksAgainOnceMetadataWithAJapaneseTitleIsStored(t *testing.T) {
	myAnimeList := &countingMyAnimeList{}
	service := openingService(myAnimeList, &recordedOpenings{})
	store := service.store.(writeGuard).Store.(*fakeStore)
	delete(store.metadata, "anime")
	if got, err := service.OpeningTimes(context.Background(), "anime", "e1", 1420000); err != nil || got.Found {
		t.Fatal(got, err)
	}
	store.metadata["anime"] = core.AnimeMetadata{Ref: core.MetadataRef{Provider: "bangumi", ID: "400602"}, NativeTitle: "葬送のフリーレン"}
	myAnimeList.id = 52991
	if got, err := service.OpeningTimes(context.Background(), "anime", "e1", 1420000); err != nil || !got.Found || myAnimeList.searches != 2 {
		t.Fatal(got, err, myAnimeList.searches)
	}
}
