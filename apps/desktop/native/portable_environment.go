// SPDX-License-Identifier: MPL-2.0

package native

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"animeportable/apps/desktop/backend"
)

var ErrPortableEnvironment = errors.New("portable UI environment unavailable")

const ApplicationID = "com.animeportable.desktop"

type environmentValue struct {
	value   string
	present bool
}

func RedirectFyneEnvironment(dataDir string) ([]string, func(), error) {
	if !filepath.IsAbs(dataDir) {
		return nil, nil, ErrPortableEnvironment
	}
	if err := validateFyneStatePaths(runtime.GOOS, dataDir); err != nil {
		return nil, nil, ErrPortableEnvironment
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, nil, ErrPortableEnvironment
	}
	if err := validateFyneStatePaths(runtime.GOOS, dataDir); err != nil {
		return nil, nil, ErrPortableEnvironment
	}
	paths := fyneEnvironmentPaths(runtime.GOOS, dataDir)
	if len(paths) == 0 {
		return nil, nil, ErrPortableEnvironment
	}
	original := os.Environ()
	prior := make(map[string]environmentValue, len(paths))
	for key := range paths {
		value, present := os.LookupEnv(key)
		prior[key] = environmentValue{value: value, present: present}
	}
	restore := func() {
		for key, previous := range prior {
			if previous.present {
				_ = os.Setenv(key, previous.value)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}
	for key, value := range paths {
		if err := os.Setenv(key, value); err != nil {
			restore()
			return nil, nil, ErrPortableEnvironment
		}
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil || !insidePortableData(dataDir, cacheDir) || backend.ValidatePortableStatePath(cacheDir) != nil {
		restore()
		return nil, nil, ErrPortableEnvironment
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		restore()
		return nil, nil, ErrPortableEnvironment
	}
	if err := validateFyneStatePaths(runtime.GOOS, dataDir); err != nil {
		restore()
		return nil, nil, ErrPortableEnvironment
	}
	return original, restore, nil
}

func validateFyneStatePaths(goos, dataDir string) error {
	for _, path := range fyneStatePaths(goos, dataDir) {
		if err := backend.ValidatePortableStatePath(path); err != nil {
			return err
		}
	}
	return nil
}

func fyneStatePaths(goos, dataDir string) []string {
	switch goos {
	case "windows":
		return []string{
			filepath.Join(dataDir, "fyne-home", "AppData", "Roaming", "fyne", ApplicationID),
			filepath.Join(dataDir, "fyne-cache", "fyne", ApplicationID),
		}
	case "darwin":
		return []string{
			filepath.Join(dataDir, "fyne-home", "Library", "Preferences", "fyne", ApplicationID),
			filepath.Join(dataDir, "fyne-home", "Library", "Caches", "fyne", ApplicationID),
			filepath.Join(dataDir, "fyne-home", "Library", "Application Support"),
		}
	case "linux":
		return []string{
			filepath.Join(dataDir, "fyne-config", "fyne", ApplicationID),
			filepath.Join(dataDir, "fyne-cache", "fyne", ApplicationID),
			filepath.Join(dataDir, "fyne-data"),
		}
	default:
		return nil
	}
}

func insidePortableData(dataDir, path string) bool {
	relative, err := filepath.Rel(dataDir, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func fyneEnvironmentPaths(goos, dataDir string) map[string]string {
	switch goos {
	case "windows":
		return map[string]string{
			"USERPROFILE":  filepath.Join(dataDir, "fyne-home"),
			"APPDATA":      filepath.Join(dataDir, "fyne-home", "AppData", "Roaming"),
			"LOCALAPPDATA": filepath.Join(dataDir, "fyne-cache"),
		}
	case "darwin":
		return map[string]string{"HOME": filepath.Join(dataDir, "fyne-home")}
	case "linux":
		return map[string]string{
			"XDG_CONFIG_HOME": filepath.Join(dataDir, "fyne-config"),
			"XDG_CACHE_HOME":  filepath.Join(dataDir, "fyne-cache"),
			"XDG_DATA_HOME":   filepath.Join(dataDir, "fyne-data"),
		}
	default:
		return nil
	}
}
