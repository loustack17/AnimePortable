// SPDX-License-Identifier: MPL-2.0

package contract

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"animeportable/core"
)

type PlayerProbe interface {
	Starts() int
	Loads() int
	ActiveSessions() int
	EventCount() int
	Clean() bool
	Requests() []core.PlayRequest
}

type PlayerSuite struct {
	New               func(t *testing.T) (core.Player, PlayerProbe)
	First             core.PlayRequest
	Second            core.PlayRequest
	CheckCancellation bool
	Timeout           time.Duration
}

func RunPlayer(t *testing.T, suite PlayerSuite) {
	t.Helper()
	validatePlayerSuite(t, &suite)
	t.Run("start and load", func(t *testing.T) {
		player, probe := suite.New(t)
		requirePlayerFactoryResult(t, player, probe)
		session, err := player.Start(context.Background(), suite.First)
		if err != nil {
			t.Fatal(err)
		}
		if session == nil {
			t.Fatal("player returned nil session")
		}
		closeSession := guardedClose(t, session, suite.Timeout)
		events := session.Events()
		if events == nil {
			t.Fatal("player returned nil event channel")
		}
		requireEventually(t, suite.Timeout, func() bool { return probe.Starts() == 1 && probe.ActiveSessions() == 1 })
		assertRequests(t, probe.Requests(), []core.PlayRequest{suite.First})
		if err := session.Load(context.Background(), suite.Second); err != nil {
			t.Fatal(err)
		}
		requireEventually(t, suite.Timeout, func() bool {
			return probe.Starts() == 1 && probe.Loads() == 1 && probe.ActiveSessions() == 1
		})
		assertRequests(t, probe.Requests(), []core.PlayRequest{suite.First, suite.Second})
		if err := closeSession(); err != nil {
			t.Fatal(err)
		}
		requireEventually(t, suite.Timeout, func() bool { return probe.ActiveSessions() == 0 && probe.Clean() })
	})
	t.Run("close cleanup", func(t *testing.T) {
		player, probe := suite.New(t)
		requirePlayerFactoryResult(t, player, probe)
		session, err := player.Start(context.Background(), suite.First)
		if err != nil {
			t.Fatal(err)
		}
		if session == nil {
			t.Fatal("player returned nil session")
		}
		closeSession := guardedClose(t, session, suite.Timeout)
		events := session.Events()
		if events == nil {
			t.Fatal("player returned nil event channel")
		}
		if err := closeSession(); err != nil {
			t.Fatal(err)
		}
		eventCount := probe.EventCount()
		time.Sleep(suite.Timeout / 4)
		if probe.EventCount() != eventCount {
			t.Fatal("player emitted event after close")
		}
		requireEventually(t, suite.Timeout, func() bool { return probe.ActiveSessions() == 0 && probe.Clean() })
	})
	if suite.CheckCancellation {
		t.Run("cancellation", func(t *testing.T) {
			player, probe := suite.New(t)
			requirePlayerFactoryResult(t, player, probe)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := player.Start(ctx, suite.First); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled start error = %v", err)
			}
			if probe.Starts() != 0 || probe.ActiveSessions() != 0 {
				t.Fatal("canceled start changed player state")
			}
			session, err := player.Start(context.Background(), suite.First)
			if err != nil {
				t.Fatal(err)
			}
			if session == nil {
				t.Fatal("player returned nil session")
			}
			closeSession := guardedClose(t, session, suite.Timeout)
			loads := probe.Loads()
			if err := session.Load(ctx, suite.Second); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled load error = %v", err)
			}
			if probe.Loads() != loads {
				t.Fatal("canceled load changed player state")
			}
			if err := closeSession(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func guardedClose(t *testing.T, session core.PlaybackSession, timeout time.Duration) func() error {
	t.Helper()
	var once sync.Once
	done := make(chan struct{})
	var closeErr error
	closeSession := func() error {
		once.Do(func() {
			go func() {
				closeErr = session.Close()
				close(done)
			}()
		})
		select {
		case <-done:
			return closeErr
		case <-time.After(timeout):
			return errors.New("session close timed out")
		}
	}
	t.Cleanup(func() { _ = closeSession() })
	return closeSession
}

func requirePlayerFactoryResult(t *testing.T, player core.Player, probe PlayerProbe) {
	t.Helper()
	if isNil(player) || isNil(probe) {
		t.Fatal("player factory returned nil")
	}
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func validatePlayerSuite(t *testing.T, suite *PlayerSuite) {
	t.Helper()
	if suite.New == nil {
		t.Fatal("player factory is nil")
	}
	if suite.First.AnimeID == "" || suite.First.EpisodeID == "" || suite.Second.AnimeID == "" || suite.Second.EpisodeID == "" {
		t.Fatal("player requests require canonical IDs")
	}
	if err := validateAbsoluteURL(suite.First.Source.URL()); err != nil {
		t.Fatal(err)
	}
	if err := validateAbsoluteURL(suite.Second.Source.URL()); err != nil {
		t.Fatal(err)
	}
	if suite.Timeout <= 0 {
		suite.Timeout = time.Second
	}
}

func assertRequests(t *testing.T, actual, expected []core.PlayRequest) {
	t.Helper()
	if err := equal(actual, expected); err != nil {
		t.Fatal(err)
	}
}

func requireEventually(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition timed out")
		}
		time.Sleep(time.Millisecond)
	}
}
