// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/core"
	"animeportable/tests/contract"
)

func TestStoreContract(t *testing.T) {
	contract.RunStore(t, contract.StoreSuite{New: func(t *testing.T) core.Store {
		store, err := Open(context.Background(), filepath.Join(t.TempDir(), "anime.sqlite"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := store.Close(); err != nil {
				t.Error(err)
			}
		})
		return store
	}})
}

func TestPlaybackAndSettingsPersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	store := openLibraryStore(t, path)
	ctx := context.Background()
	for _, anime := range []core.Anime{{ID: "anime-a"}, {ID: "anime-b"}} {
		if err := store.SaveAnime(ctx, anime); err != nil {
			t.Fatal(err)
		}
	}
	fixed := time.Date(2026, time.August, 31, 12, 34, 56, 123456789, time.UTC)
	entryA := core.HistoryEntry{Progress: core.PlaybackProgress{
		AnimeID: "anime-a", EpisodeID: "episode-a", Position: 2*time.Minute + 3*time.Nanosecond,
		Duration: 24*time.Minute + 7*time.Nanosecond, Completed: true, UpdatedAt: fixed,
	}, LastPlayedAt: fixed}
	entryB := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "anime-a", EpisodeID: "episode-b", UpdatedAt: fixed}, LastPlayedAt: fixed.Add(time.Second)}
	entryC := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "anime-b", EpisodeID: "episode-a", UpdatedAt: time.Time{}}, LastPlayedAt: fixed.Add(time.Second)}
	entryD := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "anime-a", EpisodeID: "episode-c"}}
	for _, entry := range []core.HistoryEntry{entryA, entryB, entryC, entryD} {
		if err := store.SavePlaybackCheckpoint(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := store.Progress(ctx, entryA.Progress.AnimeID, entryA.Progress.EpisodeID); err != nil || got != entryA.Progress {
		t.Fatalf("progress = %#v, %v", got, err)
	}
	wantHistory := []core.HistoryEntry{entryB, entryC, entryA, entryD}
	if got, err := store.History(ctx); err != nil || !reflect.DeepEqual(got, wantHistory) {
		t.Fatalf("history = %#v, %v, want %#v", got, err, wantHistory)
	}
	settings := core.Settings{
		Appearance: core.AppearanceDark, AutoplayNext: core.ToggleDisabled, AutoPlay: core.ToggleEnabled,
		ResumePlayback: core.ToggleEnabled, Language: core.LanguageEnglish,
		PlaybackSpeed: 150, HardwareDecoding: core.ToggleDisabled,
		ProxyMode: core.ProxyModeManual, ProxyAddress: "proxy.example:8080",
	}
	if err := store.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveHistory(ctx, "anime-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveHistory(ctx, "anime-a"); err != nil {
		t.Fatalf("idempotent removal: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openLibraryStore(t, path)
	defer store.Close()
	if got, err := store.Progress(ctx, entryC.Progress.AnimeID, entryC.Progress.EpisodeID); err != nil || got != entryC.Progress {
		t.Fatalf("reopened progress = %#v, %v", got, err)
	}
	// Removing anime-a's history also removed its resume position (owner decision 2026-10-19).
	if got, err := store.Progress(ctx, entryA.Progress.AnimeID, entryA.Progress.EpisodeID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("removed progress = %#v, %v", got, err)
	}
	if got, err := store.History(ctx); err != nil || !reflect.DeepEqual(got, []core.HistoryEntry{entryC}) {
		t.Fatalf("reopened history = %#v, %v", got, err)
	}
	if got, err := store.Settings(ctx); err != nil || got != settings {
		t.Fatalf("reopened settings = %#v, %v", got, err)
	}
}

func TestPlaybackCheckpointReopensConsistently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	store := openLibraryStore(t, path)
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	entry := core.HistoryEntry{
		Progress: core.PlaybackProgress{
			AnimeID: "anime", EpisodeID: "episode", Position: 3 * time.Minute,
			Duration: 24 * time.Minute, UpdatedAt: time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC),
		},
		LastPlayedAt: time.Date(2026, 8, 31, 12, 0, 1, 0, time.UTC),
	}
	if err := store.SavePlaybackCheckpoint(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openLibraryStore(t, path)
	defer store.Close()
	if got, err := store.Progress(ctx, entry.Progress.AnimeID, entry.Progress.EpisodeID); err != nil || got != entry.Progress {
		t.Fatalf("reopened checkpoint progress = %#v, %v", got, err)
	}
	history, err := store.History(ctx)
	if err != nil || len(history) != 1 {
		t.Fatalf("reopened checkpoint history = %#v, %v", history, err)
	}
	if err := equalHistoryEntry(history[0], entry); err != nil {
		t.Fatal(err)
	}
}

func TestPlaybackCheckpointSameTimestampUsesDeterministicMaximums(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	timestamp := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	first := core.HistoryEntry{Progress: core.PlaybackProgress{
		AnimeID: "anime", EpisodeID: "episode", Position: time.Minute, Duration: 24 * time.Minute, UpdatedAt: timestamp,
	}, LastPlayedAt: timestamp}
	second := first
	second.Progress.Position = 2 * time.Minute
	second.Progress.Duration = 20 * time.Minute
	second.Progress.Completed = true
	second.LastPlayedAt = timestamp.Add(time.Second)
	for _, entry := range []core.HistoryEntry{first, second} {
		if err := store.SavePlaybackCheckpoint(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}
	wantProgress := first.Progress
	wantProgress.Position = second.Progress.Position
	wantProgress.Completed = true
	if got, err := store.Progress(ctx, "anime", "episode"); err != nil || got != wantProgress {
		t.Fatalf("same timestamp progress = %#v, %v, want %#v", got, err, wantProgress)
	}
	history, err := store.History(ctx)
	if err != nil || len(history) != 1 {
		t.Fatalf("same timestamp history = %#v, %v", history, err)
	}
	wantEntry := core.HistoryEntry{Progress: wantProgress, LastPlayedAt: second.LastPlayedAt}
	if err := equalHistoryEntry(history[0], wantEntry); err != nil {
		t.Fatal(err)
	}
}

func TestPlaybackCheckpointConcurrentWritesKeepNewestTimestamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	first := openLibraryStore(t, path)
	second := openLibraryStore(t, path)
	defer first.Close()
	defer second.Close()
	ctx := context.Background()
	if err := first.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	older := core.HistoryEntry{Progress: core.PlaybackProgress{
		AnimeID: "anime", EpisodeID: "episode", Position: time.Minute, Duration: 20 * time.Minute, Completed: true, UpdatedAt: base,
	}, LastPlayedAt: base}
	newer := older
	newer.Progress.Position = 4 * time.Minute
	newer.Progress.Duration = 24 * time.Minute
	newer.Progress.Completed = false
	newer.Progress.UpdatedAt = base.Add(time.Minute)
	newer.LastPlayedAt = base.Add(time.Minute)
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		<-start
		results <- first.SavePlaybackCheckpoint(ctx, older)
	}()
	go func() {
		defer group.Done()
		<-start
		results <- second.SavePlaybackCheckpoint(ctx, newer)
	}()
	close(start)
	group.Wait()
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("concurrent checkpoint error = %v", err)
		}
	}
	want := newer.Progress
	want.Completed = true
	if got, err := first.Progress(ctx, "anime", "episode"); err != nil || got != want {
		t.Fatalf("concurrent checkpoint progress = %#v, %v, want %#v", got, err, want)
	}
}

func equalHistoryEntry(actual, expected core.HistoryEntry) error {
	if !reflect.DeepEqual(actual, expected) {
		return errors.New("history entry mismatch")
	}
	return nil
}

func TestHistoryOrdersFixedWidthTimesWithinSameSecond(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	second := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)
	earlier := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "anime", EpisodeID: "earlier"}, LastPlayedAt: second}
	later := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "anime", EpisodeID: "later"}, LastPlayedAt: second.Add(100 * time.Millisecond)}
	for _, entry := range []core.HistoryEntry{earlier, later} {
		if err := store.SavePlaybackCheckpoint(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.History(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []core.HistoryEntry{later, earlier}) {
		t.Fatalf("history = %#v, want later nanosecond first", got)
	}
}

func TestSchemaDoesNotPersistPlaybackCapabilities(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	rows, err := store.db.Query(`SELECT tables.name, columns.name
		FROM sqlite_schema AS tables
		JOIN pragma_table_info(tables.name) AS columns
		WHERE tables.type = 'table'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var tableName, columnName string
		if err := rows.Scan(&tableName, &columnName); err != nil {
			t.Fatal(err)
		}
		qualified := strings.ToLower(tableName + "." + columnName)
		for _, forbidden := range []string{
			"cookie",
			"token",
			"request_header",
			"authorization",
			"stream_url",
			"playback_url",
			"proxy_session",
			"ipc_endpoint",
			"credential",
			"secret",
		} {
			if strings.Contains(qualified, forbidden) {
				t.Fatalf("persistent schema contains forbidden capability field %q", qualified)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
