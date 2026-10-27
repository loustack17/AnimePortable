// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPlanPortableWindowsRoot(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "Anime Portable ü")
	executable := filepath.Join(parent, "AnimePortable")
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("executable fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := planPortable("windows", executable)
	if err != nil {
		t.Fatal(err)
	}
	root := parent
	wantData := filepath.Join(root, "data")
	wantDatabase := filepath.Join(wantData, "animeportable.db")
	if plan.Root != root || plan.DataDir != wantData || plan.DatabasePath != wantDatabase {
		t.Fatalf("portable locations = %#v, want root=%q data=%q database=%q", plan, root, wantData, wantDatabase)
	}
	if _, err := os.Stat(plan.DataDir); !os.IsNotExist(err) {
		t.Fatalf("planning created portable data directory: stat error = %v", err)
	}
}

func TestNewAtStartsAtRequestedPortableDatabasePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Anime Portable ü")
	databasePath := filepath.Join(root, "data", "animeportable.db")
	service := NewAt(databasePath)
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(databasePath)
	if err != nil {
		t.Fatalf("portable database was not created at the requested path: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("portable database is not a regular file: mode %v", info.Mode())
	}
}

func TestPlanPortableUsesExecutablePathNotWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	executable := portableTestExecutable(filepath.Join(root, "Anime Portable ü"))
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("app"), 0o600); err != nil {
		t.Fatal(err)
	}
	want, err := planPortable(runtime.GOOS, executable)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	otherDir := t.TempDir()
	if err := os.Chdir(otherDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	got, err := planPortable(runtime.GOOS, executable)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("plan depends on working directory: before=%#v after=%#v", want, got)
	}
}

func portableTestExecutable(root string) string {
	return filepath.Join(root, "animeportable")
}

func TestPlanPortableRejectsSymlinkedAncestor(t *testing.T) {
	root := t.TempDir()
	realRoot := filepath.Join(root, "actual")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	executable := filepath.Join(link, "animeportable")
	if err := os.WriteFile(filepath.Join(realRoot, "animeportable"), []byte("app"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := planPortable(runtime.GOOS, executable); err == nil {
		t.Fatal("symlinked executable ancestor was accepted")
	}
}

func TestPortableFreshCreationIsExclusive(t *testing.T) {
	root := t.TempDir()
	plan := PortablePlan{Root: root, DataDir: filepath.Join(root, "data"), DatabasePath: filepath.Join(root, "data", "animeportable.db")}
	results := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			results <- plan.CreateFresh()
		}()
	}
	close(start)
	var created, conflicts int
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			created++
		case IsDatabaseExists(err):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent creation error: %v", err)
		}
	}
	if created != 1 || conflicts != 1 {
		t.Fatalf("created=%d conflicts=%d, want one of each", created, conflicts)
	}
	if info, err := os.Lstat(plan.DatabasePath); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("exclusive creation did not leave a regular database: info=%v err=%v", info, err)
	}
}
