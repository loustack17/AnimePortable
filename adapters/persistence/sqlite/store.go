// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const migrationRollbackTimeout = 5 * time.Second

var openGate = make(chan struct{}, 1)

type Store struct {
	db        *sql.DB
	mu        sync.Mutex
	cond      *sync.Cond
	active    int
	closed    bool
	closeDone chan struct{}
	closeErr  error
}

func Open(ctx context.Context, path string) (*Store, error) {
	return open(ctx, path, embeddedMigrations)
}

func open(ctx context.Context, rawPath string, source migrationSource) (*Store, error) {
	if ctx == nil {
		return nil, ErrInvalidInput
	}
	select {
	case openGate <- struct{}{}:
		defer func() { <-openGate }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := databasePath(rawPath)
	if err != nil {
		return nil, err
	}
	created, err := prepareDatabaseFile(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", databaseDSN(path))
	if err != nil {
		return nil, ErrStorage
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, sanitizeError(ctx, err, ErrStorage)
	}
	if err := migrate(ctx, db, source, created); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := secureDatabaseArtifacts(path); err != nil {
		_ = db.Close()
		return nil, err
	}
	store := &Store{db: db, closeDone: make(chan struct{})}
	store.cond = sync.NewCond(&store.mu)
	return store, nil
}

func (store *Store) begin(ctx context.Context) (func(), error) {
	if ctx == nil {
		return nil, ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	store.mu.Lock()
	if store.closed {
		store.mu.Unlock()
		return nil, ErrClosed
	}
	store.active++
	store.mu.Unlock()
	if err := ctx.Err(); err != nil {
		store.end()
		return nil, err
	}
	return store.end, nil
}

func (store *Store) end() {
	store.mu.Lock()
	store.active--
	if store.active == 0 {
		store.cond.Broadcast()
	}
	store.mu.Unlock()
}

func (store *Store) withDB(ctx context.Context, fn func(*sql.DB) error) error {
	end, err := store.begin(ctx)
	if err != nil {
		return err
	}
	defer end()
	if fn == nil {
		return ErrInvalidInput
	}
	return sanitizeError(ctx, fn(store.db), ErrStorage)
}

func (store *Store) Close() error {
	store.mu.Lock()
	if store.closed {
		done := store.closeDone
		store.mu.Unlock()
		<-done
		store.mu.Lock()
		err := store.closeErr
		store.mu.Unlock()
		return err
	}
	store.closed = true
	for store.active > 0 {
		store.cond.Wait()
	}
	store.mu.Unlock()
	err := store.db.Close()
	store.mu.Lock()
	store.closeErr = sanitizeError(context.Background(), err, ErrStorage)
	close(store.closeDone)
	store.mu.Unlock()
	return store.closeErr
}
