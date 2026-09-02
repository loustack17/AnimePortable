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
	progress := core.PlaybackProgress{
		AnimeID: "anime-a", EpisodeID: "episode-a", Position: 2*time.Minute + 3*time.Nanosecond,
		Duration: 24*time.Minute + 7*time.Nanosecond, Completed: true, UpdatedAt: fixed,
	}
	if err := store.SaveProgress(ctx, progress); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Progress(ctx, progress.AnimeID, progress.EpisodeID); err != nil || got != progress {
		t.Fatalf("progress = %#v, %v", got, err)
	}
	replacement := progress
	replacement.Position = 4 * time.Minute
	replacement.Completed = false
	replacement.UpdatedAt = time.Time{}
	if err := store.SaveProgress(ctx, replacement); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Progress(ctx, replacement.AnimeID, replacement.EpisodeID); err != nil || got != replacement {
		t.Fatalf("replaced progress = %#v, %v", got, err)
	}
	entryA := core.HistoryEntry{Progress: replacement, LastPlayedAt: fixed}
	entryB := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "anime-a", EpisodeID: "episode-b", UpdatedAt: fixed}, LastPlayedAt: fixed.Add(time.Second)}
	entryC := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "anime-b", EpisodeID: "episode-a", UpdatedAt: time.Time{}}, LastPlayedAt: fixed.Add(time.Second)}
	entryD := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "anime-a", EpisodeID: "episode-c"}}
	for _, entry := range []core.HistoryEntry{entryA, entryB, entryC, entryD} {
		if err := store.AddHistory(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}
	replacedEntry := entryA
	replacedEntry.Progress.Position = time.Second
	replacedEntry.LastPlayedAt = fixed.Add(2 * time.Second)
	if err := store.AddHistory(ctx, replacedEntry); err != nil {
		t.Fatal(err)
	}
	wantHistory := []core.HistoryEntry{replacedEntry, entryB, entryC, entryD}
	if got, err := store.History(ctx); err != nil || !reflect.DeepEqual(got, wantHistory) {
		t.Fatalf("history = %#v, %v, want %#v", got, err, wantHistory)
	}
	settings := core.Settings{
		Appearance: core.AppearanceDark, MPVPath: "C:/播放器/mpv.exe", AutoplayNext: core.ToggleDisabled,
		ResumePlayback: core.ToggleEnabled, Language: core.LanguageEnglish,
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
	if got, err := store.Progress(ctx, replacement.AnimeID, replacement.EpisodeID); err != nil || got != replacement {
		t.Fatalf("reopened progress = %#v, %v", got, err)
	}
	if got, err := store.History(ctx); err != nil || !reflect.DeepEqual(got, []core.HistoryEntry{entryC}) {
		t.Fatalf("reopened history = %#v, %v", got, err)
	}
	if got, err := store.Settings(ctx); err != nil || got != settings {
		t.Fatalf("reopened settings = %#v, %v", got, err)
	}
}

func TestPlaybackAndSettingsValidateInput(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	if got, err := store.History(ctx); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty history = %#v, %v", got, err)
	}
	if got, err := store.Settings(ctx); err != nil || got != core.DefaultSettings() {
		t.Fatalf("default settings = %#v, %v", got, err)
	}
	unknownProgress := core.PlaybackProgress{AnimeID: "missing", EpisodeID: "episode"}
	unknownHistory := core.HistoryEntry{Progress: unknownProgress}
	for name, operation := range map[string]func() error{
		"progress foreign key": func() error { return store.SaveProgress(ctx, unknownProgress) },
		"history foreign key":  func() error { return store.AddHistory(ctx, unknownHistory) },
		"remove foreign key":   func() error { return store.RemoveHistory(ctx, "missing") },
	} {
		t.Run(name, func(t *testing.T) {
			if err := operation(); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
	injection := core.AnimeID("anime'); DROP TABLE playback_progress; --")
	if err := store.SaveAnime(ctx, core.Anime{ID: injection}); err != nil {
		t.Fatal(err)
	}
	valid := core.PlaybackProgress{AnimeID: injection, EpisodeID: "episode"}
	if err := store.SaveProgress(ctx, valid); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Progress(ctx, injection, "episode"); err != nil || got != valid {
		t.Fatalf("injection-like progress = %#v, %v", got, err)
	}
	tooLongEpisode := core.EpisodeID(strings.Repeat("x", maxIdentityBytes+1))
	invalidTime := time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
	for name, operation := range map[string]func() error{
		"empty episode": func() error { return store.SaveProgress(ctx, core.PlaybackProgress{AnimeID: injection}) },
		"episode bounds": func() error {
			return store.SaveProgress(ctx, core.PlaybackProgress{AnimeID: injection, EpisodeID: tooLongEpisode})
		},
		"negative position": func() error {
			return store.SaveProgress(ctx, core.PlaybackProgress{AnimeID: injection, EpisodeID: "episode", Position: -time.Nanosecond})
		},
		"negative duration": func() error {
			return store.AddHistory(ctx, core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: injection, EpisodeID: "episode", Duration: -time.Nanosecond}})
		},
		"out of range time": func() error {
			return store.AddHistory(ctx, core.HistoryEntry{Progress: valid, LastPlayedAt: invalidTime})
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := operation(); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
	invalidUTF8 := string([]byte{0xff})
	tooLongPath := strings.Repeat("x", maxMPVPathBytes+1)
	for name, settings := range map[string]core.Settings{
		"appearance enum": {Appearance: core.AppearanceDark + 1},
		"autoplay enum":   {AutoplayNext: core.ToggleEnabled + 1},
		"resume enum":     {ResumePlayback: core.ToggleEnabled + 1},
		"language enum":   {Language: core.LanguageEnglish + 1},
		"invalid utf-8":   {MPVPath: invalidUTF8},
		"path bounds":     {MPVPath: tooLongPath},
	} {
		t.Run(name, func(t *testing.T) {
			if err := store.SaveSettings(ctx, settings); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
	for _, appearance := range []core.Appearance{core.AppearanceUnspecified, core.AppearanceSystem, core.AppearanceLight, core.AppearanceDark} {
		for _, toggle := range []core.Toggle{core.ToggleUnspecified, core.ToggleDisabled, core.ToggleEnabled} {
			for _, language := range []core.Language{core.LanguageUnspecified, core.LanguageTraditionalChinese, core.LanguageEnglish} {
				settings := core.Settings{Appearance: appearance, AutoplayNext: toggle, ResumePlayback: toggle, Language: language}
				if err := store.SaveSettings(ctx, settings); err != nil {
					t.Fatalf("settings %#v: %v", settings, err)
				}
			}
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.SaveSettings(cancelled, core.DefaultSettings()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled settings error = %v", err)
	}
	if got, err := store.History(cancelled); err == nil || got == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled history = %#v, %v", got, err)
	}
}

func TestPlaybackCheckpointRejectsMissingAnimeAndCancellation(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	entry := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "missing", EpisodeID: "episode"}}
	if err := store.SavePlaybackCheckpoint(ctx, entry); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing anime error = %v, want ErrInvalidInput", err)
	}
	if _, err := store.Progress(ctx, entry.Progress.AnimeID, entry.Progress.EpisodeID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("missing anime progress = %v, want ErrNotFound", err)
	}
	if history, err := store.History(ctx); err != nil || len(history) != 0 {
		t.Fatalf("missing anime history = %#v, %v", history, err)
	}
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	entry.Progress.AnimeID = "anime"
	if err := store.SavePlaybackCheckpoint(cancelled, entry); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled checkpoint error = %v, want context.Canceled", err)
	}
	if _, err := store.Progress(ctx, entry.Progress.AnimeID, entry.Progress.EpisodeID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("cancelled checkpoint progress = %v, want ErrNotFound", err)
	}
}

func TestPlaybackCheckpointRollsBackWhenHistoryWriteFails(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER checkpoint_abort BEFORE INSERT ON playback_history
	BEGIN SELECT RAISE(ABORT, 'checkpoint abort'); END`); err != nil {
		t.Fatal(err)
	}
	entry := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "anime", EpisodeID: "episode", Position: time.Minute}}
	if err := store.SavePlaybackCheckpoint(ctx, entry); !errors.Is(err, ErrStorage) {
		t.Fatalf("triggered checkpoint error = %v, want ErrStorage", err)
	}
	if _, err := store.db.Exec("DROP TRIGGER checkpoint_abort"); err != nil {
		t.Fatal(err)
	}
	var progressCount, historyCount int
	if err := store.db.QueryRow("SELECT count(*) FROM playback_progress").Scan(&progressCount); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT count(*) FROM playback_history").Scan(&historyCount); err != nil {
		t.Fatal(err)
	}
	if progressCount != 0 || historyCount != 0 {
		t.Fatalf("rolled back rows = progress:%d history:%d", progressCount, historyCount)
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

func TestPlaybackCheckpointGuardsStaleWritesAndCompletion(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	earlier := core.HistoryEntry{Progress: core.PlaybackProgress{
		AnimeID: "anime", EpisodeID: "episode", Position: time.Minute, Duration: 20 * time.Minute, UpdatedAt: base,
	}, LastPlayedAt: base}
	later := core.HistoryEntry{Progress: core.PlaybackProgress{
		AnimeID: "anime", EpisodeID: "episode", Position: 4 * time.Minute, Duration: 24 * time.Minute, UpdatedAt: base.Add(time.Minute),
	}, LastPlayedAt: base.Add(time.Minute)}
	staleComplete := earlier
	staleComplete.Progress.Position = 30 * time.Minute
	staleComplete.Progress.Completed = true
	staleComplete.LastPlayedAt = base.Add(-time.Second)
	for _, entry := range []core.HistoryEntry{earlier, later, staleComplete} {
		if err := store.SavePlaybackCheckpoint(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}
	want := later.Progress
	want.Completed = true
	if got, err := store.Progress(ctx, "anime", "episode"); err != nil || got != want {
		t.Fatalf("stale checkpoint progress = %#v, %v, want %#v", got, err, want)
	}
	history, err := store.History(ctx)
	if err != nil || len(history) != 1 {
		t.Fatalf("stale checkpoint history = %#v, %v", history, err)
	}
	wantEntry := later
	wantEntry.Progress.Completed = true
	if err := equalHistoryEntry(history[0], wantEntry); err != nil {
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

func TestPlaybackCheckpointRejectsCorruptDurableRows(t *testing.T) {
	tests := []struct {
		name   string
		table  string
		column string
		value  int
	}{
		{name: "negative progress position", table: "playback_progress", column: "position_ns", value: -1},
		{name: "invalid progress completed", table: "playback_progress", column: "completed", value: 2},
		{name: "negative history duration", table: "playback_history", column: "duration_ns", value: -1},
		{name: "invalid history completed", table: "playback_history", column: "completed", value: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
			defer store.Close()
			ctx := context.Background()
			if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
				t.Fatal(err)
			}
			entry := core.HistoryEntry{
				Progress: core.PlaybackProgress{
					AnimeID: "anime", EpisodeID: "episode", Position: time.Minute,
					Duration: 24 * time.Minute, UpdatedAt: time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC),
				},
				LastPlayedAt: time.Date(2026, 8, 31, 12, 0, 1, 0, time.UTC),
			}
			if err := store.SavePlaybackCheckpoint(ctx, entry); err != nil {
				t.Fatal(err)
			}
			if _, err := store.db.ExecContext(ctx, "PRAGMA ignore_check_constraints = ON"); err != nil {
				t.Fatal(err)
			}
			query := "UPDATE " + test.table + " SET " + test.column + " = ? WHERE anime_id = ? AND episode_id = ?"
			if _, err := store.db.ExecContext(ctx, query, test.value, "anime", "episode"); err != nil {
				t.Fatal(err)
			}
			entry.Progress.UpdatedAt = entry.Progress.UpdatedAt.Add(time.Minute)
			entry.LastPlayedAt = entry.LastPlayedAt.Add(time.Minute)
			if err := store.SavePlaybackCheckpoint(ctx, entry); !errors.Is(err, ErrStorage) {
				t.Fatalf("corrupt checkpoint error = %v, want ErrStorage", err)
			}
		})
	}
}

func equalHistoryEntry(actual, expected core.HistoryEntry) error {
	if !reflect.DeepEqual(actual, expected) {
		return errors.New("history entry mismatch")
	}
	return nil
}

func TestEncodeStoredTimeRejectsUTCYearOverflow(t *testing.T) {
	tests := []time.Time{
		time.Date(1, time.January, 1, 0, 0, 0, 0, time.FixedZone("+14", 14*60*60)),
		time.Date(9999, time.December, 31, 23, 59, 59, 0, time.FixedZone("-12", -12*60*60)),
	}
	for _, value := range tests {
		if _, err := encodeStoredTime(value); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("encodeStoredTime(%v) error = %v, want ErrInvalidInput", value, err)
		}
	}
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
		if err := store.AddHistory(ctx, entry); err != nil {
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

func TestPersistenceRejectsCorruptedStoredValues(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	progress := core.PlaybackProgress{AnimeID: "anime", EpisodeID: "episode"}
	if err := store.SaveProgress(ctx, progress); err != nil {
		t.Fatal(err)
	}
	invalidTime := "2026-99-99T99:99:99.000000000Z"
	if _, err := store.db.ExecContext(ctx, "UPDATE playback_progress SET updated_at = ?", invalidTime); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Progress(ctx, "anime", "episode"); err != ErrStorage || strings.Contains(err.Error(), invalidTime) {
		t.Fatalf("corrupted time error = %v, want sanitized ErrStorage", err)
	}

	if _, err := store.db.ExecContext(ctx, "PRAGMA ignore_check_constraints = ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO settings(id, appearance, mpv_path, autoplay_next, resume_playback, language)
		VALUES (1, 256, '', 256, 256, 256)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "PRAGMA ignore_check_constraints = OFF"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Settings(ctx); err != ErrStorage {
		t.Fatalf("corrupted settings error = %v, want ErrStorage", err)
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
