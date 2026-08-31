package sqlite

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDatabasePathRejectsUnsafeInputs(t *testing.T) {
	inputs := []string{
		"",
		"anime.sqlite",
		"./anime.sqlite",
		"file:///tmp/anime.sqlite",
		"https://example.com/anime.sqlite",
		`\\server\share\anime.sqlite`,
		`//server/share/anime.sqlite`,
		`\\.\PhysicalDrive0`,
		`\\?\C:\anime.sqlite`,
		`\??\C:\anime.sqlite`,
		"C:anime.sqlite",
		"anime\x00.sqlite",
	}
	for _, input := range inputs {
		if _, err := databasePath(input); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("databasePath(%q) error = %v, want ErrInvalidInput", input, err)
		}
	}
}

func TestDatabasePathRejectsSymlinkAndNonRegularPath(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "directory")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := databasePath(directory); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("directory path error = %v, want ErrInvalidInput", err)
	}

	target := filepath.Join(root, "target.sqlite")
	if err := os.WriteFile(target, []byte("sqlite target"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.sqlite")
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("creating a symlink requires additional Windows privileges: %v", err)
		}
		t.Fatal(err)
	}
	if _, err := databasePath(link); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("symlink path error = %v, want ErrInvalidInput", err)
	}
}

func TestOpenRoundTripsSpecialDatabasePath(t *testing.T) {
	name := "anime space 資料 ?#%.sqlite"
	if runtime.GOOS == "windows" {
		name = "anime space 資料 #%.sqlite"
	}
	path := filepath.Join(t.TempDir(), name)
	ctx := context.Background()
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "INSERT INTO settings(id, appearance, mpv_path, autoplay_next, resume_playback, language) VALUES (1, 0, '', 0, 0, 0)"); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database path was not created: %v", err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var id int
	if err := store.db.QueryRowContext(ctx, "SELECT id FROM settings WHERE id = 1").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Fatalf("round-trip setting id = %d", id)
	}
}

func TestOpenCreatesPrivateDatabaseAndParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not portable to Windows")
	}
	parent := filepath.Join(t.TempDir(), "private", "database")
	path := filepath.Join(parent, "anime.sqlite")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	parentInfo, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	if mode := parentInfo.Mode().Perm(); mode != 0o700 {
		t.Fatalf("parent mode = %o, want 700", mode)
	}
	databaseInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := databaseInfo.Mode().Perm(); mode != 0o600 {
		t.Fatalf("database mode = %o, want 600", mode)
	}
	for _, artifact := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(artifact)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode&0o077 != 0 {
			t.Fatalf("%s mode = %o, want no group/world bits", artifact, mode)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	store, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	databaseInfo, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := databaseInfo.Mode().Perm(); mode != 0o600 {
		t.Fatalf("reopened database mode = %o, want 600", mode)
	}
}

func TestDatabaseDSNUsesNoFollow(t *testing.T) {
	dsn := databaseDSN(filepath.Join(t.TempDir(), "anime.sqlite"))
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if value := parsed.Query().Get("nofollow"); value != "1" {
		t.Fatalf("nofollow = %q, want 1", value)
	}
}

func TestOpenRejectsMalformedDatabaseWithoutLeakingDetails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malformed.sqlite")
	contents := "not a SQLite database: " + path
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Open(context.Background(), path)
	if err == nil {
		t.Fatal("Open accepted a malformed database")
	}
	if !errors.Is(err, ErrStorage) && !errors.Is(err, ErrMigration) {
		t.Fatalf("malformed database error = %v, want a sanitized storage or migration error", err)
	}
	if strings.Contains(err.Error(), path) || strings.Contains(err.Error(), contents) {
		t.Fatalf("malformed database error leaked details: %v", err)
	}
}
