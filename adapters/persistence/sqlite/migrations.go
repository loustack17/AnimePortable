// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

type migration struct {
	version int
	name    string
	sql     string
}

type migrationSource struct {
	files      embed.FS
	migrations []migration
}

var embeddedMigrations = migrationSource{files: migrationFS}

func loadMigrations(source migrationSource) ([]migration, error) {
	if source.migrations != nil {
		migrations := append([]migration(nil), source.migrations...)
		return migrations, validateMigrationList(migrations)
	}
	entries, err := fs.ReadDir(source.files, "migrations")
	if err != nil {
		return nil, ErrMigration
	}
	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			return nil, ErrMigration
		}
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, ErrMigration
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil || version <= 0 {
			return nil, ErrMigration
		}
		contents, err := fs.ReadFile(source.files, "migrations/"+entry.Name())
		if err != nil || len(contents) == 0 {
			return nil, ErrMigration
		}
		migrations = append(migrations, migration{version: version, name: entry.Name(), sql: string(contents)})
	}
	return migrations, validateMigrationList(migrations)
}

func validateMigrationList(migrations []migration) error {
	if len(migrations) == 0 {
		return ErrMigration
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
	for index, item := range migrations {
		if item.version != index+1 || item.name == "" || item.sql == "" {
			return ErrMigration
		}
	}
	return nil
}

func migrationChecksum(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func migrate(ctx context.Context, db *sql.DB, source migrationSource, created bool) error {
	migrations, err := loadMigrations(source)
	if err != nil {
		return ErrMigration
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return sanitizeError(ctx, err, ErrMigration)
	}
	defer conn.Close()
	began := false
	fail := func(operationErr error) error {
		if began {
			rollbackMigration(conn)
		}
		return sanitizeError(ctx, operationErr, ErrMigration)
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fail(err)
	}
	began = true
	hasTracking, err := migrationTableExists(ctx, conn)
	if err != nil {
		return fail(err)
	}
	if hasTracking {
		if err := validateMigrationTable(ctx, conn); err != nil {
			return fail(err)
		}
		count, err := appliedMigrationCount(ctx, conn)
		if err != nil {
			return fail(err)
		}
		if count == 0 {
			return fail(ErrMigration)
		}
	} else {
		if !created {
			hasObjects, err := hasUserSchemaObjects(ctx, conn)
			if err != nil {
				return fail(err)
			}
			if hasObjects {
				return fail(ErrMigration)
			}
		}
		if _, err := conn.ExecContext(ctx, migrationBootstrapSQL); err != nil {
			return fail(err)
		}
	}
	applied, latestSchemaChecksum, err := readAppliedMigrations(ctx, conn, migrations)
	if err != nil {
		return fail(err)
	}
	if latestSchemaChecksum != "" {
		currentChecksum, err := schemaFingerprint(ctx, conn)
		if err != nil {
			return fail(err)
		}
		if currentChecksum != latestSchemaChecksum {
			return fail(ErrMigration)
		}
	}
	for _, item := range migrations {
		if _, ok := applied[item.version]; ok {
			continue
		}
		if _, err := conn.ExecContext(ctx, item.sql); err != nil {
			return fail(err)
		}
		schemaChecksum, err := schemaFingerprint(ctx, conn)
		if err != nil {
			return fail(err)
		}
		appliedAt, err := encodeStoredTime(time.Now().UTC())
		if err != nil {
			return fail(err)
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations(version, name, checksum, schema_checksum, applied_at) VALUES (?, ?, ?, ?, ?)", item.version, item.name, migrationChecksum(item.sql), schemaChecksum, appliedAt); err != nil {
			return fail(err)
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fail(err)
	}
	return nil
}

func readAppliedMigrations(ctx context.Context, conn *sql.Conn, migrations []migration) (map[int]struct{}, string, error) {
	known := make(map[int]migration, len(migrations))
	for _, item := range migrations {
		known[item.version] = item
	}
	rows, err := conn.QueryContext(ctx, "SELECT version, name, checksum, schema_checksum, applied_at FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	applied := make(map[int]struct{}, len(migrations))
	maxVersion := 0
	latestSchemaChecksum := ""
	for rows.Next() {
		var version int
		var name, checksum, schemaChecksum, appliedAt string
		if err := rows.Scan(&version, &name, &checksum, &schemaChecksum, &appliedAt); err != nil {
			return nil, "", err
		}
		item, ok := known[version]
		if !ok || version <= 0 || name != item.name || checksum != migrationChecksum(item.sql) || !isSHA256Hex(schemaChecksum) || appliedAt == "" {
			return nil, "", ErrMigration
		}
		if _, exists := applied[version]; exists {
			return nil, "", ErrMigration
		}
		if _, err := decodeStoredTime(appliedAt); err != nil {
			return nil, "", ErrMigration
		}
		applied[version] = struct{}{}
		if version > maxVersion {
			maxVersion = version
			latestSchemaChecksum = schemaChecksum
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	for version := 1; version <= maxVersion; version++ {
		if _, ok := applied[version]; !ok {
			return nil, "", ErrMigration
		}
	}
	return applied, latestSchemaChecksum, nil
}

func rollbackMigration(conn *sql.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), migrationRollbackTimeout)
	defer cancel()
	_, _ = conn.ExecContext(ctx, "ROLLBACK")
}

var migrationBootstrapSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY CHECK (version > 0),
    name TEXT NOT NULL CHECK (length(name) > 0),
    checksum TEXT NOT NULL CHECK (length(checksum) = 64),
    schema_checksum TEXT NOT NULL CHECK (length(schema_checksum) = 64),
    applied_at TEXT NOT NULL CHECK (length(applied_at) = 30)
)`

func validateMigrationTable(ctx context.Context, conn *sql.Conn) error {
	var tableType string
	var definition string
	if err := conn.QueryRowContext(ctx, "SELECT type, sql FROM sqlite_master WHERE name = 'schema_migrations'").Scan(&tableType, &definition); err != nil {
		return err
	}
	want := strings.Replace(normalizeSQL(migrationBootstrapSQL), " if not exists", "", 1)
	if tableType != "table" || normalizeSQL(definition) != want {
		return ErrMigration
	}
	return nil
}

func migrationTableExists(ctx context.Context, conn *sql.Conn) (bool, error) {
	var count int
	err := conn.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'").Scan(&count)
	return count == 1, err
}

func hasUserSchemaObjects(ctx context.Context, conn *sql.Conn) (bool, error) {
	var count int
	err := conn.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master
WHERE type IN ('table', 'index', 'view', 'trigger') AND name NOT LIKE 'sqlite_%'`).Scan(&count)
	return count > 0, err
}

func appliedMigrationCount(ctx context.Context, conn *sql.Conn) (int, error) {
	var count int
	err := conn.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count)
	return count, err
}

func schemaFingerprint(ctx context.Context, conn *sql.Conn) (string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT type, name, tbl_name, sql FROM sqlite_master
WHERE type IN ('table', 'index', 'view', 'trigger')
  AND name != 'schema_migrations'
  AND name NOT LIKE 'sqlite_%'
ORDER BY type, name, tbl_name, sql`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	hash := sha256.New()
	for rows.Next() {
		var objectType, name, tableName string
		var definition sql.NullString
		if err := rows.Scan(&objectType, &name, &tableName, &definition); err != nil {
			return "", err
		}
		for _, value := range []string{objectType, name, tableName, definition.String} {
			if _, err := fmt.Fprintf(hash, "%d:%s", len(value), value); err != nil {
				return "", err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func normalizeSQL(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
