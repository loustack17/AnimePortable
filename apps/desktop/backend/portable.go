// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
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
	LegacyPath   string
	OfferImport  bool
	TargetExists bool
}

func PlanPortable(executablePath, userConfigDir string) (PortablePlan, error) {
	return planPortable(runtime.GOOS, executablePath, userConfigDir)
}

func planPortable(goos, executablePath, userConfigDir string) (PortablePlan, error) {
	if strings.TrimSpace(executablePath) == "" || !filepath.IsAbs(executablePath) {
		return PortablePlan{}, ErrInvalidInput
	}
	executablePath = filepath.Clean(executablePath)
	info, err := os.Lstat(executablePath)
	if err != nil || !info.Mode().IsRegular() {
		return PortablePlan{}, ErrUnavailable
	}
	root := filepath.Dir(executablePath)
	if goos == "darwin" {
		macOSDir := filepath.Dir(executablePath)
		contents := filepath.Dir(macOSDir)
		bundle := filepath.Dir(contents)
		if filepath.Base(macOSDir) != "MacOS" || filepath.Base(contents) != "Contents" || filepath.Base(bundle) != "AnimePortable.app" {
			return PortablePlan{}, ErrInvalidInput
		}
		root = filepath.Dir(bundle)
	}
	dataDir := filepath.Join(root, "data")
	databasePath := filepath.Join(dataDir, "animeportable.db")
	if err := sqlite.ValidatePortablePath(databasePath); err != nil {
		return PortablePlan{}, ErrUnavailable
	}
	plan := PortablePlan{Root: root, DataDir: dataDir, DatabasePath: databasePath}
	targetInfo, targetErr := os.Lstat(databasePath)
	if targetErr == nil {
		if !targetInfo.Mode().IsRegular() {
			return PortablePlan{}, ErrUnavailable
		}
		plan.TargetExists = true
	} else if !errors.Is(targetErr, os.ErrNotExist) {
		return PortablePlan{}, ErrUnavailable
	}
	if userConfigDir == "" || !filepath.IsAbs(userConfigDir) {
		return plan, nil
	}
	legacyPath := filepath.Join(filepath.Clean(userConfigDir), "AnimePortable", "animeportable.db")
	plan.LegacyPath = legacyPath
	legacyInfo, legacyErr := os.Lstat(legacyPath)
	if legacyErr == nil && legacyInfo.Mode().IsRegular() {
		plan.OfferImport = !plan.TargetExists
	}
	return plan, nil
}

func (plan PortablePlan) Import(ctx context.Context) error {
	if !plan.OfferImport || plan.LegacyPath == "" || plan.DatabasePath == "" {
		return ErrInvalidInput
	}
	return sqlite.ImportPortable(ctx, plan.LegacyPath, plan.DatabasePath)
}

func IsPortableImportConflict(err error) bool {
	return errors.Is(err, sqlite.ErrImportConflict)
}

func (plan PortablePlan) CreateFresh() error {
	if plan.TargetExists || plan.DataDir == "" || plan.DatabasePath != filepath.Join(plan.DataDir, "animeportable.db") {
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
		return sqlite.ErrImportConflict
	}
	if err != nil {
		return ErrUnavailable
	}
	if err := file.Close(); err != nil {
		return ErrUnavailable
	}
	return nil
}
