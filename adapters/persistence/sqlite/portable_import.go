// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	modernsqlite "modernc.org/sqlite"
)

var (
	ErrImportConflict      = errors.New("portable import destination already exists")
	ErrImportUnavailable   = errors.New("portable import unavailable")
	ErrImportInvalidSource = errors.New("portable import source is invalid or incompatible")
)

func ImportPortable(ctx context.Context, legacyPath, targetPath string) error {
	if ctx == nil || ctx.Err() != nil {
		if ctx == nil {
			return ErrInvalidInput
		}
		return ctx.Err()
	}
	source, err := importPath(legacyPath)
	if err != nil {
		return ErrImportInvalidSource
	}
	target, err := importPath(targetPath)
	if err != nil {
		return ErrImportUnavailable
	}
	if samePath(source, target) {
		return ErrImportInvalidSource
	}
	if err := rejectSymlinkAncestors(source); err != nil {
		return ErrImportInvalidSource
	}
	if err := ValidatePortablePath(filepath.Dir(target)); err != nil {
		return ErrImportUnavailable
	}
	if err := validateImportSourceFiles(source); err != nil {
		return ErrImportInvalidSource
	}
	if _, err := os.Lstat(target); err == nil {
		return ErrImportConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return ErrImportUnavailable
	}
	if err := prepareImportDirectory(filepath.Dir(target)); err != nil {
		return ErrImportUnavailable
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".animeportable-import-*")
	if err != nil {
		return ErrImportUnavailable
	}
	stage := temp.Name()
	if err := temp.Close(); err != nil {
		removeImportStage(stage)
		return ErrImportUnavailable
	}
	defer removeImportStage(stage)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(stage, 0o600); err != nil {
			return ErrImportUnavailable
		}
	}
	if err := backupReadOnly(ctx, source, stage); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return ErrImportInvalidSource
	}
	if err := validatePortableSnapshot(ctx, stage); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return ErrImportInvalidSource
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := rejectSymlinkAncestors(filepath.Dir(target)); err != nil {
		return ErrImportUnavailable
	}
	if err := installPortableSnapshot(stage, target); err != nil {
		if _, statErr := os.Lstat(target); statErr == nil {
			return ErrImportConflict
		}
		return ErrImportUnavailable
	}
	return nil
}

func importPath(raw string) (string, error) {
	if raw == "" || strings.IndexByte(raw, 0) >= 0 || isRemoteOrDevicePath(raw) || !filepath.IsAbs(raw) {
		return "", ErrInvalidInput
	}
	return filepath.Clean(raw), nil
}

func ValidatePortablePath(path string) error {
	absolute, err := importPath(path)
	if err != nil {
		return ErrInvalidInput
	}
	current := absolute
	for {
		if err := rejectSymlinkAncestors(current); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return ErrInvalidInput
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ErrInvalidInput
		}
		current = parent
	}
}

func validateImportSourceFiles(source string) error {
	for _, path := range []string{source, source + "-wal", source + "-shm"} {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) && path != source {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return ErrImportInvalidSource
		}
	}
	return nil
}

func samePath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func rejectSymlinkAncestors(path string) error {
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	rest := strings.TrimPrefix(path, current)
	for _, component := range strings.Split(rest, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return ErrInvalidInput
		}
	}
	return nil
}

func readOnlySQLiteDSN(path string) string {
	slashPath := filepath.ToSlash(path)
	if isWindowsDrivePath(slashPath) {
		slashPath = "/" + slashPath
	}
	uri := url.URL{Scheme: "file", Path: slashPath}
	query := uri.Query()
	query.Set("mode", "ro")
	query.Set("nofollow", "1")
	query.Set("_busy_timeout", "5000")
	uri.RawQuery = query.Encode()
	return uri.String()
}

func backupReadOnly(ctx context.Context, source, stage string) error {
	if _, err := os.Lstat(source + "-wal"); err == nil {
		return backupWalFromPrivateCopy(ctx, source, stage)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return backupOnline(ctx, source, stage, true)
}

func backupWalFromPrivateCopy(ctx context.Context, source, stage string) error {
	stageDir, err := os.MkdirTemp(filepath.Dir(stage), ".animeportable-source-*")
	if err != nil {
		return err
	}
	clone := filepath.Join(stageDir, "source.db")
	defer removeImportDirectory(stageDir, clone)
	before, err := sourceFileHashes(source)
	if err != nil {
		return err
	}
	for _, suffix := range []string{"", "-wal"} {
		if err := copyImportSourceFile(source+suffix, clone+suffix); err != nil {
			return err
		}
	}
	after, err := sourceFileHashes(source)
	if err != nil || before != after {
		return ErrImportInvalidSource
	}
	return backupOnline(ctx, clone, stage, false)
}

func backupOnline(ctx context.Context, source, stage string, readOnly bool) error {
	dsn := readWriteSQLiteDSN(source)
	if readOnly {
		dsn = readOnlySQLiteDSN(source)
	}
	db, err := sql.Open("sqlite", dsn)
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
	return conn.Raw(func(driverConn any) error {
		backuper, ok := driverConn.(interface {
			NewBackup(string) (*modernsqlite.Backup, error)
		})
		if !ok {
			return ErrStorage
		}
		backup, err := backuper.NewBackup(readWriteSQLiteDSN(stage))
		if err != nil {
			return err
		}
		finished := false
		defer func() {
			if !finished {
				_ = backup.Finish()
			}
		}()
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			more, err := backup.Step(128)
			if err != nil {
				return err
			}
			if !more {
				err := backup.Finish()
				finished = true
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Millisecond):
			}
		}
	})
}

func sourceFileHashes(source string) ([2][32]byte, error) {
	var hashes [2][32]byte
	for index, suffix := range []string{"", "-wal"} {
		hash, err := hashRegularFile(source + suffix)
		if err != nil {
			return hashes, err
		}
		hashes[index] = hash
	}
	return hashes, nil
}

func hashRegularFile(path string) ([32]byte, error) {
	var empty [32]byte
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() {
		return empty, ErrImportInvalidSource
	}
	file, err := os.Open(path)
	if err != nil {
		return empty, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return empty, ErrImportInvalidSource
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return empty, err
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result, nil
}

func copyImportSourceFile(source, target string) error {
	before, err := os.Lstat(source)
	if err != nil || !before.Mode().IsRegular() {
		return ErrImportInvalidSource
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return ErrImportInvalidSource
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(output, hash), input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	var copied [32]byte
	copy(copied[:], hash.Sum(nil))
	current, err := hashRegularFile(source)
	if err != nil || current != copied {
		return ErrImportInvalidSource
	}
	return nil
}

func removeImportDirectory(path, database string) {
	for _, candidate := range []string{database, database + "-wal", database + "-shm", database + "-journal"} {
		info, err := os.Lstat(candidate)
		if err == nil && (info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
			_ = os.Remove(candidate)
		}
	}
	info, err := os.Lstat(path)
	if err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		_ = os.Remove(path)
	}
}

func readWriteSQLiteDSN(path string) string {
	slashPath := filepath.ToSlash(path)
	if isWindowsDrivePath(slashPath) {
		slashPath = "/" + slashPath
	}
	uri := url.URL{Scheme: "file", Path: slashPath}
	query := uri.Query()
	query.Set("mode", "rwc")
	query.Set("nofollow", "1")
	query.Set("_busy_timeout", "5000")
	uri.RawQuery = query.Encode()
	return uri.String()
}

func validatePortableSnapshot(ctx context.Context, path string) error {
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

func removeImportStage(path string) {
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Lstat(candidate)
		if err == nil && (info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
			_ = os.Remove(candidate)
		}
	}
}

func prepareImportDirectory(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return ErrInvalidInput
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(path)
	if err := rejectSymlinkAncestors(parent); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err = os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return ErrInvalidInput
	}
	return rejectSymlinkAncestors(path)
}
