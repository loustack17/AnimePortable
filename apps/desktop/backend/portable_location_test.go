// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"animeportable/adapters/persistence/sqlite"
	"animeportable/core"
)

type locationFixture struct {
	root string
	plan PortablePlan
}

func newLocationFixture(t *testing.T) locationFixture {
	t.Helper()
	root := filepath.Join(t.TempDir(), "App ü")
	executable := portableTestExecutable(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("app"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := planPortable(runtime.GOOS, executable)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Location != LocationDefault || plan.CustomDir != "" {
		t.Fatalf("fresh plan location = %v %q, want default", plan.Location, plan.CustomDir)
	}
	return locationFixture{root: root, plan: plan}
}

func (fixture locationFixture) replan(t *testing.T) PortablePlan {
	t.Helper()
	plan, err := planPortable(runtime.GOOS, fixture.plan.executable)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func (fixture locationFixture) locationFile() string {
	return filepath.Join(fixture.root, LocationFileName)
}

func (fixture locationFixture) writeLocation(t *testing.T, content string) {
	t.Helper()
	if err := os.WriteFile(fixture.locationFile(), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newLocationFolder(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "Chosen ü")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// openSeededStore opens the plan's database with one history row.
func openSeededStore(t *testing.T, plan PortablePlan) *sqlite.Store {
	t.Helper()
	if err := os.MkdirAll(plan.DataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := sqlite.Open(context.Background(), plan.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	entry := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "anime", EpisodeID: "episode", Position: time.Minute}}
	if err := store.SavePlaybackCheckpoint(ctx, entry); err != nil {
		t.Fatal(err)
	}
	return store
}

func requireHistoryRows(t *testing.T, databasePath string, want int) {
	t.Helper()
	store, err := sqlite.Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if history, err := store.History(context.Background()); err != nil || len(history) != want {
		t.Fatalf("history of %q = %#v, %v; want %d rows", databasePath, history, err, want)
	}
}

func requireLocationFile(t *testing.T, fixture locationFixture, want string) {
	t.Helper()
	got, err := os.ReadFile(fixture.locationFile())
	if err != nil || strings.TrimSpace(string(got)) != want {
		t.Fatalf("location file = %q, %v; want %q", got, err, want)
	}
}

func TestPlanPortableReadsCustomLocation(t *testing.T) {
	fixture := newLocationFixture(t)
	folder := newLocationFolder(t)
	custom := filepath.Join(folder, DatabaseFileName)
	if err := os.WriteFile(custom, []byte("db"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"newline":      folder + "\n",
		"crlf":         folder + "\r\n",
		"no newline":   folder,
		"bom":          "\ufeff" + folder + "\n",
		"unclean path": folder + string(filepath.Separator) + "." + string(filepath.Separator) + "\n",
	} {
		fixture.writeLocation(t, content)
		plan := fixture.replan(t)
		if plan.Location != LocationCustom || plan.CustomDir != folder || plan.DataDir != folder || plan.DatabasePath != custom || !plan.TargetExists {
			t.Fatalf("%s: plan = %#v, want custom %q", name, plan, folder)
		}
	}
}

func TestPlanPortableReportsMissingCustomLocation(t *testing.T) {
	folder := newLocationFolder(t)
	emptyFolder := newLocationFolder(t)
	dbIsDirectory := newLocationFolder(t)
	if err := os.Mkdir(filepath.Join(dbIsDirectory, DatabaseFileName), 0o700); err != nil {
		t.Fatal(err)
	}
	notDirectory := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDirectory, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		content string
		wantDir string
	}{
		"folder missing":     {filepath.Join(folder, "gone") + "\n", filepath.Join(folder, "gone")},
		"database missing":   {emptyFolder + "\n", emptyFolder},
		"database is folder": {dbIsDirectory + "\n", dbIsDirectory},
		"not a folder":       {notDirectory + "\n", notDirectory},
		"relative":           {"data\n", ""},
		"unc":                {`\\server\share\data` + "\n", `\\server\share\data`},
		"device":             {`\\.\C:\data` + "\n", `\\.\C:\data`},
		"empty":              {"\n", ""},
		"two lines":          {folder + "\n" + folder + "\n", ""},
		"nul":                {folder + "\x00\n", ""},
		"invalid utf-8":      {folder + "\xff\n", ""},
		"too large":          {strings.Repeat("a", locationFileMaxBytes+1), ""},
	}
	for name, test := range cases {
		fixture := newLocationFixture(t)
		fixture.writeLocation(t, test.content)
		plan := fixture.replan(t)
		if plan.Location != LocationCustomMissing || plan.CustomDir != test.wantDir {
			t.Fatalf("%s: location = %v %q, want missing %q", name, plan.Location, plan.CustomDir, test.wantDir)
		}
		if plan.DatabasePath != "" || plan.DataDir != "" {
			t.Fatalf("%s: missing location must not expose a database path: %#v", name, plan)
		}
		if err := plan.CreateFresh(); err == nil {
			t.Fatalf("%s: CreateFresh must refuse a missing custom location", name)
		}
		if _, err := os.Lstat(filepath.Join(fixture.root, "data")); !os.IsNotExist(err) {
			t.Fatalf("%s: planning created the default data folder: %v", name, err)
		}
	}
}

func TestPlanPortableRejectsLocationFileThatIsALink(t *testing.T) {
	fixture := newLocationFixture(t)
	folder := newLocationFolder(t)
	if err := os.WriteFile(filepath.Join(folder, DatabaseFileName), []byte("db"), 0o600); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(t.TempDir(), "real.txt")
	if err := os.WriteFile(real, []byte(folder+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, fixture.locationFile()); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if plan := fixture.replan(t); plan.Location != LocationCustomMissing || plan.DatabasePath != "" {
		t.Fatalf("linked location file plan = %#v, want missing", plan)
	}
}

func TestUseDefaultLocationRemovesLocationFileOnlyOnRequest(t *testing.T) {
	fixture := newLocationFixture(t)
	fixture.writeLocation(t, filepath.Join(fixture.root, "gone")+"\n")
	plan := fixture.replan(t)
	if plan.Location != LocationCustomMissing {
		t.Fatalf("setup: location = %v", plan.Location)
	}
	if _, err := os.Lstat(fixture.locationFile()); err != nil {
		t.Fatalf("planning removed the location file: %v", err)
	}
	next, err := plan.UseDefaultLocation()
	if err != nil {
		t.Fatal(err)
	}
	if next.Location != LocationDefault || next.DataDir != filepath.Join(fixture.root, "data") || next.DatabasePath == "" {
		t.Fatalf("default plan = %#v", next)
	}
	if _, err := os.Lstat(fixture.locationFile()); !os.IsNotExist(err) {
		t.Fatalf("location file still present: %v", err)
	}
	if _, err := fixture.plan.UseDefaultLocation(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("UseDefaultLocation on a normal plan = %v, want ErrInvalidInput", err)
	}
}

func TestChangeLocationToEmptyFolderCopiesDataAndWritesLocationFile(t *testing.T) {
	fixture := newLocationFixture(t)
	store := openSeededStore(t, fixture.plan)
	folder := newLocationFolder(t)

	move, err := fixture.plan.PrepareLocationChange(folder)
	if err != nil {
		t.Fatal(err)
	}
	if move.HasExistingData() {
		t.Fatal("empty folder reported existing data")
	}
	if _, err := os.Lstat(fixture.locationFile()); !os.IsNotExist(err) {
		t.Fatalf("preparing wrote the location file: %v", err)
	}
	if err := move.CopyCurrent(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	requireHistoryRows(t, filepath.Join(folder, DatabaseFileName), 1)
	requireLocationFile(t, fixture, folder)
	requireHistoryRows(t, fixture.plan.DatabasePath, 1)

	plan := fixture.replan(t)
	if plan.Location != LocationCustom || plan.DatabasePath != filepath.Join(folder, DatabaseFileName) {
		t.Fatalf("next start plan = %#v", plan)
	}
	entries, err := os.ReadDir(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".data-location-") {
			t.Fatalf("temporary location file %q left behind", entry.Name())
		}
	}
}

func TestChangeLocationUseExistingKeepsFolderData(t *testing.T) {
	fixture := newLocationFixture(t)
	store := openSeededStore(t, fixture.plan)
	folder := newLocationFolder(t)
	other, err := sqlite.Open(context.Background(), filepath.Join(folder, DatabaseFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(folder, DatabaseFileName))
	if err != nil {
		t.Fatal(err)
	}

	move, err := fixture.plan.PrepareLocationChange(folder)
	if err != nil {
		t.Fatal(err)
	}
	if !move.HasExistingData() {
		t.Fatal("existing database not reported")
	}
	if err := move.CopyCurrent(context.Background(), store); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CopyCurrent over existing data = %v, want ErrInvalidInput", err)
	}
	if _, err := os.Lstat(fixture.locationFile()); !os.IsNotExist(err) {
		t.Fatal("refused copy wrote the location file")
	}
	if err := move.UseExisting(context.Background()); err != nil {
		t.Fatal(err)
	}
	requireLocationFile(t, fixture, folder)
	after, err := os.ReadFile(filepath.Join(folder, DatabaseFileName))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("existing database changed: %v", err)
	}
	requireHistoryRows(t, filepath.Join(folder, DatabaseFileName), 0)
}

func TestChangeLocationUseExistingRejectsForeignDatabase(t *testing.T) {
	fixture := newLocationFixture(t)
	openSeededStore(t, fixture.plan)
	folder := newLocationFolder(t)
	if err := os.WriteFile(filepath.Join(folder, DatabaseFileName), []byte("this is not sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	move, err := fixture.plan.PrepareLocationChange(folder)
	if err != nil {
		t.Fatal(err)
	}
	if err := move.UseExisting(context.Background()); err == nil {
		t.Fatal("foreign database was accepted")
	}
	if _, err := os.Lstat(fixture.locationFile()); !os.IsNotExist(err) {
		t.Fatal("rejected database still wrote the location file")
	}
}

func TestChangeLocationOverwriteBacksUpExistingFiles(t *testing.T) {
	fixture := newLocationFixture(t)
	store := openSeededStore(t, fixture.plan)
	folder := newLocationFolder(t)
	database := filepath.Join(folder, DatabaseFileName)
	files := map[string]string{database: "original db", database + "-wal": "original wal", database + "-shm": "original shm"}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	move, err := fixture.plan.PrepareLocationChange(folder)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 21, 9, 8, 7, 0, time.UTC)
	if err := move.OverwriteWithCurrent(context.Background(), store, now); err != nil {
		t.Fatal(err)
	}
	backup := database + ".bak-20261021T090807Z"
	for path, content := range files {
		suffix := strings.TrimPrefix(path, database)
		got, err := os.ReadFile(backup + suffix)
		if err != nil || string(got) != content {
			t.Fatalf("backup%s = %q, %v; want %q", suffix, got, err, content)
		}
	}
	requireHistoryRows(t, database, 1)
	requireLocationFile(t, fixture, folder)
}

func TestChangeLocationOverwriteRestoresBackupWhenSnapshotFails(t *testing.T) {
	fixture := newLocationFixture(t)
	folder := newLocationFolder(t)
	database := filepath.Join(folder, DatabaseFileName)
	if err := os.WriteFile(database, []byte("original db"), 0o600); err != nil {
		t.Fatal(err)
	}
	move, err := fixture.plan.PrepareLocationChange(folder)
	if err != nil {
		t.Fatal(err)
	}
	err = move.OverwriteWithCurrent(context.Background(), failingSnapshotter{}, time.Now())
	if err == nil {
		t.Fatal("failed snapshot reported success")
	}
	if got, readErr := os.ReadFile(database); readErr != nil || string(got) != "original db" {
		t.Fatalf("original database not restored: %q %v", got, readErr)
	}
	if _, err := os.Lstat(fixture.locationFile()); !os.IsNotExist(err) {
		t.Fatal("failed overwrite wrote the location file")
	}
}

type failingSnapshotter struct{}

func (failingSnapshotter) SnapshotTo(context.Context, string) error { return ErrUnavailable }

func TestRestoreDefaultLocationRemovesLocationFile(t *testing.T) {
	fixture := newLocationFixture(t)
	folder := newLocationFolder(t)
	custom := PortablePlan{Root: fixture.root, DataDir: folder, DatabasePath: filepath.Join(folder, DatabaseFileName), Location: LocationCustom, CustomDir: folder, executable: fixture.plan.executable}
	store := openSeededStore(t, custom)
	fixture.writeLocation(t, folder+"\n")

	move, err := custom.PrepareRestoreDefault()
	if err != nil {
		t.Fatal(err)
	}
	defaultDir := filepath.Join(fixture.root, "data")
	if move.Dir() != defaultDir || move.HasExistingData() {
		t.Fatalf("restore move = %q existing=%t", move.Dir(), move.HasExistingData())
	}
	info, err := os.Lstat(defaultDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("default folder not created: %v", err)
	}
	if err := move.CopyCurrent(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	requireHistoryRows(t, filepath.Join(defaultDir, DatabaseFileName), 1)
	if _, err := os.Lstat(fixture.locationFile()); !os.IsNotExist(err) {
		t.Fatalf("location file still present: %v", err)
	}
	if plan := fixture.replan(t); plan.Location != LocationDefault || !plan.TargetExists {
		t.Fatalf("restored plan = %#v", plan)
	}
	if _, err := fixture.plan.PrepareRestoreDefault(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("restore on the default location = %v, want ErrInvalidInput", err)
	}
}

func TestRestoreDefaultLocationWithExistingDefaultData(t *testing.T) {
	fixture := newLocationFixture(t)
	defaultStore := openSeededStore(t, fixture.plan)
	if err := defaultStore.Close(); err != nil {
		t.Fatal(err)
	}
	folder := newLocationFolder(t)
	custom := PortablePlan{Root: fixture.root, DataDir: folder, DatabasePath: filepath.Join(folder, DatabaseFileName), Location: LocationCustom, CustomDir: folder}
	fixture.writeLocation(t, folder+"\n")
	move, err := custom.PrepareRestoreDefault()
	if err != nil {
		t.Fatal(err)
	}
	if !move.HasExistingData() {
		t.Fatal("existing default data not reported")
	}
	if err := move.UseExisting(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(fixture.locationFile()); !os.IsNotExist(err) {
		t.Fatalf("location file still present: %v", err)
	}
}

func TestChangeLocationRejectsInvalidTargets(t *testing.T) {
	fixture := newLocationFixture(t)
	openSeededStore(t, fixture.plan)
	notDirectory := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDirectory, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	targets := map[string]string{
		"empty":         "",
		"relative":      "data2",
		"unc":           `\\server\share\data`,
		"device":        `\\.\C:\data`,
		"extended":      `\\?\C:\data`,
		"nul":           newLocationFolder(t) + "\x00",
		"not a folder":  notDirectory,
		"missing":       filepath.Join(t.TempDir(), "missing"),
		"current":       fixture.plan.DataDir,
		"current cased": strings.ToUpper(fixture.plan.DataDir),
	}
	if runtime.GOOS != "windows" {
		delete(targets, "current cased")
	}
	for name, target := range targets {
		if _, err := fixture.plan.PrepareLocationChange(target); err == nil {
			t.Fatalf("%s target %q was accepted", name, target)
		}
	}

	real := newLocationFolder(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := os.Mkdir(filepath.Join(real, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"link": link, "under link": filepath.Join(link, "child")} {
		if _, err := fixture.plan.PrepareLocationChange(target); err == nil {
			t.Fatalf("%s target %q was accepted", name, target)
		}
	}
}

func TestChangeLocationRejectsStaleSidecarsWithoutDatabase(t *testing.T) {
	fixture := newLocationFixture(t)
	openSeededStore(t, fixture.plan)
	folder := newLocationFolder(t)
	if err := os.WriteFile(filepath.Join(folder, DatabaseFileName+"-wal"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.plan.PrepareLocationChange(folder); err == nil {
		t.Fatal("stale WAL without a database was accepted")
	}
}

func TestLocationFileIsReplacedAtomically(t *testing.T) {
	fixture := newLocationFixture(t)
	first := newLocationFolder(t)
	second := newLocationFolder(t)
	if err := writeLocationFile(fixture.root, first); err != nil {
		t.Fatal(err)
	}
	requireLocationFile(t, fixture, first)
	if err := writeLocationFile(fixture.root, second); err != nil {
		t.Fatal(err)
	}
	requireLocationFile(t, fixture, second)
	entries, err := os.ReadDir(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".data-location-") {
			t.Fatalf("temporary file %q left behind", entry.Name())
		}
	}
	if err := writeLocationFile(filepath.Join(fixture.root, "missing"), second); err == nil {
		t.Fatal("writing into a missing root succeeded")
	}
	requireLocationFile(t, fixture, second)
}

func useRemoteDrive(t *testing.T) {
	t.Helper()
	previous := isLocalDrive
	isLocalDrive = func(string) bool { return false }
	t.Cleanup(func() { isLocalDrive = previous })
}

func TestChangeLocationRejectsNonLocalDrive(t *testing.T) {
	fixture := newLocationFixture(t)
	openSeededStore(t, fixture.plan)
	folder := newLocationFolder(t)
	useRemoteDrive(t)
	if _, err := fixture.plan.PrepareLocationChange(folder); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("remote drive target error = %v, want ErrInvalidInput", err)
	}
}

func TestPlanPortableReportsLocationOnNonLocalDriveAsMissing(t *testing.T) {
	fixture := newLocationFixture(t)
	folder := newLocationFolder(t)
	if err := os.WriteFile(filepath.Join(folder, DatabaseFileName), []byte("db"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture.writeLocation(t, folder+"\n")
	useRemoteDrive(t)
	plan := fixture.replan(t)
	if plan.Location != LocationCustomMissing || plan.CustomDir != folder || plan.DatabasePath != "" {
		t.Fatalf("remote drive plan = %#v, want missing with %q", plan, folder)
	}
}

func TestLocalDriveTypeAcceptsTestFolderAndRejectsRelative(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("drive types are Windows-only")
	}
	if !localDriveType(t.TempDir()) {
		t.Fatal("temp folder drive was not treated as local")
	}
	if localDriveType(`\server\share\x`) || localDriveType(`relative`) {
		t.Fatal("UNC or relative path was treated as local")
	}
}
