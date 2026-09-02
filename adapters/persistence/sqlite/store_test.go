// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestOpenFreshAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "anime.sqlite")
	ctx := context.Background()
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	var migrations int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if migrations != 2 {
		t.Fatalf("migration count = %d", migrations)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if migrations != 2 {
		t.Fatalf("reopen migration count = %d", migrations)
	}
}

func TestOpenConfiguresSQLite(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "anime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var foreignKeys int
	if err := store.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d", foreignKeys)
	}
	var busyTimeout int
	if err := store.db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d", busyTimeout)
	}
	var journalMode string
	if err := store.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" && journalMode != "WAL" {
		t.Fatalf("journal_mode = %q", journalMode)
	}
}

func TestCloseIsIdempotentAndRejectsNewOperations(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "anime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.withDB(context.Background(), func(*sql.DB) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed operation error = %v", err)
	}
}
