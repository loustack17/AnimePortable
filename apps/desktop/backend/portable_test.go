// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPlanPortableOSRootsAndLegacyOffer(t *testing.T) {
	for _, goos := range []string{"windows", "linux", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			parent := filepath.Join(t.TempDir(), "Anime Portable ü")
			executable := filepath.Join(parent, "AnimePortable")
			if goos == "darwin" {
				executable = filepath.Join(parent, "AnimePortable.app", "Contents", "MacOS", "AnimePortable")
			}
			if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(executable, []byte("executable fixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			configDir := filepath.Join(t.TempDir(), "Legacy Config")
			legacyPath := filepath.Join(configDir, "AnimePortable", "animeportable.db")
			if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
				t.Fatal(err)
			}
			legacyBytes := []byte("legacy profile")
			if err := os.WriteFile(legacyPath, legacyBytes, 0o600); err != nil {
				t.Fatal(err)
			}

			plan, err := planPortable(goos, executable, configDir)
			if err != nil {
				t.Fatal(err)
			}
			root := parent
			wantData := filepath.Join(root, "data")
			wantDatabase := filepath.Join(wantData, "animeportable.db")
			if plan.Root != root || plan.DataDir != wantData || plan.DatabasePath != wantDatabase {
				t.Fatalf("portable locations = %#v, want root=%q data=%q database=%q", plan, root, wantData, wantDatabase)
			}
			if plan.LegacyPath != legacyPath || !plan.OfferImport {
				t.Fatalf("legacy offer = path %q, offer %t; want %q and true", plan.LegacyPath, plan.OfferImport, legacyPath)
			}
			if _, err := os.Stat(plan.DataDir); !os.IsNotExist(err) {
				t.Fatalf("planning created portable data directory: stat error = %v", err)
			}
			gotLegacy, err := os.ReadFile(legacyPath)
			if err != nil || string(gotLegacy) != string(legacyBytes) {
				t.Fatalf("planning changed legacy profile: bytes=%q err=%v", gotLegacy, err)
			}
		})
	}
}

func TestNewAtStartsAtRequestedPortableDatabasePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Anime Portable ü")
	databasePath := filepath.Join(root, "data", "animeportable.db")
	configDir := filepath.Join(t.TempDir(), "legacy config")
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", configDir)
	case "linux":
		t.Setenv("XDG_CONFIG_HOME", configDir)
	}

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
	legacyPath := filepath.Join(configDir, "AnimePortable", "animeportable.db")
	if _, err := os.Lstat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("startup wrote outside the selected portable root at %q: stat error = %v", legacyPath, err)
	}
}

func TestPlanPortableSuppressesImportWhenTargetExists(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "animeportable")
	if err := os.WriteFile(executable, []byte("app"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "data", "animeportable.db")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("portable profile"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(t.TempDir(), "AnimePortable", "animeportable.db")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}

	plan, err := planPortable(runtime.GOOS, executable, filepath.Dir(filepath.Dir(legacy)))
	if err != nil {
		t.Fatal(err)
	}
	if plan.OfferImport {
		t.Fatal("existing portable database must suppress import offer")
	}
}

func TestPlanPortableMissingLegacyProfileDoesNotOfferImport(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "animeportable")
	if err := os.WriteFile(executable, []byte("app"), 0o600); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(t.TempDir(), "not-created")

	plan, err := planPortable(runtime.GOOS, executable, configDir)
	if err != nil {
		t.Fatal(err)
	}
	wantLegacy := filepath.Join(configDir, "AnimePortable", "animeportable.db")
	if plan.LegacyPath != wantLegacy || plan.OfferImport {
		t.Fatalf("missing config profile plan = %#v; want path %q and no import offer", plan, wantLegacy)
	}
}

func TestPlanPortableUsesExecutablePathNotWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "Anime Portable ü", "animeportable")
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("app"), 0o600); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(t.TempDir(), "config")
	want, err := planPortable(runtime.GOOS, executable, configDir)
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
	got, err := planPortable(runtime.GOOS, executable, configDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("plan depends on working directory: before=%#v after=%#v", want, got)
	}
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
	if _, err := planPortable(runtime.GOOS, executable, filepath.Join(root, "config")); err == nil {
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
		case IsPortableImportConflict(err):
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
