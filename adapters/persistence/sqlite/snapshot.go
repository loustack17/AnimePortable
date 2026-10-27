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
// is written to a staging file, checked by validateSnapshot, and
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
		return ErrTargetExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return ErrStorage
	}
	temp, err := os.CreateTemp(dir, ".animeportable-snapshot-*")
	if err != nil {
		return ErrStorage
	}
	stage := temp.Name()
	if err := temp.Close(); err != nil {
		removeStagedSnapshot(stage)
		return ErrStorage
	}
	defer removeStagedSnapshot(stage)
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
	if err := validateSnapshot(ctx, stage); err != nil {
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
			return ErrTargetExists
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
	if err := validateDatabaseFiles(clean); err != nil {
		return ErrInvalidInput
	}
	if err := validateSnapshot(ctx, clean); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return ErrMigration
	}
	return nil
}

// validateDatabaseFiles accepts a database whose file and any -wal or -shm sidecar are
// regular files.
func validateDatabaseFiles(source string) error {
	for _, path := range []string{source, source + "-wal", source + "-shm"} {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) && path != source {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return ErrInvalidInput
		}
	}
	return nil
}

// validateSnapshot opens path read-only and accepts it only when it is intact, has no
// foreign-key violations and carries this build's migrations and schema.
func validateSnapshot(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", readOnlySQLiteDSN(path))
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return ErrMigration
	}
	integrityRows := 0
	for rows.Next() {
		var integrity string
		if err := rows.Scan(&integrity); err != nil || integrity != "ok" {
			_ = rows.Close()
			return ErrMigration
		}
		integrityRows++
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return ErrMigration
	}
	if err := rows.Close(); err != nil || integrityRows != 1 {
		return ErrMigration
	}
	foreignKeys, err := conn.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return ErrMigration
	}
	if foreignKeys.Next() {
		_ = foreignKeys.Close()
		return ErrMigration
	}
	if err := foreignKeys.Err(); err != nil {
		_ = foreignKeys.Close()
		return ErrMigration
	}
	if err := foreignKeys.Close(); err != nil {
		return ErrMigration
	}
	if err := validateMigrationTable(ctx, conn); err != nil {
		return ErrMigration
	}
	migrations, err := loadMigrations(embeddedMigrations)
	if err != nil {
		return ErrMigration
	}
	applied, latestChecksum, err := readAppliedMigrations(ctx, conn, migrations)
	if err != nil || len(applied) == 0 {
		return ErrMigration
	}
	if latestChecksum != "" {
		fingerprint, err := schemaFingerprint(ctx, conn)
		if err != nil || fingerprint != latestChecksum {
			return ErrMigration
		}
	}
	return ctx.Err()
}

// removeStagedSnapshot deletes a staging file and its sidecars, never following a link.
func removeStagedSnapshot(path string) {
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Lstat(candidate)
		if err == nil && (info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
			_ = os.Remove(candidate)
		}
	}
}
