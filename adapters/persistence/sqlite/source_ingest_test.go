// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"animeportable/core"
)

func TestSourceIngestAtomicRollback(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`CREATE TRIGGER reject_ref BEFORE INSERT ON source_refs BEGIN SELECT RAISE(ABORT, 'secret'); END`); err != nil {
		t.Fatal(err)
	}
	result, err := store.IngestSourceAnime(ctx, core.Anime{ID: "candidate", Title: "A"}, core.SourceRef{Provider: "p", ID: "1"})
	if !errors.Is(err, ErrStorage) || result != (core.Anime{}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	items, err := store.ListAnime(ctx)
	if err != nil || len(items) != 0 {
		t.Fatalf("orphan rows=%v err=%v", items, err)
	}
	if _, err := store.db.Exec(`DROP TRIGGER reject_ref`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.IngestSourceAnime(ctx, core.Anime{ID: "retry", Title: "A"}, core.SourceRef{Provider: "p", ID: "1"}); err != nil {
		t.Fatal(err)
	}
}

func TestSourceIngestReopenAndCollision(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	ref := core.SourceRef{Provider: "p", ID: "1"}
	first := core.Anime{ID: "first", Title: "A", Description: "Keep"}
	if _, err := store.IngestSourceAnime(ctx, first, ref); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	got, err := store.IngestSourceAnime(ctx, core.Anime{ID: "new", Title: "Updated"}, ref)
	if err != nil || got.ID != first.ID || got.Description != "Keep" || got.Title != "Updated" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if _, err := store.IngestSourceAnime(ctx, first, core.SourceRef{Provider: "p", ID: "2"}); !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("collision=%v", err)
	}
	persisted, err := store.Anime(ctx, first.ID)
	if err != nil || persisted != got {
		t.Fatalf("collision overwrote owner: %+v %v", persisted, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.IngestSourceAnime(canceled, core.Anime{ID: "canceled"}, ref); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestSourceIngestConcurrentStores(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	first, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var wg sync.WaitGroup
	results := make(chan core.AnimeID, 12)
	for i := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store := []*Store{first, second}[i%2]
			got, err := store.IngestSourceAnime(ctx, core.Anime{ID: core.AnimeID(fmt.Sprintf("candidate-%d", i)), Title: "A"}, core.SourceRef{Provider: "p", ID: "1"})
			if err != nil {
				t.Error(err)
				return
			}
			results <- got.ID
		}()
	}
	wg.Wait()
	close(results)
	items, err := first.ListAnime(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	count := 0
	for id := range results {
		count++
		if id != items[0].ID {
			t.Errorf("different owner %s", id)
		}
	}
	if count != 12 {
		t.Fatalf("successful writes=%d", count)
	}
}
