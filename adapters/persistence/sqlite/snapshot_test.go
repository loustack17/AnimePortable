// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"animeportable/core"
)

func seedHistory(t *testing.T, store *Store) {
	t.Helper()
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	entry := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "anime", EpisodeID: "episode", Position: time.Minute}}
	if err := store.SavePlaybackCheckpoint(ctx, entry); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotToCopiesCommittedWalDataAndValidates(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, filepath.Join(dir, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	seedHistory(t, store)
	if _, err := os.Lstat(filepath.Join(dir, "live.db-wal")); err != nil {
		t.Fatalf("fixture must still have a WAL file: %v", err)
	}

	target := filepath.Join(t.TempDir(), "copy.db")
	if err := store.SnapshotTo(ctx, target); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDatabase(ctx, target); err != nil {
		t.Fatalf("snapshot does not validate: %v", err)
	}
	copied, err := Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer copied.Close()
	if history, err := copied.History(ctx); err != nil || len(history) != 1 {
		t.Fatalf("copied history = %#v, %v; want one row", history, err)
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if name := entry.Name(); name != "copy.db" && name != "copy.db-wal" && name != "copy.db-shm" {
			t.Fatalf("snapshot left %q behind", name)
		}
	}
}

func TestSnapshotToRefusesExistingAndInvalidTargets(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	existing := filepath.Join(t.TempDir(), "taken.db")
	if err := os.WriteFile(existing, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.SnapshotTo(ctx, existing); !errors.Is(err, ErrImportConflict) {
		t.Fatalf("existing target error = %v, want ErrImportConflict", err)
	}
	if got, err := os.ReadFile(existing); err != nil || string(got) != "keep me" {
		t.Fatalf("existing target changed: %q %v", got, err)
	}
	for _, target := range []string{"relative.db", `\\server\share\x.db`, `\\.\pipe\x`, filepath.Join(t.TempDir(), "missing", "x.db")} {
		if err := store.SnapshotTo(ctx, target); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("target %q error = %v, want ErrInvalidInput", target, err)
		}
	}
}

func TestValidateDatabaseRejectsForeignFiles(t *testing.T) {
	ctx := context.Background()
	garbage := filepath.Join(t.TempDir(), "garbage.db")
	if err := os.WriteFile(garbage, []byte("not a database at all, just text"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDatabase(ctx, garbage); err == nil {
		t.Fatal("garbage file was accepted")
	}
	if err := ValidateDatabase(ctx, filepath.Join(t.TempDir(), "absent.db")); err == nil {
		t.Fatal("missing file was accepted")
	}
	if err := ValidateDatabase(ctx, "relative.db"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("relative path error = %v, want ErrInvalidInput", err)
	}
}
