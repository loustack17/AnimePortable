// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"animeportable/adapters/persistence/sqlite"
)

// ErrDataFolderInUse means the chosen data folder is the one the open database is in.
var ErrDataFolderInUse = fmt.Errorf("data folder is in use: %w", ErrInvalidInput)

const (
	DatabaseFileName     = "animeportable.db"
	LocationFileName     = "data-location.txt"
	locationFileMaxBytes = 4 << 10
	backupTimeLayout     = "20060102T150405Z"
	utf8BOM              = "\xef\xbb\xbf"
)

type LocationState int

const (
	// LocationDefault means no location file: data lives in the data folder next to the executable.
	LocationDefault LocationState = iota
	// LocationCustom means data-location.txt names an existing folder that holds a database.
	LocationCustom
	// LocationCustomMissing means data-location.txt exists but its folder or database cannot be used.
	LocationCustomMissing
)

// DatabaseSnapshotter is the open store, as far as moving data needs it.
type DatabaseSnapshotter interface {
	SnapshotTo(ctx context.Context, target string) error
}

type dataLocation struct {
	state LocationState
	dir   string
}

func defaultDataDir(root string) string {
	return filepath.Join(root, "data")
}

func locationFilePath(root string) string {
	return filepath.Join(root, LocationFileName)
}

func readDataLocation(root string) dataLocation {
	path := locationFilePath(root)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return dataLocation{state: LocationDefault}
	}
	missing := dataLocation{state: LocationCustomMissing}
	if err != nil || !info.Mode().IsRegular() {
		return missing
	}
	file, err := os.Open(path)
	if err != nil {
		return missing
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return missing
	}
	raw, err := io.ReadAll(io.LimitReader(file, locationFileMaxBytes+1))
	if err != nil || len(raw) > locationFileMaxBytes {
		return missing
	}
	dir, ok := parseLocation(raw)
	if !ok {
		return missing
	}
	missing.dir = dir
	if !isLocalDrive(dir) {
		return missing
	}
	if sqlite.ValidatePortablePath(filepath.Join(dir, DatabaseFileName)) != nil {
		return missing
	}
	if folder, err := os.Lstat(dir); err != nil || !folder.IsDir() {
		return missing
	}
	if database, err := os.Lstat(filepath.Join(dir, DatabaseFileName)); err != nil || !database.Mode().IsRegular() {
		return missing
	}
	return dataLocation{state: LocationCustom, dir: dir}
}

// parseLocation accepts one UTF-8 line holding an absolute path.
func parseLocation(raw []byte) (string, bool) {
	text := strings.TrimPrefix(string(raw), utf8BOM)
	if !utf8.ValidString(text) {
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" || strings.IndexFunc(text, unicode.IsControl) >= 0 || !filepath.IsAbs(text) {
		return "", false
	}
	return filepath.Clean(text), true
}

// UseDefaultLocation answers a missing custom location: it removes data-location.txt
// and plans again, so startup continues with the data folder next to the executable.
func (plan PortablePlan) UseDefaultLocation() (PortablePlan, error) {
	if plan.Location != LocationCustomMissing || plan.executable == "" {
		return PortablePlan{}, ErrInvalidInput
	}
	if err := removeLocationFile(plan.Root); err != nil {
		return PortablePlan{}, err
	}
	return PlanPortable(plan.executable)
}

func removeLocationFile(root string) error {
	if err := os.Remove(locationFilePath(root)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return ErrUnavailable
	}
	return nil
}

// LocationMove is a validated change of the data folder. It takes effect at the next start.
type LocationMove struct {
	root           string
	dir            string
	restoreDefault bool
	hasData        bool
}

// PrepareLocationChange validates a folder chosen by the user as the new data location.
func (plan PortablePlan) PrepareLocationChange(target string) (LocationMove, error) {
	if plan.Root == "" || plan.DataDir == "" {
		return LocationMove{}, ErrInvalidInput
	}
	dir, err := validateLocationFolder(target)
	if err != nil {
		return LocationMove{}, err
	}
	if plan.opensFolder(dir) {
		return LocationMove{}, ErrDataFolderInUse
	}
	return newLocationMove(plan.Root, dir, false)
}

// PrepareRestoreDefault targets the data folder next to the executable, creating it when needed.
func (plan PortablePlan) PrepareRestoreDefault() (LocationMove, error) {
	if plan.Root == "" || plan.DataDir == "" || plan.Location == LocationDefault {
		return LocationMove{}, ErrInvalidInput
	}
	dir := defaultDataDir(plan.Root)
	if err := sqlite.ValidatePortablePath(filepath.Join(dir, DatabaseFileName)); err != nil {
		return LocationMove{}, ErrUnavailable
	}
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return LocationMove{}, ErrUnavailable
	}
	if _, err := validateLocationFolder(dir); err != nil {
		return LocationMove{}, err
	}
	if plan.opensFolder(dir) {
		return LocationMove{}, ErrDataFolderInUse
	}
	return newLocationMove(plan.Root, dir, true)
}

// opensFolder reports whether dir holds the database the app has open. Renaming or replacing
// that file would break the running app, so it is never a target.
func (plan PortablePlan) opensFolder(dir string) bool {
	return sameFolder(dir, plan.DataDir) || plan.DatabasePath != "" && sameFolder(dir, filepath.Dir(plan.DatabasePath))
}

func validateLocationFolder(target string) (string, error) {
	if sqlite.ValidatePortablePath(target) != nil {
		return "", ErrInvalidInput
	}
	dir := filepath.Clean(target)
	if !isLocalDrive(dir) {
		return "", ErrInvalidInput
	}
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
		return "", ErrInvalidInput
	}
	return dir, nil
}

func newLocationMove(root, dir string, restoreDefault bool) (LocationMove, error) {
	move := LocationMove{root: root, dir: dir, restoreDefault: restoreDefault}
	info, err := os.Lstat(move.databasePath())
	switch {
	case err == nil:
		if !info.Mode().IsRegular() {
			return LocationMove{}, ErrInvalidInput
		}
		move.hasData = true
	case errors.Is(err, os.ErrNotExist):
		// Sidecars without a database would be picked up by the copy as stale journal data.
		for _, suffix := range []string{"-wal", "-shm"} {
			if _, err := os.Lstat(move.databasePath() + suffix); !errors.Is(err, os.ErrNotExist) {
				return LocationMove{}, ErrInvalidInput
			}
		}
	default:
		return LocationMove{}, ErrUnavailable
	}
	return move, nil
}

func (move LocationMove) databasePath() string {
	return filepath.Join(move.dir, DatabaseFileName)
}

// Dir is the validated target folder.
func (move LocationMove) Dir() string {
	return move.dir
}

// HasExistingData is true when the folder already holds animeportable.db; the user then
// chooses UseExisting or OverwriteWithCurrent.
func (move LocationMove) HasExistingData() bool {
	return move.hasData
}

// CopyCurrent snapshots the open database into an empty target and then switches the location.
func (move LocationMove) CopyCurrent(ctx context.Context, store DatabaseSnapshotter) error {
	if move.hasData || store == nil {
		return ErrInvalidInput
	}
	if err := store.SnapshotTo(ctx, move.databasePath()); err != nil {
		return err
	}
	return move.commit()
}

// UseExisting switches to the database already in the folder after checking that it opens.
func (move LocationMove) UseExisting(ctx context.Context) error {
	if !move.hasData {
		return ErrInvalidInput
	}
	if err := sqlite.ValidateDatabase(ctx, move.databasePath()); err != nil {
		return err
	}
	return move.commit()
}

// OverwriteWithCurrent renames the existing database (and sidecars) to a timestamped backup,
// snapshots the open database in its place and switches the location. Nothing is deleted.
func (move LocationMove) OverwriteWithCurrent(ctx context.Context, store DatabaseSnapshotter, now time.Time) error {
	if !move.hasData || store == nil {
		return ErrInvalidInput
	}
	renamed, err := move.backUpExisting(now)
	if err != nil {
		return err
	}
	if err := store.SnapshotTo(ctx, move.databasePath()); err != nil {
		undoRenames(renamed)
		return err
	}
	return move.commit()
}

type renamedFile struct{ from, to string }

func (move LocationMove) backUpExisting(now time.Time) ([]renamedFile, error) {
	stamp := now.UTC().Format(backupTimeLayout)
	database := move.databasePath()
	backup := database + ".bak-" + stamp
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, err := os.Lstat(backup + suffix); !errors.Is(err, os.ErrNotExist) {
			return nil, ErrUnavailable
		}
	}
	var renamed []renamedFile
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, err := os.Lstat(database + suffix); errors.Is(err, os.ErrNotExist) && suffix != "" {
			continue
		}
		step := renamedFile{from: database + suffix, to: backup + suffix}
		if err := os.Rename(step.from, step.to); err != nil {
			undoRenames(renamed)
			return nil, ErrUnavailable
		}
		renamed = append(renamed, step)
	}
	return renamed, nil
}

func undoRenames(renamed []renamedFile) {
	for index := len(renamed) - 1; index >= 0; index-- {
		_ = os.Rename(renamed[index].to, renamed[index].from)
	}
}

// commit records the new location: a restore removes data-location.txt, anything else replaces it atomically.
func (move LocationMove) commit() error {
	if move.restoreDefault {
		return removeLocationFile(move.root)
	}
	return writeLocationFile(move.root, move.dir)
}

func writeLocationFile(root, dir string) error {
	temp, err := os.CreateTemp(root, ".data-location-*")
	if err != nil {
		return ErrUnavailable
	}
	name := temp.Name()
	_, writeErr := temp.WriteString(dir + "\n")
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(name)
		return ErrUnavailable
	}
	if err := os.Rename(name, locationFilePath(root)); err != nil {
		_ = os.Remove(name)
		return ErrUnavailable
	}
	return nil
}

// sameFolder compares by name first and then by identity, so a second spelling of the same
// folder (an 8.3 short name, a subst drive) is recognised as well.
func sameFolder(left, right string) bool {
	if runtime.GOOS == "windows" && strings.EqualFold(filepath.Clean(left), filepath.Clean(right)) || filepath.Clean(left) == filepath.Clean(right) {
		return true
	}
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
}
