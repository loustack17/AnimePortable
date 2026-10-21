// SPDX-License-Identifier: MPL-2.0

package main

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"testing"

	"animeportable/apps/desktop/backend"
)

type playerFollowTestService struct {
	following  []string
	err        error
	changeErr  error // fails Follow and Unfollow only
	followed   []string
	unfollowed []string
}

func (service *playerFollowTestService) Following(context.Context) ([]backend.Following, error) {
	result := []backend.Following{}
	for _, animeID := range service.following {
		result = append(result, backend.Following{AnimeID: animeID})
	}
	return result, service.err
}

func (service *playerFollowTestService) Follow(_ context.Context, animeID string) error {
	if err := cmp.Or(service.err, service.changeErr); err != nil {
		return err
	}
	service.followed = append(service.followed, animeID)
	service.following = append(service.following, animeID)
	return nil
}

func (service *playerFollowTestService) Unfollow(_ context.Context, animeID string) error {
	if err := cmp.Or(service.err, service.changeErr); err != nil {
		return err
	}
	service.unfollowed = append(service.unfollowed, animeID)
	service.following = slices.DeleteFunc(service.following, func(id string) bool { return id == animeID })
	return nil
}

type playerFollowTestStar struct {
	known, following bool
	failures         []error
	playing          string
	works, updates   []func()
}

// newPlayerFollowTest queues background work and UI updates; run drains both, and a test
// can also run them one by one in any order to replay a race.
func newPlayerFollowTest(service *playerFollowTestService) (*playerFollow, *playerFollowTestStar, func()) {
	star := &playerFollowTestStar{}
	follow := &playerFollow{
		ctx:     context.Background(),
		service: service,
		start:   func(work func()) { star.works = append(star.works, work) },
		awake:   func(update func()) { star.updates = append(star.updates, update) },
		playing: func() string { return star.playing },
		show:    func(known, following bool) { star.known, star.following = known, following },
		fail:    func(err error) { star.failures = append(star.failures, err) },
	}
	run := func() {
		for len(star.works)+len(star.updates) > 0 {
			works, updates := star.works, star.updates
			star.works, star.updates = nil, nil
			for _, work := range works {
				work()
			}
			for _, update := range updates {
				update()
			}
		}
	}
	return follow, star, run
}

func TestPlayerFollowShowsStoredStateForThePlayedAnime(t *testing.T) {
	service := &playerFollowTestService{following: []string{"followed"}}
	follow, star, run := newPlayerFollowTest(service)

	star.playing = "followed"
	follow.load("followed")
	run()
	if !star.known || !star.following {
		t.Fatalf("followed anime: known %v, following %v", star.known, star.following)
	}
	star.playing = "other"
	follow.load("other")
	run()
	if !star.known || star.following {
		t.Fatalf("unfollowed anime: known %v, following %v", star.known, star.following)
	}

	// A result for an anime no longer playing is dropped.
	star.known = false
	follow.load("followed")
	run()
	if star.known {
		t.Fatal("state of an anime that is no longer playing was shown")
	}

	// A failed read leaves the star hidden; there is nothing to report.
	service.err = errors.New("database locked")
	follow.load("other")
	run()
	if star.known || len(star.failures) != 0 {
		t.Fatalf("failed read: known %v, failures %v", star.known, star.failures)
	}
}

func TestPlayerFollowTogglesOneChangeAtATime(t *testing.T) {
	service := &playerFollowTestService{}
	follow, star, run := newPlayerFollowTest(service)
	star.playing = "anime"
	star.known = true

	follow.toggle("anime", false)
	follow.toggle("anime", false)
	run()
	if !slices.Equal(service.followed, []string{"anime"}) || !star.following {
		t.Fatalf("followed = %#v, star following %v; want one Follow", service.followed, star.following)
	}
	follow.toggle("anime", true)
	run()
	if !slices.Equal(service.unfollowed, []string{"anime"}) || star.following || !star.known {
		t.Fatalf("unfollowed = %#v, star following %v known %v", service.unfollowed, star.following, star.known)
	}

	// A failure is reported and the star keeps its state.
	service.err = errors.New("database locked")
	follow.toggle("anime", false)
	run()
	if len(star.failures) != 1 || star.following {
		t.Fatalf("failures %v, following %v", star.failures, star.following)
	}

	// A change that lands after playback moved to another anime does not touch its star.
	service.err = nil
	follow.toggle("anime", false)
	star.playing = "other"
	run()
	if star.following {
		t.Fatal("another anime's star changed")
	}

	// A load dropped because a change was pending is redone when that change shows nothing:
	// it failed, or playback moved to another anime.
	for _, moved := range []bool{false, true} {
		service.changeErr = errors.New("database locked")
		if moved {
			service.changeErr = nil
		}
		star.playing, star.known = "anime", true
		follow.toggle("anime", star.following)
		if moved {
			star.playing = "other"
		}
		star.known = false // play() hides the star before it loads the state
		follow.load(star.playing)
		run()
		if !star.known {
			t.Fatalf("moved %v: star stays hidden after a pending change", moved)
		}
	}
	service.changeErr = nil
	star.failures = nil

	// A load that read the list before a change committed does not undo the change, whether
	// it started before the change or while it was pending, and whichever update lands last.
	star.playing = "anime"
	follow.toggle("anime", true)
	run()
	for _, loadFirst := range []bool{true, false} {
		if loadFirst {
			follow.load("anime")
			follow.toggle("anime", star.following)
		} else {
			follow.toggle("anime", star.following)
			follow.load("anime")
		}
		want := !star.following
		load, change := star.works[0], star.works[1]
		if !loadFirst {
			load, change = change, load
		}
		load()   // reads the list before the change commits
		change() // commits and queues the change's update
		loadUpdate, changeUpdate := star.updates[0], star.updates[1]
		star.works, star.updates = nil, nil
		changeUpdate()
		loadUpdate()
		if !star.known || star.following != want {
			t.Fatalf("load first %v: known %v, following %v, want %v", loadFirst, star.known, star.following, want)
		}
	}
}
