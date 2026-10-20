// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"animeportable/adapters/persistence/sqlite"
	"animeportable/core"
)

// fakeScheduleProvider serves entries, or err once it is set, and counts calls.
type fakeScheduleProvider struct {
	entries []core.ScheduleEntry
	err     error
	calls   int
}

func (provider *fakeScheduleProvider) Schedule(context.Context) ([]core.ScheduleEntry, error) {
	provider.calls++
	if provider.err != nil {
		return nil, provider.err
	}
	return provider.entries, nil
}

// catalogSource serves an Anime1-like catalog, or err once it is set.
type catalogSource struct {
	core.AnimeSource
	catalog []core.SourceAnime
	err     error
}

func (source *catalogSource) Catalog(context.Context) ([]core.SourceAnime, error) {
	return source.catalog, source.err
}

func bangumiRef(id string) core.MetadataRef {
	return core.MetadataRef{Provider: "bangumi", ID: id}
}

func TestScheduleIsStoredMatchedAndCachedOfflineAfterRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "schedule.sqlite")
	store, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, anime := range []core.Anime{{ID: "spy", Title: "間諜家家酒"}, {ID: "frieren", Title: "葬送的芙莉蓮"}} {
		if err := store.SaveAnime(ctx, anime); err != nil {
			t.Fatal(err)
		}
	}
	// 間諜家家酒 has Bangumi metadata, so its entry matches by subject even though Bangumi's
	// converted title differs from the Anime1 one.
	if err := store.SaveMetadata(ctx, "spy", core.AnimeMetadata{Ref: bangumiRef("100"), Title: "間諜過家家"}); err != nil {
		t.Fatal(err)
	}
	provider := &fakeScheduleProvider{entries: []core.ScheduleEntry{
		{Ref: bangumiRef("100"), Title: "間諜過家家", NativeTitle: "SPY×FAMILY", Weekday: time.Saturday},
		{Ref: bangumiRef("200"), Title: "葬送的芙莉蓮", NativeTitle: "葬送のフリーレン", Weekday: time.Friday},
		{Ref: bangumiRef("300"), Title: "新作品", Weekday: time.Monday},
	}}
	service := newWithDependencies(dependencies{store: store, source: &catalogSource{}, schedule: provider})
	fetched, err := service.Schedule(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []ScheduleItem{
		{AnimeID: "", Title: "新作品", Query: "新作品", Weekday: 1},
		{AnimeID: "frieren", Title: "葬送的芙莉蓮", Query: "葬送的芙莉蓮", Weekday: 5},
		{AnimeID: "spy", Title: "間諜家家酒", Query: "間諜過家家", Weekday: 6},
	}
	if len(fetched) != len(want) {
		t.Fatalf("fetched = %#v", fetched)
	}
	for index := range want {
		if fetched[index] != want[index] {
			t.Fatalf("fetched[%d] = %#v, want %#v", index, fetched[index], want[index])
		}
	}
	// A failed fetch leaves the stored schedule as it was.
	provider.err = errors.New("bangumi unreachable")
	if _, err := service.Schedule(ctx); err == nil {
		t.Fatal("schedule succeeded while the provider failed")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	offline := &fakeScheduleProvider{err: errors.New("bangumi unreachable")}
	service = newWithDependencies(dependencies{store: reopened, source: &catalogSource{err: errors.New("anime1 unreachable")}, schedule: offline})
	t.Cleanup(func() { _ = service.Close() })
	cached, err := service.CachedSchedule(ctx)
	if err != nil || len(cached) != len(want) {
		t.Fatalf("cached = %#v, %v", cached, err)
	}
	for index := range want {
		if cached[index] != want[index] {
			t.Fatalf("cached[%d] = %#v, want %#v", index, cached[index], want[index])
		}
	}
	if offline.calls != 0 {
		t.Fatalf("CachedSchedule called the provider %d times", offline.calls)
	}
}

func TestScheduleRejectsOversizedFetchAndKeepsCache(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "schedule.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	provider := &fakeScheduleProvider{entries: []core.ScheduleEntry{{Ref: bangumiRef("1"), Title: "舊", Weekday: time.Monday}}}
	service := newWithDependencies(dependencies{store: store, source: &catalogSource{}, schedule: provider})
	t.Cleanup(func() { _ = service.Close() })
	if _, err := service.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	oversized := make([]core.ScheduleEntry, core.MaxScheduleEntries+1)
	for index := range oversized {
		oversized[index] = core.ScheduleEntry{Ref: bangumiRef(strconv.Itoa(index + 1)), Title: "作品", Weekday: time.Weekday(index % 7)}
	}
	provider.entries = oversized
	if _, err := service.Schedule(ctx); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("oversized fetch error = %v, want ErrInvalidInput", err)
	}
	cached, err := service.CachedSchedule(ctx)
	if err != nil || len(cached) != 1 || cached[0].Title != "舊" {
		t.Fatalf("cached after rejected fetch = %#v, %v", cached, err)
	}
}

func TestScheduleLinksShowsFromTheSourceCatalog(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "schedule.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	source := &catalogSource{catalog: []core.SourceAnime{
		{Ref: core.SourceRef{Provider: "anime1", ID: "1"}, Title: "轉生就是劍"},
		{Ref: core.SourceRef{Provider: "anime1", ID: "2"}, Title: "轉生就是劍 第二季"},
		{Ref: core.SourceRef{Provider: "anime1", ID: "3"}, Title: "不在時間表"},
	}}
	provider := &fakeScheduleProvider{entries: []core.ScheduleEntry{
		{Ref: bangumiRef("10"), Title: "轉生就是劍 第二季", Weekday: time.Wednesday},
		{Ref: bangumiRef("11"), Title: "藥屋少女的呢喃 第三季", Weekday: time.Saturday},
	}}
	service := newWithDependencies(dependencies{store: store, source: source, schedule: provider})
	t.Cleanup(func() { _ = service.Close() })
	items, err := service.Schedule(ctx)
	if err != nil || len(items) != 2 {
		t.Fatalf("items = %#v, %v", items, err)
	}
	linked, err := store.Anime(ctx, core.AnimeID(items[0].AnimeID))
	if err != nil || linked.Title != "轉生就是劍 第二季" {
		t.Fatalf("linked anime = %#v, %v; want the second season", linked, err)
	}
	if items[1].AnimeID != "" || items[1].Query != "藥屋少女的呢喃" {
		t.Fatalf("unknown show = %#v", items[1])
	}
	library, err := store.ListAnime(ctx)
	if err != nil || len(library) != 1 {
		t.Fatalf("library = %#v, %v; only the scheduled catalog anime is stored", library, err)
	}

	// Without the catalog the schedule still refreshes; known shows stay linked.
	source.err = errors.New("anime1 unreachable")
	provider.entries = append(provider.entries, core.ScheduleEntry{Ref: bangumiRef("12"), Title: "新作品", Weekday: time.Monday})
	items, err = service.Schedule(ctx)
	if err != nil || len(items) != 3 || items[1].AnimeID == "" {
		t.Fatalf("items without catalog = %#v, %v", items, err)
	}
}
