// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestImportPortableCopiesCommittedWALWithoutChangingSource(t *testing.T) {
	root := filepath.Join(t.TempDir(), "資料 with spaces")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "legacy.sqlite")
	target := filepath.Join(root, "data", "portable.sqlite")
	store, err := Open(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO anime(id, title, native_title, description) VALUES ('committed', 'Saved', '', '')"); err != nil {
		t.Fatal(err)
	}
	tx, err := store.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("INSERT INTO anime(id, title, native_title, description) VALUES ('uncommitted', 'Pending', '', '')"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source + "-wal"); err != nil {
		t.Fatalf("active WAL was not present: %v", err)
	}
	before := databaseArtifacts(t, source)
	if err := ImportPortable(context.Background(), source, target); err != nil {
		t.Fatalf("ImportPortable returned %v", err)
	}
	if after := databaseArtifacts(t, source); !equalArtifactHashes(before, after) {
		t.Fatal("legacy database or WAL artifacts changed during import")
	}
	imported, err := Open(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	defer imported.Close()
	var committed, uncommitted int
	if err := imported.db.QueryRow("SELECT count(*) FROM anime WHERE id='committed'").Scan(&committed); err != nil {
		t.Fatal(err)
	}
	if err := imported.db.QueryRow("SELECT count(*) FROM anime WHERE id='uncommitted'").Scan(&uncommitted); err != nil {
		t.Fatal(err)
	}
	if committed != 1 || uncommitted != 0 {
		t.Fatalf("imported committed=%d uncommitted=%d; want 1 and 0", committed, uncommitted)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestImportPortableReadsCommittedWALWithoutLegacyShm(t *testing.T) {
	root := t.TempDir()
	activeSource := filepath.Join(root, "active.sqlite")
	store, err := Open(context.Background(), activeSource)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO anime(id, title, native_title, description) VALUES ('wal-only', 'Saved', '', '')"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(activeSource + "-wal"); err != nil {
		t.Fatal(err)
	}
	legacyDir := filepath.Join(root, "legacy")
	if err := os.Mkdir(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(legacyDir, "animeportable.db")
	for _, suffix := range []string{"", "-wal"} {
		contents, err := os.ReadFile(activeSource + suffix)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(legacy+suffix, contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Lstat(legacy + "-shm"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source fixture unexpectedly has shm: %v", err)
	}
	before := databaseArtifacts(t, legacy)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "portable", "animeportable.db")
	if err := ImportPortable(context.Background(), legacy, target); err != nil {
		t.Fatalf("import without source shm: %v", err)
	}
	if _, err := os.Lstat(legacy + "-shm"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("import changed source listing by creating shm: %v", err)
	}
	if after := databaseArtifacts(t, legacy); !equalArtifactHashes(before, after) {
		t.Fatal("import changed legacy database or WAL bytes")
	}
	imported, err := Open(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	defer imported.Close()
	var count int
	if err := imported.db.QueryRow("SELECT count(*) FROM anime WHERE id='wal-only'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("imported wal-only row count = %d", count)
	}
}

func TestImportPortableRejectsInvalidSourceAndExistingTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.sqlite")
	target := filepath.Join(root, "target.sqlite")
	if err := os.WriteFile(source, []byte("not sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ImportPortable(context.Background(), source, target); !errors.Is(err, ErrImportInvalidSource) {
		t.Fatalf("corrupt source error = %v, want ErrImportInvalidSource", err)
	}
	incompatible := filepath.Join(root, "incompatible.sqlite")
	db, err := sql.Open("sqlite", databaseDSN(incompatible))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE unrelated(id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ImportPortable(context.Background(), incompatible, target); !errors.Is(err, ErrImportInvalidSource) {
		t.Fatalf("incompatible source error = %v, want ErrImportInvalidSource", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target after invalid import stat error = %v", err)
	}
	store, err := Open(context.Background(), source)
	if err == nil {
		_ = store.Close()
		t.Fatal("corrupt source unexpectedly opened")
	}
	validSource := filepath.Join(root, "valid.sqlite")
	store, err = Open(context.Background(), validSource)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := os.WriteFile(target, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ImportPortable(context.Background(), validSource, target); !errors.Is(err, ErrImportConflict) {
		t.Fatalf("existing target error = %v, want ErrImportConflict", err)
	}
	contents, err := os.ReadFile(target)
	if err != nil || string(contents) != "existing" {
		t.Fatalf("existing target changed: contents=%q err=%v", contents, err)
	}
}

func TestImportPortableRejectsNonRegularSourceBeforeOpening(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "portable.sqlite")
	if err := ImportPortable(context.Background(), root, target); !errors.Is(err, ErrImportInvalidSource) {
		t.Fatalf("directory source error = %v", err)
	}
	if _, err := os.Lstat(target + "-shm"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("directory source created a SQLite shm file: %v", err)
	}
}

func TestImportPortableConcurrentDestinationHasSingleWinner(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.sqlite")
	target := filepath.Join(root, "target.sqlite")
	store, err := Open(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			results <- ImportPortable(context.Background(), source, target)
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	var success, conflict int
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrImportConflict):
			conflict++
		default:
			t.Fatalf("concurrent import error = %v", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d; want one of each", success, conflict)
	}
	if imported, err := Open(context.Background(), target); err != nil {
		t.Fatal(err)
	} else if err := imported.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestImportPortableCancellationAndInvalidPaths(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.sqlite")
	target := filepath.Join(root, "target.sqlite")
	store, err := Open(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ImportPortable(ctx, source, target); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled import error = %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled target stat error = %v", err)
	}
	missingParent := filepath.Join(root, "missing", "deeper", "target.sqlite")
	if err := ImportPortable(context.Background(), source, missingParent); !errors.Is(err, ErrImportUnavailable) {
		t.Fatalf("missing destination directory error = %v", err)
	}
}

func TestImportPortableCancellationCleansOwnedStage(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.sqlite")
	targetDir := filepath.Join(root, "data")
	target := filepath.Join(targetDir, "target.sqlite")
	store, err := Open(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	large := strings.Repeat("x", 1024*1024)
	for index := range 8 {
		id := fmt.Sprintf("bulk-%d", index)
		if _, err := store.db.Exec("INSERT INTO anime(id, title, native_title, description) VALUES (?, 'Saved', '', ?)", id, large); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopWatcher := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			entries, _ := os.ReadDir(targetDir)
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".animeportable-import-") {
					cancel()
					return
				}
			}
			select {
			case <-stopWatcher:
				return
			case <-ticker.C:
			}
		}
	}()
	err = ImportPortable(ctx, source, target)
	close(stopWatcher)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled staged import error = %v", err)
	}
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("canceled import left artifacts: %v", entries)
	}
}

func TestValidatePortablePathRejectsSymlinkAncestor(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	linked := filepath.Join(root, "linked")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, linked); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if err := ValidatePortablePath(filepath.Join(linked, "data", "animeportable.db")); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("linked ancestor error = %v, want ErrInvalidInput", err)
	}
	source := filepath.Join(real, "source.sqlite")
	store, err := Open(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := ImportPortable(context.Background(), source, filepath.Join(linked, "target.sqlite")); !errors.Is(err, ErrImportUnavailable) {
		t.Fatalf("import through linked destination error = %v", err)
	}
}

func TestImportPortableSourceIsReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file mode bits do not enforce read-only access")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source.sqlite")
	target := filepath.Join(root, "target.sqlite")
	store, err := Open(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT INTO anime(id, title, native_title, description) VALUES ('read-only', 'Saved', '', '')"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(source, 0o400); err != nil {
		t.Fatal(err)
	}
	before := databaseArtifacts(t, source)
	if err := ImportPortable(context.Background(), source, target); err != nil {
		t.Fatalf("read-only source import: %v", err)
	}
	if after := databaseArtifacts(t, source); !equalArtifactHashes(before, after) {
		t.Fatal("read-only source database artifacts changed")
	}
	if err := os.Chmod(source, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

type artifactState struct {
	hash     [32]byte
	size     int64
	modified time.Time
	readable bool
}

func databaseArtifacts(t *testing.T, path string) map[string]artifactState {
	t.Helper()
	artifacts := make(map[string]artifactState)
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		data, err := readArtifact(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			if runtime.GOOS != "windows" || !strings.HasSuffix(candidate, "-shm") {
				t.Fatal(err)
			}
			info, statErr := os.Stat(candidate)
			if statErr != nil {
				t.Fatal(statErr)
			}
			artifacts[candidate] = artifactState{size: info.Size(), modified: info.ModTime()}
			continue
		}
		info, err := os.Stat(candidate)
		if err != nil {
			t.Fatal(err)
		}
		artifacts[candidate] = artifactState{hash: sha256.Sum256(data), size: info.Size(), modified: info.ModTime(), readable: true}
	}
	return artifacts
}

func readArtifact(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil || runtime.GOOS != "windows" || !strings.HasSuffix(path, "-shm") {
		return data, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	const lockStart = 120
	const lockEnd = 128
	if info.Size() < lockEnd {
		return nil, err
	}
	data = make([]byte, info.Size())
	if _, err := file.ReadAt(data[:lockStart], 0); err != nil {
		return nil, err
	}
	if _, err := file.ReadAt(data[lockEnd:], lockEnd); err != nil {
		return nil, err
	}
	return data, nil
}

func equalArtifactHashes(left, right map[string]artifactState) bool {
	if len(left) != len(right) {
		return false
	}
	for path, state := range left {
		if right[path] != state {
			return false
		}
	}
	return true
}
