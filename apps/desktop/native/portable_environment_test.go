// SPDX-License-Identifier: MPL-2.0

package native

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFyneEnvironmentPathsStayInsidePortableData(t *testing.T) {
	data := filepath.Join(t.TempDir(), "portable data")
	for _, goos := range []string{"windows", "linux", "darwin"} {
		paths := fyneEnvironmentPaths(goos, data)
		if len(paths) == 0 {
			t.Fatalf("no environment paths for %s", goos)
		}
		for key, value := range paths {
			if !filepath.IsAbs(value) || !strings.HasPrefix(filepath.Clean(value), data+string(filepath.Separator)) {
				t.Fatalf("%s %s resolves outside portable data: %q", goos, key, value)
			}
		}
	}
}

func TestFyneEnvironmentRejectsLinkedStateDirectory(t *testing.T) {
	linkedNames := []string{"fyne-cache", "fyne-home"}
	if runtime.GOOS == "darwin" {
		linkedNames = []string{"fyne-home"}
	} else if runtime.GOOS == "linux" {
		linkedNames = []string{"fyne-cache", "fyne-config", "fyne-data"}
	}
	for _, linkedName := range linkedNames {
		t.Run(linkedName, func(t *testing.T) {
			data := filepath.Join(t.TempDir(), "data")
			if err := os.Mkdir(data, 0o700); err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			if err := os.Symlink(outside, filepath.Join(data, linkedName)); err != nil {
				t.Skipf("directory symlink unavailable: %v", err)
			}
			before := os.Environ()
			_, _, err := RedirectFyneEnvironment(data)
			if !errors.Is(err, ErrPortableEnvironment) {
				t.Fatalf("linked state directory error = %v", err)
			}
			if after := os.Environ(); !equalEnvironment(before, after) {
				t.Fatal("environment changed after rejecting linked state directory")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatalf("linked destination changed: entries=%v err=%v", entries, err)
			}
		})
	}
}

func TestFyneEnvironmentRejectsWindowsJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction fixture")
	}
	data := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(data, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	junction := filepath.Join(data, "fyne-cache")
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", junction, outside).CombinedOutput(); err != nil {
		t.Fatalf("create Windows junction: %v: %s", err, output)
	}
	t.Cleanup(func() { _ = os.Remove(junction) })
	info, err := os.Lstat(junction)
	if err != nil || info.Mode()&os.ModeIrregular == 0 {
		t.Fatalf("junction fixture is not irregular: info=%v err=%v", info, err)
	}
	before := os.Environ()
	_, _, err = RedirectFyneEnvironment(data)
	if !errors.Is(err, ErrPortableEnvironment) || !equalEnvironment(before, os.Environ()) {
		t.Fatalf("junction escape error=%v environment changed=%v", err, !equalEnvironment(before, os.Environ()))
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("junction target changed: entries=%v err=%v", entries, err)
	}
}

func equalEnvironment(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestFyneEnvironmentRedirectPreservesOriginalForMPVAndRestores(t *testing.T) {
	data := filepath.Join(t.TempDir(), "portable data")
	paths := fyneEnvironmentPaths(runtime.GOOS, data)
	if len(paths) == 0 {
		t.Skip("unsupported desktop OS")
	}
	prior := make(map[string]environmentValue, len(paths))
	for key := range paths {
		value, present := os.LookupEnv(key)
		prior[key] = environmentValue{value: value, present: present}
	}
	original, restore, err := RedirectFyneEnvironment(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	for key, want := range paths {
		if got := os.Getenv(key); got != want {
			t.Fatalf("redirected %s=%q, want %q", key, got, want)
		}
		foundOriginal := false
		for _, pair := range original {
			if strings.HasPrefix(strings.ToUpper(pair), strings.ToUpper(key)+"=") {
				foundOriginal = true
				if !prior[key].present || !strings.EqualFold(strings.SplitN(pair, "=", 2)[1], prior[key].value) {
					t.Fatalf("original %s was not preserved", key)
				}
			}
		}
		if foundOriginal != prior[key].present {
			t.Fatalf("original presence of %s changed", key)
		}
	}
	cacheDir, cacheErr := os.UserCacheDir()
	configDir, configErr := os.UserConfigDir()
	if cacheErr != nil || configErr != nil {
		t.Fatalf("redirected OS directories: cache=%v config=%v", cacheErr, configErr)
	}
	if !strings.HasPrefix(filepath.Clean(cacheDir), data+string(filepath.Separator)) || !strings.HasPrefix(filepath.Clean(configDir), data+string(filepath.Separator)) {
		t.Fatalf("Go OS directories escaped portable data: cache=%q config=%q", cacheDir, configDir)
	}
	if info, err := os.Stat(cacheDir); err != nil || !info.IsDir() {
		t.Fatalf("portable cache root was not created: info=%v err=%v", info, err)
	}
	if info, err := os.Stat(data); err != nil || !info.IsDir() {
		t.Fatalf("portable data directory was not created: info=%v err=%v", info, err)
	}
	restore()
	for key, previous := range prior {
		value, present := os.LookupEnv(key)
		if present != previous.present || value != previous.value {
			t.Fatalf("%s was not restored", key)
		}
	}
}
