// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"animeportable/core"
)

func scheduleEntry(id string, weekday time.Weekday) core.ScheduleEntry {
	return core.ScheduleEntry{Ref: core.MetadataRef{Provider: "bangumi", ID: id}, Title: "Title " + id, Weekday: weekday}
}

func openScheduleStore(t *testing.T) *Store {
	t.Helper()
	return openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
}

func scheduleRowCount(t *testing.T, store *Store) int {
	t.Helper()
	var count int
	if err := store.db.QueryRow("SELECT count(*) FROM schedule_entries").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestScheduleRoundTripsAndSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	store := openLibraryStore(t, path)
	ctx := context.Background()
	entries := []core.ScheduleEntry{
		{Ref: core.MetadataRef{Provider: "bangumi", ID: "1"}, Title: "週一的豐滿", NativeTitle: "月曜日のたわわ", Weekday: time.Monday},
		{Ref: core.MetadataRef{Provider: "bangumi", ID: "2"}, Title: "無原名", Weekday: time.Sunday},
	}
	for weekday := time.Tuesday; weekday <= time.Saturday; weekday++ {
		entries = append(entries, scheduleEntry("d"+strconv.Itoa(int(weekday)), weekday))
	}
	if err := store.ReplaceSchedule(ctx, entries); err != nil {
		t.Fatal(err)
	}
	want := []core.ScheduleEntry{entries[1], entries[0], entries[2], entries[3], entries[4], entries[5], entries[6]}
	if got, err := store.Schedule(ctx); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("schedule = %#v, %v, want %#v", got, err, want)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openLibraryStore(t, path)
	defer store.Close()
	if got, err := store.Schedule(ctx); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("reopened schedule = %#v, %v, want %#v", got, err, want)
	}
}

func TestScheduleOrdersByWeekdayThenPosition(t *testing.T) {
	store := openScheduleStore(t)
	defer store.Close()
	entries := []core.ScheduleEntry{
		scheduleEntry("z", time.Friday),
		scheduleEntry("b", time.Monday),
		scheduleEntry("y", time.Friday),
		scheduleEntry("a", time.Monday),
		scheduleEntry("s", time.Sunday),
	}
	if err := store.ReplaceSchedule(context.Background(), entries); err != nil {
		t.Fatal(err)
	}
	want := []core.ScheduleEntry{entries[4], entries[1], entries[3], entries[0], entries[2]}
	if got, err := store.Schedule(context.Background()); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("schedule = %#v, %v, want %#v", got, err, want)
	}
}

func TestScheduleEmptyIsNonNil(t *testing.T) {
	store := openScheduleStore(t)
	defer store.Close()
	if got, err := store.Schedule(context.Background()); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty schedule = %#v, %v, want empty non-nil slice", got, err)
	}
}

func TestReplaceScheduleDropsOldRows(t *testing.T) {
	store := openScheduleStore(t)
	defer store.Close()
	ctx := context.Background()
	if err := store.ReplaceSchedule(ctx, []core.ScheduleEntry{scheduleEntry("old", time.Monday)}); err != nil {
		t.Fatal(err)
	}
	fresh := scheduleEntry("fresh", time.Tuesday)
	if err := store.ReplaceSchedule(ctx, []core.ScheduleEntry{fresh}); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Schedule(ctx); err != nil || !reflect.DeepEqual(got, []core.ScheduleEntry{fresh}) {
		t.Fatalf("schedule = %#v, %v, want only fresh", got, err)
	}
	if err := store.ReplaceSchedule(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if count := scheduleRowCount(t, store); count != 0 {
		t.Fatalf("rows after empty replace = %d, want 0", count)
	}
}

func TestScheduleSameShowOnTwoWeekdays(t *testing.T) {
	store := openScheduleStore(t)
	defer store.Close()
	entries := []core.ScheduleEntry{scheduleEntry("1", time.Monday), scheduleEntry("1", time.Thursday)}
	if err := store.ReplaceSchedule(context.Background(), entries); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Schedule(context.Background()); err != nil || !reflect.DeepEqual(got, entries) {
		t.Fatalf("schedule = %#v, %v, want %#v", got, err, entries)
	}
}

func TestScheduleFillsAnimeIDFromMatchingMetadataOnly(t *testing.T) {
	store := openScheduleStore(t)
	defer store.Close()
	ctx := context.Background()
	for _, id := range []core.AnimeID{"known", "other"} {
		if err := store.SaveAnime(ctx, core.Anime{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveMetadata(ctx, "known", core.AnimeMetadata{Ref: core.MetadataRef{Provider: "bangumi", ID: "1"}, Title: "Known"}); err != nil {
		t.Fatal(err)
	}
	// Same external id under another provider must not match.
	if err := store.SaveMetadata(ctx, "other", core.AnimeMetadata{Ref: core.MetadataRef{Provider: "anilist", ID: "2"}, Title: "Other"}); err != nil {
		t.Fatal(err)
	}
	// AnimeID on input is ignored.
	input := []core.ScheduleEntry{scheduleEntry("1", time.Monday), scheduleEntry("2", time.Monday), scheduleEntry("3", time.Monday)}
	input[2].AnimeID = "other"
	if err := store.ReplaceSchedule(ctx, input); err != nil {
		t.Fatal(err)
	}
	got, err := store.Schedule(ctx)
	if err != nil || len(got) != 3 {
		t.Fatalf("schedule = %#v, %v", got, err)
	}
	if got[0].AnimeID != "known" || got[1].AnimeID != "" || got[2].AnimeID != "" {
		t.Fatalf("anime ids = %q, %q, %q, want known, empty, empty", got[0].AnimeID, got[1].AnimeID, got[2].AnimeID)
	}
}

func TestReplaceScheduleRejectsInvalidInputWithoutWriting(t *testing.T) {
	store := openScheduleStore(t)
	defer store.Close()
	ctx := context.Background()
	kept := scheduleEntry("kept", time.Monday)
	if err := store.ReplaceSchedule(ctx, []core.ScheduleEntry{kept}); err != nil {
		t.Fatal(err)
	}
	valid := scheduleEntry("1", time.Tuesday)
	with := func(change func(*core.ScheduleEntry)) []core.ScheduleEntry {
		entry := scheduleEntry("2", time.Wednesday)
		change(&entry)
		return []core.ScheduleEntry{valid, entry}
	}
	tooMany := make([]core.ScheduleEntry, core.MaxScheduleEntries+1)
	for index := range tooMany {
		tooMany[index] = scheduleEntry(strconv.Itoa(index), time.Monday)
	}
	oversized := strings.Repeat("x", maxIdentityBytes+1)
	tests := map[string][]core.ScheduleEntry{
		"over limit":        tooMany,
		"empty provider":    with(func(e *core.ScheduleEntry) { e.Ref.Provider = "" }),
		"empty external id": with(func(e *core.ScheduleEntry) { e.Ref.ID = "" }),
		"oversized ref":     with(func(e *core.ScheduleEntry) { e.Ref.ID = oversized }),
		"weekday too high":  with(func(e *core.ScheduleEntry) { e.Weekday = 7 }),
		"weekday negative":  with(func(e *core.ScheduleEntry) { e.Weekday = -1 }),
		"both titles empty": with(func(e *core.ScheduleEntry) { e.Title, e.NativeTitle = "", "" }),
		"control in title":  with(func(e *core.ScheduleEntry) { e.Title = "a\x01b" }),
		"invalid utf8":      with(func(e *core.ScheduleEntry) { e.NativeTitle = "\xff" }),
		"oversized title":   with(func(e *core.ScheduleEntry) { e.Title = strings.Repeat("t", maxTitleBytes+1) }),
		"duplicate":         {valid, valid},
	}
	for name, entries := range tests {
		if err := store.ReplaceSchedule(ctx, entries); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s error = %v, want ErrInvalidInput", name, err)
		}
		if got, err := store.Schedule(ctx); err != nil || !reflect.DeepEqual(got, []core.ScheduleEntry{kept}) {
			t.Fatalf("%s left schedule = %#v, %v, want previous", name, got, err)
		}
	}
}

func TestScheduleAcceptsMaximumEntries(t *testing.T) {
	store := openScheduleStore(t)
	defer store.Close()
	entries := make([]core.ScheduleEntry, core.MaxScheduleEntries)
	for index := range entries {
		entries[index] = scheduleEntry(strconv.Itoa(index), time.Weekday(index%7))
	}
	if err := store.ReplaceSchedule(context.Background(), entries); err != nil {
		t.Fatal(err)
	}
	if count := scheduleRowCount(t, store); count != core.MaxScheduleEntries {
		t.Fatalf("rows = %d, want %d", count, core.MaxScheduleEntries)
	}
}

func TestScheduleFailsClosedOnCorruptRow(t *testing.T) {
	store := openScheduleStore(t)
	defer store.Close()
	if _, err := store.db.Exec("INSERT INTO schedule_entries (provider, external_id, title, native_title, weekday, position) VALUES ('bangumi', '1', 'a' || char(1), '', 1, 0)"); err != nil {
		t.Fatal(err)
	}
	got, err := store.Schedule(context.Background())
	if !errors.Is(err, ErrStorage) || len(got) != 0 {
		t.Fatalf("corrupt row = %#v, %v, want ErrStorage and no entries", got, err)
	}
}

func TestScheduleHonoursCancelledContext(t *testing.T) {
	store := openScheduleStore(t)
	defer store.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.ReplaceSchedule(ctx, []core.ScheduleEntry{scheduleEntry("1", time.Monday)}); !errors.Is(err, context.Canceled) {
		t.Fatalf("replace error = %v, want context.Canceled", err)
	}
	if _, err := store.Schedule(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("schedule error = %v, want context.Canceled", err)
	}
	if count := scheduleRowCount(t, store); count != 0 {
		t.Fatalf("rows after cancelled replace = %d, want 0", count)
	}
}

func TestScheduleMigrationAppliesToExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	var previous []migration
	for version, name := range []string{"0001_initial.sql", "0002_episode_mappings.sql", "0003_drop_settings_mpv_path.sql", "0004_episode_number.sql", "0005_recent_updates.sql"} {
		contents, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		previous = append(previous, migration{version: version + 1, name: name, sql: string(contents)})
	}
	legacy, err := open(context.Background(), path, migrationSource{migrations: previous})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.db.Exec(`INSERT INTO anime (id, title, native_title, description) VALUES ('anime', 'Title', '', '')`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	store := openLibraryStore(t, path)
	defer store.Close()
	ctx := context.Background()
	if got, err := store.Anime(ctx, "anime"); err != nil || got.Title != "Title" {
		t.Fatalf("anime after upgrade = %#v, %v", got, err)
	}
	entry := scheduleEntry("1", time.Monday)
	if err := store.ReplaceSchedule(ctx, []core.ScheduleEntry{entry}); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Schedule(ctx); err != nil || !reflect.DeepEqual(got, []core.ScheduleEntry{entry}) {
		t.Fatalf("schedule after upgrade = %#v, %v", got, err)
	}
	var version int
	if err := store.db.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&version); err != nil || version != 11 {
		t.Fatalf("schema version = %d, %v, want 11", version, err)
	}
}
