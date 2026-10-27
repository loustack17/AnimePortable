// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"animeportable/adapters/persistence/sqlite"
)

type PortablePlan struct {
	Root         string
	DataDir      string
	DatabasePath string
	TargetExists bool
	// Location says whether data-location.txt is in use. CustomDir is the folder it names
	// (empty when the file was unreadable); both are empty or default for the standard layout.
	Location  LocationState
	CustomDir string

	executable string
}

func PlanPortable(executablePath string) (PortablePlan, error) {
	return planPortable(runtime.GOOS, executablePath)
}

func planPortable(goos, executablePath string) (PortablePlan, error) {
	if goos != "windows" {
		return PortablePlan{}, ErrInvalidInput
	}
	if strings.TrimSpace(executablePath) == "" || !filepath.IsAbs(executablePath) {
		return PortablePlan{}, ErrInvalidInput
	}
	executablePath = filepath.Clean(executablePath)
	info, err := os.Lstat(executablePath)
	if err != nil || !info.Mode().IsRegular() {
		return PortablePlan{}, ErrUnavailable
	}
	root := filepath.Dir(executablePath)
	plan := PortablePlan{Root: root, executable: executablePath}
	dataDir := defaultDataDir(root)
	switch location := readDataLocation(root); location.state {
	case LocationCustomMissing:
		// No database path on purpose: nothing may open or create data until the user chooses.
		plan.Location = LocationCustomMissing
		plan.CustomDir = location.dir
		return plan, nil
	case LocationCustom:
		plan.Location = LocationCustom
		plan.CustomDir = location.dir
		dataDir = location.dir
	}
	databasePath := filepath.Join(dataDir, DatabaseFileName)
	if err := sqlite.ValidatePortablePath(databasePath); err != nil {
		return PortablePlan{}, ErrUnavailable
	}
	plan.DataDir = dataDir
	plan.DatabasePath = databasePath
	targetInfo, targetErr := os.Lstat(databasePath)
	if targetErr == nil {
		if !targetInfo.Mode().IsRegular() {
			return PortablePlan{}, ErrUnavailable
		}
		plan.TargetExists = true
	} else if !errors.Is(targetErr, os.ErrNotExist) {
		return PortablePlan{}, ErrUnavailable
	}
	return plan, nil
}

// IsDatabaseExists reports whether err says the database file was already there.
func IsDatabaseExists(err error) bool {
	return errors.Is(err, sqlite.ErrTargetExists)
}

func (plan PortablePlan) CreateFresh() error {
	if plan.TargetExists || plan.DataDir == "" || plan.DatabasePath != filepath.Join(plan.DataDir, DatabaseFileName) {
		return ErrInvalidInput
	}
	if err := sqlite.ValidatePortablePath(plan.DatabasePath); err != nil {
		return ErrUnavailable
	}
	if err := os.Mkdir(plan.DataDir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return ErrUnavailable
	}
	if err := sqlite.ValidatePortablePath(plan.DatabasePath); err != nil {
		return ErrUnavailable
	}
	dir, err := os.Lstat(plan.DataDir)
	if err != nil || !dir.IsDir() {
		return ErrUnavailable
	}
	file, err := os.OpenFile(plan.DatabasePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return sqlite.ErrTargetExists
	}
	if err != nil {
		return ErrUnavailable
	}
	if err := file.Close(); err != nil {
		return ErrUnavailable
	}
	return nil
}
