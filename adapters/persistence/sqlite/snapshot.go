// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// SnapshotTo writes a consistent copy of the open database to target, which must not exist yet.
//
// VACUUM INTO runs on the store's own connection: in WAL mode it reads one
// consistent snapshot that already includes committed pages still in the
// -wal file, and it needs no second connection or sidecar copy. The store
// has a single connection, so other store calls wait while it runs. The copy
// is written to a staging file, checked with the same rules as an import, and
// then moved into place without replacing anything.
func (store *Store) SnapshotTo(ctx context.Context, target string) error {
	path, err := absoluteLocalPath(target)
	if err != nil {
		return ErrInvalidInput
	}
	dir := filepath.Dir(path)
	if err := ValidatePortablePath(dir); err != nil {
		return ErrInvalidInput
	}
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
		return ErrInvalidInput
	}
	if _, err := os.Lstat(path); err == nil {
		return ErrImportConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return ErrStorage
	}
	temp, err := os.CreateTemp(dir, ".animeportable-snapshot-*")
	if err != nil {
		return ErrStorage
	}
	stage := temp.Name()
	if err := temp.Close(); err != nil {
		removeImportStage(stage)
		return ErrStorage
	}
	defer removeImportStage(stage)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(stage, 0o600); err != nil {
			return ErrStorage
		}
	}
	// SQLite accepts an existing target only when it is empty, which is what CreateTemp leaves.
	err = store.withDB(ctx, func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, "VACUUM INTO ?", stage)
		return err
	})
	if err != nil {
		return err
	}
	if err := validatePortableSnapshot(ctx, stage); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return ErrMigration
	}
	if err := rejectSymlinkAncestors(dir); err != nil {
		return ErrInvalidInput
	}
	if err := installPortableSnapshot(stage, path); err != nil {
		if _, statErr := os.Lstat(path); statErr == nil {
			return ErrImportConflict
		}
		return ErrStorage
	}
	return nil
}

// ValidateDatabase reports whether path is an intact database of this application
// whose schema this build understands. It opens the file read-only and never migrates it.
func ValidateDatabase(ctx context.Context, path string) error {
	if ctx == nil {
		return ErrInvalidInput
	}
	clean, err := absoluteLocalPath(path)
	if err != nil {
		return ErrInvalidInput
	}
	if err := ValidatePortablePath(clean); err != nil {
		return ErrInvalidInput
	}
	if err := validateImportSourceFiles(clean); err != nil {
		return ErrInvalidInput
	}
	if err := validatePortableSnapshot(ctx, clean); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return ErrMigration
	}
	return nil
}
