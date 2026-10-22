// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"animeportable/core"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"animeportable/adapters/persistence/sqlite"
)

// newLocationService is a Service over the fixture's real store, holding the fixture's plan.
func newLocationService(t *testing.T, fixture locationFixture) *Service {
	t.Helper()
	store := openSeededStore(t, fixture.plan)
	service := newWithDependencies(dependencies{store: store, source: &persistentSource{}})
	service.plan = fixture.plan
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func seedFolderDatabase(t *testing.T, folder string) string {
	t.Helper()
	path := filepath.Join(folder, DatabaseFileName)
	other, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDataLocationReportsTheConfiguredFolder(t *testing.T) {
	fixture := newLocationFixture(t)
	service := newLocationService(t, fixture)
	got, err := service.DataLocation(context.Background())
	if err != nil || got != (DataLocation{Dir: fixture.plan.DataDir}) {
		t.Fatalf("DataLocation = %#v, %v; want the default folder, not custom", got, err)
	}

	missing := newWithDependencies(dependencies{store: openSeededStore(t, fixture.plan), source: &persistentSource{}})
	if _, err := missing.DataLocation(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("DataLocation without a plan = %v, want ErrUnavailable", err)
	}
}

func TestMoveDataLocationToEmptyFolderCopiesAndReportsTheNewFolder(t *testing.T) {
	fixture := newLocationFixture(t)
	service := newLocationService(t, fixture)
	folder := newLocationFolder(t)

	result, err := service.MoveDataLocation(context.Background(), MoveDataLocationRequest{Target: folder})
	if err != nil || result != (MoveDataLocationResult{Dir: folder}) {
		t.Fatalf("MoveDataLocation = %#v, %v", result, err)
	}
	requireHistoryRows(t, filepath.Join(folder, DatabaseFileName), 1)
	requireLocationFile(t, fixture, folder)
	if got, _ := service.DataLocation(context.Background()); got != (DataLocation{Dir: folder, Custom: true}) {
		t.Fatalf("DataLocation after the move = %#v", got)
	}
}

func TestMoveDataLocationAsksForAChoiceBeforeTouchingAFolderWithData(t *testing.T) {
	fixture := newLocationFixture(t)
	service := newLocationService(t, fixture)
	folder := newLocationFolder(t)
	database := seedFolderDatabase(t, folder)
	before, err := os.ReadFile(database)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.MoveDataLocation(context.Background(), MoveDataLocationRequest{Target: folder})
	if err != nil || result != (MoveDataLocationResult{NeedsChoice: true, Dir: folder}) {
		t.Fatalf("MoveDataLocation = %#v, %v; want NeedsChoice", result, err)
	}
	if _, err := os.Lstat(fixture.locationFile()); !os.IsNotExist(err) {
		t.Fatal("asking for a choice wrote the location file")
	}
	if after, err := os.ReadFile(database); err != nil || string(after) != string(before) {
		t.Fatalf("asking for a choice changed the folder's database: %v", err)
	}
	if got, _ := service.DataLocation(context.Background()); got.Custom {
		t.Fatalf("asking for a choice changed the location to %#v", got)
	}
}

func TestMoveDataLocationUseExistingKeepsTheFolderData(t *testing.T) {
	fixture := newLocationFixture(t)
	service := newLocationService(t, fixture)
	folder := newLocationFolder(t)
	database := seedFolderDatabase(t, folder)

	result, err := service.MoveDataLocation(context.Background(), MoveDataLocationRequest{Target: folder, Existing: ExistingUse})
	if err != nil || result != (MoveDataLocationResult{Dir: folder}) {
		t.Fatalf("MoveDataLocation = %#v, %v", result, err)
	}
	requireLocationFile(t, fixture, folder)
	requireHistoryRows(t, database, 0)
}

func TestMoveDataLocationOverwriteBacksUpTheFolderDatabase(t *testing.T) {
	fixture := newLocationFixture(t)
	service := newLocationService(t, fixture)
	folder := newLocationFolder(t)
	database := seedFolderDatabase(t, folder)

	result, err := service.MoveDataLocation(context.Background(), MoveDataLocationRequest{Target: folder, Existing: ExistingOverwrite})
	if err != nil || result != (MoveDataLocationResult{Dir: folder}) {
		t.Fatalf("MoveDataLocation = %#v, %v", result, err)
	}
	requireLocationFile(t, fixture, folder)
	requireHistoryRows(t, database, 1)
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal(err)
	}
	backups := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), DatabaseFileName+".bak-") {
			backups++
		}
	}
	if backups != 1 {
		t.Fatalf("folder holds %d backups, want the replaced database kept once", backups)
	}
}

func TestMoveDataLocationRestoreDefaultReturnsToTheDefaultFolder(t *testing.T) {
	fixture := newLocationFixture(t)
	folder := newLocationFolder(t)
	store := openSeededStore(t, fixture.plan)
	move, err := fixture.plan.PrepareLocationChange(folder)
	if err != nil {
		t.Fatal(err)
	}
	if err := move.CopyCurrent(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	// The app now runs from the chosen folder, as after a restart.
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	custom := fixture.replan(t)
	service := newLocationService(t, locationFixture{root: fixture.root, plan: custom})
	if got, _ := service.DataLocation(context.Background()); got != (DataLocation{Dir: folder, Custom: true}) {
		t.Fatalf("DataLocation = %#v", got)
	}

	// The default folder already holds the original database: the user has to choose.
	result, err := service.MoveDataLocation(context.Background(), MoveDataLocationRequest{RestoreDefault: true})
	if err != nil || !result.NeedsChoice || result.Dir != fixture.plan.DataDir {
		t.Fatalf("restore = %#v, %v; want NeedsChoice for %q", result, err, fixture.plan.DataDir)
	}
	result, err = service.MoveDataLocation(context.Background(), MoveDataLocationRequest{RestoreDefault: true, Existing: ExistingUse})
	if err != nil || result != (MoveDataLocationResult{Dir: fixture.plan.DataDir}) {
		t.Fatalf("restore with choice = %#v, %v", result, err)
	}
	if _, err := os.Lstat(fixture.locationFile()); !os.IsNotExist(err) {
		t.Fatal("restoring the default left the location file")
	}
	if got, _ := service.DataLocation(context.Background()); got != (DataLocation{Dir: fixture.plan.DataDir}) {
		t.Fatalf("DataLocation after the restore = %#v", got)
	}
}

func TestMoveDataLocationRejectsBadRequests(t *testing.T) {
	fixture := newLocationFixture(t)
	service := newLocationService(t, fixture)
	folder := newLocationFolder(t)
	for name, request := range map[string]MoveDataLocationRequest{
		"unknown choice":             {Target: folder, Existing: "merge"},
		"restore with a target":      {Target: folder, RestoreDefault: true},
		"restore while default":      {RestoreDefault: true},
		"relative target":            {Target: "data"},
		"empty target":               {},
		"the folder already in use":  {Target: fixture.plan.DataDir},
		"folder that does not exist": {Target: filepath.Join(folder, "missing")},
		"target with a control byte": {Target: folder + "\n"},
		"unknown choice and restore": {RestoreDefault: true, Existing: "merge"},
		"target that is a file":      {Target: fixture.plan.DatabasePath},
	} {
		if _, err := service.MoveDataLocation(context.Background(), request); err == nil {
			t.Errorf("%s: MoveDataLocation succeeded, want an error", name)
		}
	}
	if _, err := os.Lstat(fixture.locationFile()); !os.IsNotExist(err) {
		t.Fatal("a rejected request wrote the location file")
	}
	if got, _ := service.DataLocation(context.Background()); got.Custom {
		t.Fatalf("a rejected request changed the location to %#v", got)
	}
}

func TestMoveDataLocationRefusesWritesToTheOldDatabaseAfterwards(t *testing.T) {
	fixture := newLocationFixture(t)
	service := newLocationService(t, fixture)
	ctx := context.Background()
	folder := newLocationFolder(t)
	if err := service.SaveSettings(ctx, Settings{Appearance: "dark"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.MoveDataLocation(ctx, MoveDataLocationRequest{Target: folder}); err != nil {
		t.Fatal(err)
	}

	// Everything written before the move is in the copy; nothing written after it can reach the old file.
	err := service.SaveSettings(ctx, Settings{Appearance: "light"})
	if !errors.Is(err, ErrRestartRequired) || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("SaveSettings after the move = %v, want ErrRestartRequired", err)
	}
	if err := service.Follow(ctx, "anime"); !errors.Is(err, ErrRestartRequired) {
		t.Fatalf("Follow after the move = %v, want ErrRestartRequired", err)
	}
	copied, err := sqlite.Open(ctx, filepath.Join(folder, DatabaseFileName))
	if err != nil {
		t.Fatal(err)
	}
	defer copied.Close()
	if settings, err := copied.Settings(ctx); err != nil || settings.Appearance != core.AppearanceDark {
		t.Fatalf("the copy holds %+v, %v; want the settings saved before the move", settings, err)
	}
	if history, err := copied.History(ctx); err != nil || len(history) != 1 {
		t.Fatalf("the copy holds %d history rows, %v; want the pre-move row", len(history), err)
	}
	if settings, err := service.Settings(ctx); err != nil || settings.Appearance != "dark" {
		t.Fatalf("reads after the move = %+v, %v; want the old database unchanged", settings, err)
	}
	if _, err := service.MoveDataLocation(ctx, MoveDataLocationRequest{Target: newLocationFolder(t)}); !errors.Is(err, ErrRestartRequired) {
		t.Fatalf("a second move = %v, want ErrRestartRequired", err)
	}
	requireLocationFile(t, fixture, folder)
}

// snapshotHook runs inside the snapshot, where a write queued on the store would be waiting.
type snapshotHook struct {
	*sqlite.Store
	during func()
}

func (store snapshotHook) SnapshotTo(ctx context.Context, target string) error {
	store.during()
	return store.Store.SnapshotTo(ctx, target)
}

func TestMoveDataLocationRefusesWritesAttemptedWhileTheCopyRuns(t *testing.T) {
	fixture := newLocationFixture(t)
	ctx := context.Background()
	var service *Service
	var duringErr error
	store := snapshotHook{Store: openSeededStore(t, fixture.plan), during: func() {
		duringErr = service.SaveSettings(ctx, Settings{Appearance: "light"})
	}}
	service = newWithDependencies(dependencies{store: store, source: &persistentSource{}})
	service.plan = fixture.plan
	t.Cleanup(func() { _ = service.Close() })

	if _, err := service.MoveDataLocation(ctx, MoveDataLocationRequest{Target: newLocationFolder(t)}); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(duringErr, ErrRestartRequired) {
		t.Fatalf("a write during the copy = %v, want ErrRestartRequired", duringErr)
	}
}

func TestMoveDataLocationThatFailsAllowsWritesAgain(t *testing.T) {
	fixture := newLocationFixture(t)
	service := newLocationService(t, fixture)
	ctx := context.Background()
	folder := newLocationFolder(t)
	if err := os.WriteFile(filepath.Join(folder, DatabaseFileName), []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := service.MoveDataLocation(ctx, MoveDataLocationRequest{Target: folder, Existing: ExistingUse}); err == nil {
		t.Fatal("using a foreign file as the database succeeded")
	}
	if err := service.SaveSettings(ctx, Settings{Appearance: "light"}); err != nil {
		t.Fatalf("SaveSettings after a failed move = %v, want writes allowed", err)
	}
	// A choice request changes nothing either.
	if result, err := service.MoveDataLocation(ctx, MoveDataLocationRequest{Target: folder}); err != nil || !result.NeedsChoice {
		t.Fatalf("choice request = %#v, %v", result, err)
	}
	if err := service.SaveSettings(ctx, Settings{Appearance: "dark"}); err != nil {
		t.Fatalf("SaveSettings after a choice request = %v", err)
	}
}

func TestMoveDataLocationRejectsTheFolderOfTheOpenDatabase(t *testing.T) {
	fixture := newLocationFixture(t)
	folder := newLocationFolder(t)
	store := openSeededStore(t, fixture.plan)
	move, err := fixture.plan.PrepareLocationChange(folder)
	if err != nil {
		t.Fatal(err)
	}
	if err := move.CopyCurrent(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	// After the restart the app runs from the chosen folder, and the original folder still has its database.
	custom := fixture.replan(t)
	service := newLocationService(t, locationFixture{root: fixture.root, plan: custom})
	before, err := os.ReadFile(custom.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	for name, request := range map[string]MoveDataLocationRequest{
		"the live folder":                 {Target: folder},
		"the live folder, other spelling": {Target: folder + string(filepath.Separator) + "."},
		"overwrite the live folder":       {Target: folder, Existing: ExistingOverwrite},
	} {
		_, err := service.MoveDataLocation(context.Background(), request)
		if !errors.Is(err, ErrDataFolderInUse) || !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: MoveDataLocation = %v, want ErrDataFolderInUse", name, err)
		}
	}
	if after, err := os.ReadFile(custom.DatabasePath); err != nil || string(after) != string(before) {
		t.Fatalf("a rejected move touched the live database: %v", err)
	}
	entries, _ := os.ReadDir(folder)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".bak-") {
			t.Fatalf("a rejected move renamed the live database to %q", entry.Name())
		}
	}

	// Moving back to the folder that is not in use is still possible.
	if result, err := service.MoveDataLocation(context.Background(), MoveDataLocationRequest{Target: fixture.plan.DataDir}); err != nil || !result.NeedsChoice {
		t.Fatalf("moving back to the original folder = %#v, %v", result, err)
	}
}
