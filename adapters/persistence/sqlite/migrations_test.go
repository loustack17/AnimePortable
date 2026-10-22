// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/core"
)

func TestOpenConcurrentlyAppliesMigrationsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	source := testMigrationSource(false)
	const openerCount = 4

	start := make(chan struct{})
	stores := make(chan *Store, openerCount)
	errs := make(chan error, openerCount)
	var group sync.WaitGroup
	group.Add(openerCount)
	for range openerCount {
		go func() {
			defer group.Done()
			<-start
			store, err := open(context.Background(), path, source)
			if err != nil {
				errs <- err
				return
			}
			stores <- store
		}()
	}
	close(start)
	group.Wait()
	close(stores)
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Open failed: %v", err)
	}
	for store := range stores {
		if err := store.Close(); err != nil {
			t.Errorf("closing concurrent store: %v", err)
		}
	}

	store, err := open(context.Background(), path, source)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var count int
	if err := store.db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("applied migration count = %d, want 1", count)
	}
}

func TestOpenRejectsMalformedMigrationList(t *testing.T) {
	tests := []struct {
		name       string
		migrations []migration
	}{
		{"gap", []migration{{version: 2, name: "0002_gap.sql", sql: "SELECT 1"}}},
		{"duplicate", []migration{
			{version: 1, name: "0001_first.sql", sql: "SELECT 1"},
			{version: 1, name: "0001_duplicate.sql", sql: "SELECT 2"},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := open(context.Background(), filepath.Join(t.TempDir(), "anime.sqlite"), migrationSource{migrations: test.migrations})
			if !errors.Is(err, ErrMigration) {
				t.Fatalf("malformed migration list error = %v, want ErrMigration", err)
			}
		})
	}
}

func TestMigrationFailureRollsBackAllStatements(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	source := migrationSource{migrations: []migration{{
		version: 1,
		name:    "0001_failure.sql",
		sql:     "CREATE TABLE rollback_probe(id INTEGER PRIMARY KEY); SELECT * FROM missing_table;",
	}}}
	if _, err := open(context.Background(), path, source); !errors.Is(err, ErrMigration) {
		t.Fatalf("Open error = %v, want ErrMigration", err)
	}
	db := openRawDatabase(t, path)
	defer db.Close()
	if tableExists(t, db, "rollback_probe") || tableExists(t, db, "schema_migrations") {
		t.Fatal("failed migration left partial schema")
	}
}

func TestMigrationCancellationRollsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	source := migrationSource{migrations: []migration{{
		version: 1,
		name:    "0001_cancel.sql",
		sql: `CREATE TABLE cancel_probe(id INTEGER PRIMARY KEY);
WITH RECURSIVE counter(value) AS (
    VALUES(0)
    UNION ALL
    SELECT value + 1 FROM counter WHERE value < 100000000
)
SELECT sum(value) FROM counter;`,
	}}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := open(ctx, path, source)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Open error = %v, want context.DeadlineExceeded", err)
	}
	db := openRawDatabase(t, path)
	defer db.Close()
	if tableExists(t, db, "cancel_probe") || tableExists(t, db, "schema_migrations") {
		t.Fatal("canceled migration left partial schema")
	}
}

func TestMigrationRejectsPreexistingPartialDomainSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	db := openRawDatabase(t, path)
	if _, err := db.Exec("CREATE TABLE anime(id TEXT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(context.Background(), path); !errors.Is(err, ErrMigration) {
		t.Fatalf("Open error = %v, want ErrMigration", err)
	}
	db = openRawDatabase(t, path)
	defer db.Close()
	if !tableExists(t, db, "anime") {
		t.Fatal("partial source schema was not preserved")
	}
	if tableExists(t, db, "schema_migrations") {
		t.Fatal("partial source schema was incorrectly certified")
	}
}

func settingsColumns(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query("SELECT name FROM pragma_table_info('settings') ORDER BY cid")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return columns
}

func TestMigrationDropsLegacySettingsMPVPathPreservingSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	var previous []migration
	for version, name := range map[int]string{1: "0001_initial.sql", 2: "0002_episode_mappings.sql"} {
		contents, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		previous = append(previous, migration{version: version, name: name, sql: string(contents)})
	}
	store, err := open(context.Background(), path, migrationSource{migrations: previous})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(settingsColumns(t, store.db), "mpv_path") {
		t.Fatal("previous schema is missing mpv_path")
	}
	if _, err := store.db.Exec(`INSERT INTO settings(id, appearance, mpv_path, autoplay_next, resume_playback, language)
		VALUES (1, 3, 'C:/private/old-mpv.exe', 2, 1, 1)`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	want := []string{"id", "appearance", "autoplay_next", "resume_playback", "language", "auto_play", "playback_speed", "hardware_decoding", "proxy_mode", "proxy_address"}
	if got := settingsColumns(t, store.db); !slices.Equal(got, want) {
		t.Fatalf("settings columns = %v, want %v", got, want)
	}
	var appearance, autoplay, resume, language int
	if err := store.db.QueryRow("SELECT appearance, autoplay_next, resume_playback, language FROM settings WHERE id = 1").
		Scan(&appearance, &autoplay, &resume, &language); err != nil {
		t.Fatal(err)
	}
	if appearance != 3 || autoplay != 2 || resume != 1 || language != 1 {
		t.Fatalf("preserved settings = %d %d %d %d, want 3 2 1 1", appearance, autoplay, resume, language)
	}
	if _, err := store.db.Exec("INSERT INTO settings(id, appearance, autoplay_next, resume_playback, language) VALUES (2, 0, 0, 0, 0)"); err == nil {
		t.Fatal("rebuilt settings table lost its id = 1 constraint")
	}
	if _, err := store.db.Exec("UPDATE settings SET language = 4"); err == nil {
		t.Fatal("rebuilt settings table lost its language constraint")
	}
}

func TestMigrationAddsAutoPlayKeepingSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	var previous []migration
	for version, name := range []string{"0001_initial.sql", "0002_episode_mappings.sql", "0003_drop_settings_mpv_path.sql", "0004_episode_number.sql", "0005_recent_updates.sql", "0006_schedule.sql"} {
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
	if _, err := legacy.db.Exec(`INSERT INTO settings(id, appearance, autoplay_next, resume_playback, language)
		VALUES (1, 3, 1, 2, 2)`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	store := openLibraryStore(t, path)
	defer store.Close()
	want := core.Settings{Appearance: core.AppearanceDark, AutoplayNext: core.ToggleDisabled, ResumePlayback: core.ToggleEnabled, Language: core.LanguageEnglish}
	if got, err := store.Settings(context.Background()); err != nil || got != want {
		t.Fatalf("settings after migration = %#v, %v, want %#v", got, err, want)
	}
	if _, err := store.db.Exec("UPDATE settings SET auto_play = 3"); err == nil {
		t.Fatal("auto_play accepted a value outside 0-2")
	}
}

func TestMigrationAddsPlaybackAndProxyKeepingSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	var previous []migration
	for version, name := range []string{"0001_initial.sql", "0002_episode_mappings.sql", "0003_drop_settings_mpv_path.sql", "0004_episode_number.sql", "0005_recent_updates.sql", "0006_schedule.sql", "0007_settings_auto_play.sql"} {
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
	if _, err := legacy.db.Exec(`INSERT INTO settings(id, appearance, autoplay_next, auto_play, resume_playback, language)
		VALUES (1, 3, 1, 1, 2, 2)`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	store := openLibraryStore(t, path)
	defer store.Close()
	want := core.Settings{
		Appearance: core.AppearanceDark, AutoplayNext: core.ToggleDisabled, AutoPlay: core.ToggleDisabled,
		ResumePlayback: core.ToggleEnabled, Language: core.LanguageEnglish,
	}
	if got, err := store.Settings(context.Background()); err != nil || got != want {
		t.Fatalf("settings after migration = %#v, %v, want %#v", got, err, want)
	}
	for name, statement := range map[string]string{
		"playback_speed":    "UPDATE settings SET playback_speed = 110",
		"hardware_decoding": "UPDATE settings SET hardware_decoding = 3",
		"proxy_mode":        "UPDATE settings SET proxy_mode = 4",
		"proxy_address":     "UPDATE settings SET proxy_address = '" + strings.Repeat("a", 256) + "'",
	} {
		if _, err := store.db.Exec(statement); err == nil {
			t.Errorf("%s accepted an out-of-range value", name)
		}
	}
}

func TestMigrationDropsSkipIntroKeepingSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	var previous []migration
	for version, name := range []string{"0001_initial.sql", "0002_episode_mappings.sql", "0003_drop_settings_mpv_path.sql", "0004_episode_number.sql", "0005_recent_updates.sql", "0006_schedule.sql", "0007_settings_auto_play.sql", "0008_settings_playback_proxy.sql"} {
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
	if _, err := legacy.db.Exec(`INSERT INTO settings(id, appearance, autoplay_next, auto_play, resume_playback, language,
		playback_speed, skip_intro_seconds, hardware_decoding, proxy_mode, proxy_address)
		VALUES (1, 3, 1, 1, 2, 2, 150, 90, 1, 3, 'proxy.example:8080')`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	store := openLibraryStore(t, path)
	defer store.Close()
	if slices.Contains(settingsColumns(t, store.db), "skip_intro_seconds") {
		t.Fatal("skip_intro_seconds is still in the settings table")
	}
	want := core.Settings{
		Appearance: core.AppearanceDark, AutoplayNext: core.ToggleDisabled, AutoPlay: core.ToggleDisabled,
		ResumePlayback: core.ToggleEnabled, Language: core.LanguageEnglish,
		PlaybackSpeed: 150, HardwareDecoding: core.ToggleDisabled,
		ProxyMode: core.ProxyModeManual, ProxyAddress: "proxy.example:8080",
	}
	if got, err := store.Settings(context.Background()); err != nil || got != want {
		t.Fatalf("settings after migration = %#v, %v, want %#v", got, err, want)
	}
	if _, err := store.db.Exec("UPDATE settings SET playback_speed = 110"); err == nil {
		t.Fatal("the other settings constraints were lost")
	}
}

func TestMigrationAllowsJapaneseKeepingSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	var previous []migration
	for version, name := range []string{"0001_initial.sql", "0002_episode_mappings.sql", "0003_drop_settings_mpv_path.sql", "0004_episode_number.sql", "0005_recent_updates.sql", "0006_schedule.sql", "0007_settings_auto_play.sql", "0008_settings_playback_proxy.sql", "0009_settings_drop_skip_intro.sql"} {
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
	if _, err := legacy.db.Exec(`INSERT INTO settings(id, appearance, autoplay_next, auto_play, resume_playback, language,
		playback_speed, hardware_decoding, proxy_mode, proxy_address)
		VALUES (1, 3, 1, 1, 2, 2, 150, 1, 3, 'proxy.example:8080')`); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.db.Exec("UPDATE settings SET language = 3"); err == nil {
		t.Fatal("schema before 0010 already allowed Japanese")
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	store := openLibraryStore(t, path)
	defer store.Close()
	want := core.Settings{
		Appearance: core.AppearanceDark, AutoplayNext: core.ToggleDisabled, AutoPlay: core.ToggleDisabled,
		ResumePlayback: core.ToggleEnabled, Language: core.LanguageEnglish,
		PlaybackSpeed: 150, HardwareDecoding: core.ToggleDisabled,
		ProxyMode: core.ProxyModeManual, ProxyAddress: "proxy.example:8080",
	}
	if got, err := store.Settings(context.Background()); err != nil || got != want {
		t.Fatalf("settings after migration = %#v, %v, want %#v", got, err, want)
	}
	want.Language = core.LanguageJapanese
	if err := store.SaveSettings(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Settings(context.Background()); err != nil || got != want {
		t.Fatalf("Japanese settings = %#v, %v, want %#v", got, err, want)
	}
	for _, update := range []string{"UPDATE settings SET language = 4", "UPDATE settings SET playback_speed = 110", "UPDATE settings SET proxy_mode = 4"} {
		if _, err := store.db.Exec(update); err == nil {
			t.Fatalf("%s succeeded; the rebuilt table lost a constraint", update)
		}
	}
}
func TestFreshSchemaHasNoSettingsMPVPath(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	want := []string{"id", "appearance", "autoplay_next", "resume_playback", "language", "auto_play", "playback_speed", "hardware_decoding", "proxy_mode", "proxy_address"}
	if got := settingsColumns(t, store.db); !slices.Equal(got, want) {
		t.Fatalf("settings columns = %v, want %v", got, want)
	}
}

func openRawDatabase(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", databaseDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	return db
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count == 1
}

func TestOpenRejectsPartialFutureAndChecksumMigrationHistory(t *testing.T) {
	tests := []struct {
		name   string
		source migrationSource
		mutate func(t *testing.T, store *Store)
	}{
		{
			name:   "partial history",
			source: testMigrationSource(true),
			mutate: func(t *testing.T, store *Store) {
				t.Helper()
				if _, err := store.db.Exec("DELETE FROM schema_migrations WHERE version = 1"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:   "future version",
			source: testMigrationSource(false),
			mutate: func(t *testing.T, store *Store) {
				t.Helper()
				if _, err := store.db.Exec(
					"INSERT INTO schema_migrations(version, name, checksum, schema_checksum, applied_at) VALUES (?, ?, ?, ?, ?)",
					2,
					"0002_future.sql",
					migrationChecksum("SELECT 2"),
					strings.Repeat("0", 64),
					"2026-08-30T00:00:00.000000000Z",
				); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:   "checksum mismatch",
			source: testMigrationSource(false),
			mutate: func(t *testing.T, store *Store) {
				t.Helper()
				if _, err := store.db.Exec("UPDATE schema_migrations SET checksum = ? WHERE version = 1", migrationChecksum("incorrect")); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "anime.sqlite")
			store, err := open(context.Background(), path, test.source)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(t, store)
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}

			store, err = open(context.Background(), path, test.source)
			if store != nil {
				_ = store.Close()
			}
			if !errors.Is(err, ErrMigration) {
				t.Fatalf("Open error = %v, want ErrMigration", err)
			}
			if err != nil && err.Error() != ErrMigration.Error() {
				t.Fatalf("Open leaked migration details: %v", err)
			}
		})
	}
}

func TestOpenRejectsExistingUntrackedSchemaWithoutChangingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	db := openRawDatabase(t, path)
	if _, err := db.Exec(`CREATE TABLE unrelated (id INTEGER PRIMARY KEY, value TEXT NOT NULL); INSERT INTO unrelated(value) VALUES ('keep'); CREATE INDEX unrelated_value ON unrelated(value); CREATE VIEW unrelated_view AS SELECT value FROM unrelated; CREATE TRIGGER unrelated_trigger AFTER INSERT ON unrelated BEGIN SELECT 1; END;`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(context.Background(), path); !errors.Is(err, ErrMigration) {
		t.Fatalf("Open error = %v, want ErrMigration", err)
	}
	db = openRawDatabase(t, path)
	defer db.Close()
	for _, name := range []string{"unrelated", "unrelated_value", "unrelated_view", "unrelated_trigger"} {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name = ?", name).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("schema object %q was changed", name)
		}
	}
	var value string
	if err := db.QueryRow("SELECT value FROM unrelated").Scan(&value); err != nil || value != "keep" {
		t.Fatalf("unrelated data changed: value=%q err=%v", value, err)
	}
	if tableExists(t, db, "schema_migrations") {
		t.Fatal("untracked database was modified with migration tracking")
	}
}

func TestOpenAllowsExistingEmptyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	db := openRawDatabase(t, path)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if !tableExists(t, store.db, "schema_migrations") {
		t.Fatal("existing empty database was not initialized")
	}
}

func TestOpenRejectsZeroAppliedTrackingTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	db := openRawDatabase(t, path)
	if _, err := db.Exec(migrationBootstrapSQL); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), path); !errors.Is(err, ErrMigration) {
		t.Fatalf("Open error = %v, want ErrMigration", err)
	}
	db = openRawDatabase(t, path)
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != 0 {
		t.Fatalf("tracking history changed: count=%d err=%v", count, err)
	}
}

func TestOpenRejectsSchemaTamperingWithoutFurtherChanges(t *testing.T) {
	tests := []struct {
		name   string
		mutate string
	}{
		{"drop table", "DROP TABLE fingerprint_probe"},
		{"drop history index", "DROP INDEX idx_history_last_played"},
		{"add table", "CREATE TABLE attacker_probe(id INTEGER PRIMARY KEY)"},
		{"add secret column", "ALTER TABLE fingerprint_probe ADD COLUMN secret TEXT"},
		{"rebuild weaker constraint", "ALTER TABLE fingerprint_probe RENAME TO fingerprint_probe_old; CREATE TABLE fingerprint_probe(id INTEGER PRIMARY KEY, value TEXT NOT NULL); INSERT INTO fingerprint_probe SELECT id, value FROM fingerprint_probe_old; DROP TABLE fingerprint_probe_old"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "anime.sqlite")
			source := migrationSource{migrations: []migration{{version: 1, name: "0001_probe.sql", sql: "CREATE TABLE fingerprint_probe(id INTEGER PRIMARY KEY CHECK (id > 0), value TEXT NOT NULL CHECK (length(value) > 0)); CREATE INDEX idx_history_last_played ON fingerprint_probe(value)"}}}
			store, err := open(context.Background(), path, source)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			db := openRawDatabase(t, path)
			if _, err := db.Exec(test.mutate); err != nil {
				_ = db.Close()
				t.Fatal(err)
			}
			before := schemaCatalog(t, db)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := open(context.Background(), path, source); !errors.Is(err, ErrMigration) {
				t.Fatalf("Open error = %v, want ErrMigration", err)
			}
			db = openRawDatabase(t, path)
			defer db.Close()
			if after := schemaCatalog(t, db); after != before {
				t.Fatalf("failed Open changed tampered schema:\nbefore: %s\nafter:  %s", before, after)
			}
		})
	}
}

func schemaCatalog(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query("SELECT type || ':' || name || ':' || tbl_name || ':' || coalesce(sql, '') FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name, tbl_name, sql")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return strings.Join(values, "\n")
}

func testMigrationSource(withSecond bool) migrationSource {
	migrations := []migration{{
		version: 1,
		name:    "0001_first.sql",
		sql:     "CREATE TABLE first_migration_test(id INTEGER PRIMARY KEY)",
	}}
	if withSecond {
		migrations = append(migrations, migration{
			version: 2,
			name:    "0002_second.sql",
			sql:     "CREATE TABLE second_migration_test(id INTEGER PRIMARY KEY)",
		})
	}
	return migrationSource{migrations: migrations}
}
