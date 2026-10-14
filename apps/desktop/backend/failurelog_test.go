// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/adapters/player/libmpv"
	"animeportable/core"
)

var failureLineFormat = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z (resolve|proxy_upstream|player_load|other) (-|[1-5]\d\d) [a-z_]+$`)

func readFailureLines(t *testing.T, directory string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(directory, failureLogName))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func TestFailureLineFormat(t *testing.T) {
	at := time.Date(2026, 10, 14, 1, 2, 3, 999, time.FixedZone("x", 8*3600))
	cases := []struct {
		stage    failureStage
		status   int
		category failureCategory
		want     string
	}{
		{stageResolve, 502, failureCategory(securehttp.KindStatus), "2026-10-13T17:02:03Z resolve 502 status"},
		{stagePlayerLoad, 0, categoryPlayerFailed, "2026-10-13T17:02:03Z player_load - player_failed"},
		{"https://secret.example", 99, "token=abc", "2026-10-13T17:02:03Z other - unknown"},
		{stageOther, 600, categoryTimeout, "2026-10-13T17:02:03Z other - timeout"},
	}
	for _, test := range cases {
		if got := formatFailureLine(at, test.stage, test.status, test.category); got != test.want {
			t.Errorf("got %q want %q", got, test.want)
		}
	}
}

func TestFailureLogNeverWritesErrorText(t *testing.T) {
	directory := t.TempDir()
	log := NewFailureLog(directory)
	secret := "https://cdn.example/v.mp4?token=SECRET cookie=SESSION episode:abc"
	log.recordPlayFailure(&stageError{stage: stageResolve, err: errors.New(secret)})
	log.recordPlayFailure(fmt.Errorf("%s: %w", secret, libmpv.ErrPlayerFailed))
	log.recordPlayFailure(fmt.Errorf("%s: %w", secret, &securehttp.Error{Kind: securehttp.KindStatus, Status: 502}))
	lines := readFailureLines(t, directory)
	want := []string{" resolve - unknown", " player_load - player_failed", " other 502 status"}
	if len(lines) != len(want) {
		t.Fatalf("lines = %q", lines)
	}
	for index, line := range lines {
		if !failureLineFormat.MatchString(line) || !strings.HasSuffix(line, want[index]) {
			t.Errorf("line %d = %q", index, line)
		}
		for _, forbidden := range []string{"SECRET", "SESSION", "cdn.example", "episode", "http"} {
			if strings.Contains(line, forbidden) {
				t.Errorf("line %q leaks %q", line, forbidden)
			}
		}
	}
}

func TestFailureLogSkipsCancellationAndShutdown(t *testing.T) {
	directory := t.TempDir()
	log := NewFailureLog(directory)
	log.recordPlayFailure(context.Canceled)
	log.recordPlayFailure(ErrClosed)
	log.recordPlayFailure(&securehttp.Error{Kind: securehttp.KindCanceled})
	log.recordPlayFailure(nil)
	if _, err := os.Stat(filepath.Join(directory, failureLogName)); !os.IsNotExist(err) {
		t.Fatalf("log should not exist: %v", err)
	}
}

func TestFailureLogKeepsNewest50AcrossRestarts(t *testing.T) {
	directory := t.TempDir()
	for _, batch := range []int{30, 30} {
		log := NewFailureLog(directory)
		for range batch {
			log.record(stageOther, 0, categoryUnknown)
		}
	}
	if lines := readFailureLines(t, directory); len(lines) != failureLogMaxLines {
		t.Fatalf("lines = %d", len(lines))
	}
	if _, err := os.Stat(filepath.Join(directory, failureLogName+".tmp")); !os.IsNotExist(err) {
		t.Fatalf("temporary file left behind: %v", err)
	}
}

func TestFailureLogRotationDropsOldest(t *testing.T) {
	directory := t.TempDir()
	log := NewFailureLog(directory)
	for status := 100; status < 160; status++ {
		log.record(stageResolve, status, failureCategory(securehttp.KindStatus))
	}
	lines := readFailureLines(t, directory)
	if len(lines) != 50 || !strings.Contains(lines[0], " 110 ") || !strings.Contains(lines[49], " 159 ") {
		t.Fatalf("first=%q last=%q n=%d", lines[0], lines[len(lines)-1], len(lines))
	}
}

func TestFailureLogConcurrentWrites(t *testing.T) {
	directory := t.TempDir()
	log := NewFailureLog(directory)
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range 5 {
				log.record(stageProxyUpstream, 502, failureCategory(securehttp.KindStatus))
				log.UpstreamRejected(502)
				log.PlayerFailed()
			}
		}()
	}
	wait.Wait()
	lines := readFailureLines(t, directory)
	if len(lines) == 0 || len(lines) > failureLogMaxLines {
		t.Fatalf("lines = %d", len(lines))
	}
	for _, line := range lines {
		if !failureLineFormat.MatchString(line) {
			t.Fatalf("malformed line %q", line)
		}
	}
}

func TestPlayerFailureIsAttributedToRecentProxyRejection(t *testing.T) {
	directory := t.TempDir()
	log := NewFailureLog(directory)
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	log.now = func() time.Time { return now }
	log.UpstreamRejected(502)
	log.PlayerFailed() // proxy_upstream 502
	log.PlayerFailed() // consumed: plain player_load
	log.UpstreamRejected(503)
	now = now.Add(2 * upstreamFailureWindow)
	log.PlayerFailed() // stale rejection: plain player_load
	log.UpstreamRejected(502)
	log.beginAttempt()
	log.PlayerFailed() // new attempt forgot it
	log.UpstreamRejected(0)
	log.PlayerFailed() // proxy_upstream, no status
	want := []string{"proxy_upstream 502 status", "player_load - player_failed", "player_load - player_failed", "player_load - player_failed", "proxy_upstream - network"}
	lines := readFailureLines(t, directory)
	if len(lines) != len(want) {
		t.Fatalf("lines = %q", lines)
	}
	for index := range want {
		if !strings.HasSuffix(lines[index], " "+want[index]) {
			t.Errorf("line %d = %q want suffix %q", index, lines[index], want[index])
		}
	}
}

func TestNilFailureLogIsNoOp(t *testing.T) {
	var log *FailureLog
	log.record(stageOther, 0, categoryUnknown)
	log.recordPlayFailure(errors.New("x"))
	log.beginAttempt()
	log.UpstreamRejected(502)
	log.PlayerFailed()
}

func TestStageSourceTagsResolveErrors(t *testing.T) {
	cause := &securehttp.Error{Kind: securehttp.KindStatus, Status: 502}
	source := stageSource{failingSource{err: cause}}
	_, err := source.Resolve(context.Background(), core.EpisodeRef{})
	var tagged *stageError
	if !errors.As(err, &tagged) || tagged.stage != stageResolve || !errors.Is(err, cause) {
		t.Fatalf("err = %v", err)
	}
}

type failingSource struct {
	core.AnimeSource
	err error
}

func (source failingSource) Resolve(context.Context, core.EpisodeRef) (core.PlaybackSource, error) {
	return core.PlaybackSource{}, source.err
}
