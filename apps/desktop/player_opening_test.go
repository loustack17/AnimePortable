// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
)

type playerOpeningTestService struct {
	opening backend.Opening
	err     error
	asked   []string
	lengths []int64
}

func (service *playerOpeningTestService) OpeningTimes(_ context.Context, animeID, episodeID string, lengthMillis int64) (backend.Opening, error) {
	service.asked = append(service.asked, animeID+"/"+episodeID)
	service.lengths = append(service.lengths, lengthMillis)
	return service.opening, service.err
}

// newPlayerOpeningTest runs background work and UI updates at once; view is the load on screen.
func newPlayerOpeningTest(service *playerOpeningTestService, view *uint64, shown *[]backend.Opening) *playerOpening {
	return &playerOpening{
		ctx:     context.Background(),
		service: service,
		start:   func(work func()) { work() },
		awake:   func(update func()) { update() },
		view:    func() uint64 { return *view },
		show:    func(found backend.Opening) { *shown = append(*shown, found) },
	}
}

func TestPlayerOpeningAsksOncePerLoadOnceTheLengthIsKnown(t *testing.T) {
	found := backend.Opening{Found: true, Start: 90000, End: 180000}
	service := &playerOpeningTestService{opening: found}
	view := uint64(3)
	var shown []backend.Opening
	opening := newPlayerOpeningTest(service, &view, &shown)
	request := backend.PlayRequest{AnimeID: "anime", EpisodeID: "e1"}
	opening.playing(3, request, 0)
	if len(service.asked) != 0 {
		t.Fatal("asked before the length was known")
	}
	for range 3 {
		opening.playing(3, request, 1420500*time.Millisecond)
	}
	if len(service.asked) != 1 || service.asked[0] != "anime/e1" || service.lengths[0] != 1420500 || len(shown) != 1 || shown[0] != found {
		t.Fatalf("asked %v %v, shown %v", service.asked, service.lengths, shown)
	}
	view = 4
	opening.playing(4, backend.PlayRequest{AnimeID: "anime", EpisodeID: "e2"}, time.Minute)
	if len(service.asked) != 2 || service.asked[1] != "anime/e2" {
		t.Fatalf("the next load was not asked: %v", service.asked)
	}
}

func TestPlayerOpeningShowsNothingForAnotherLoadOrWithoutAnOpening(t *testing.T) {
	view := uint64(1)
	var shown []backend.Opening
	stale := &playerOpeningTestService{opening: backend.Opening{Found: true, Start: 1, End: 2}}
	opening := newPlayerOpeningTest(stale, &view, &shown)
	opening.start = func(work func()) { view = 2; work() } // a new load starts while the lookup runs
	opening.playing(1, backend.PlayRequest{AnimeID: "anime", EpisodeID: "e1"}, time.Minute)
	for name, service := range map[string]*playerOpeningTestService{
		"none":   {},
		"failed": {opening: backend.Opening{Found: true, Start: 1, End: 2}, err: errors.New("offline")},
	} {
		view = 5
		newPlayerOpeningTest(service, &view, &shown).playing(5, backend.PlayRequest{AnimeID: "anime", EpisodeID: "e1"}, time.Minute)
		if len(service.asked) != 1 {
			t.Errorf("%s: asked %v", name, service.asked)
		}
	}
	if len(shown) != 0 {
		t.Fatalf("shown %v", shown)
	}
}

func TestPlayerOpeningCancelsTheLookupOfALoadLeft(t *testing.T) {
	view := uint64(1)
	var shown []backend.Opening
	var works []func()
	// Errors are read when each lookup runs; every lookup cancels its context once it ends.
	var errs []error
	service := &cancelRecordingService{errs: &errs}
	opening := newPlayerOpeningTest(&playerOpeningTestService{}, &view, &shown)
	opening.service = service
	opening.start = func(work func()) { works = append(works, work) }
	opening.playing(1, backend.PlayRequest{AnimeID: "anime", EpisodeID: "e1"}, time.Minute)
	opening.playing(2, backend.PlayRequest{AnimeID: "anime", EpisodeID: "e2"}, time.Minute)
	opening.stop()
	for _, work := range works {
		work()
	}
	if len(errs) != 2 || errs[0] == nil || errs[1] == nil {
		t.Fatalf("lookups of loads left were not cancelled: %v", errs)
	}
	errs = nil
	opening.playing(3, backend.PlayRequest{AnimeID: "anime", EpisodeID: "e3"}, time.Minute)
	works[len(works)-1]()
	if len(errs) != 1 || errs[0] != nil {
		t.Fatalf("the current load's lookup was cancelled: %v", errs)
	}
}

type cancelRecordingService struct{ errs *[]error }

func (service *cancelRecordingService) OpeningTimes(ctx context.Context, _, _ string, _ int64) (backend.Opening, error) {
	*service.errs = append(*service.errs, ctx.Err())
	return backend.Opening{}, ctx.Err()
}
